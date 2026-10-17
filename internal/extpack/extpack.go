// Package extpack builds the per-browser packages of the ODM extension from
// the single source folder extension/.
//
// One manifest cannot serve both families cleanly: Firefox does not support
// background.service_worker and needs background.scripts, while Chrome warns
// about Firefox-only keys. Chromium keeps the "key" that pins its unpacked
// ID; Firefox identifies the add-on by browser_specific_settings.gecko.id
// and has no use for "key".
package extpack

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Targets.
const (
	Chromium = "chromium"
	Firefox  = "firefox"
)

// skip lists source entries that are not part of the shipped extension;
// Go files (files.go, which embeds it into ODM) are left out too.
var skip = map[string]bool{"manifest.json": true, "README.md": true, "native": true, "test": true}

var versionRe = regexp.MustCompile(`^\d+(\.\d+){0,3}$`)

// ManifestVersion turns a release tag ("v0.2.0", "0.2.0-rc1") into the
// numeric dotted form extension manifests require ("0.2.0").
func ManifestVersion(tag string) (string, error) {
	v := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if !versionRe.MatchString(v) {
		return "", fmt.Errorf("%q is not a numeric version for an extension manifest", tag)
	}
	return v, nil
}

// Manifest derives the manifest for target from the source manifest.
func Manifest(src []byte, target, version string) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(src, &m); err != nil {
		return nil, fmt.Errorf("manifest.json: %w", err)
	}
	if version != "" {
		m["version"] = version
	}
	bg, _ := m["background"].(map[string]any)
	switch target {
	case Chromium:
		delete(m, "browser_specific_settings")
		if bg != nil {
			delete(bg, "scripts")
		}
	case Firefox:
		delete(m, "key")
		bss, _ := m["browser_specific_settings"].(map[string]any)
		gecko, _ := bss["gecko"].(map[string]any)
		if id, _ := gecko["id"].(string); id == "" {
			return nil, fmt.Errorf("manifest.json has no browser_specific_settings.gecko.id")
		}
		sw, _ := bg["service_worker"].(string)
		if sw == "" {
			return nil, fmt.Errorf("manifest.json has no background.service_worker to convert")
		}
		m["background"] = map[string]any{"scripts": []string{sw}}
	default:
		return nil, fmt.Errorf("unknown target %q", target)
	}
	return json.MarshalIndent(m, "", "  ")
}

// Build writes outDir/<target>/ (unpacked) and the archive for each target:
// odm-extension-chromium.zip and odm-extension-firefox-unsigned.xpi. It
// returns the archive paths keyed by target.
func Build(srcDir, outDir, version string) (map[string]string, error) {
	fsys := os.DirFS(srcDir)
	src, err := fs.ReadFile(fsys, "manifest.json")
	if err != nil {
		return nil, err
	}
	files, err := sourceFiles(fsys)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, target := range []string{Chromium, Firefox} {
		man, err := Manifest(src, target, version)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", target, err)
		}
		dir := filepath.Join(outDir, target)
		if err := os.RemoveAll(dir); err != nil {
			return nil, err
		}
		if err := writeTree(fsys, dir, files, man); err != nil {
			return nil, err
		}
		name := "odm-extension-chromium.zip"
		if target == Firefox {
			name = "odm-extension-firefox-unsigned.xpi"
		}
		archive := filepath.Join(outDir, name)
		if err := zipTree(dir, archive); err != nil {
			return nil, err
		}
		out[target] = archive
	}
	return out, nil
}

// Unpack writes the extension in fsys (the source folder's files, as
// extension.Files holds them) to dir for target, as "Load unpacked" reads
// it, with version in the manifest ("" keeps the source's). Only files
// whose content differs are written, and files that are no longer part of
// it are removed, so a browser that loaded dir sees a change only when
// there is one. changed reports whether anything was written or removed.
func Unpack(fsys fs.FS, target, dir, version string) (changed bool, err error) {
	src, err := fs.ReadFile(fsys, "manifest.json")
	if err != nil {
		return false, err
	}
	man, err := Manifest(src, target, version)
	if err != nil {
		return false, err
	}
	files, err := sourceFiles(fsys)
	if err != nil {
		return false, err
	}
	want := map[string][]byte{"manifest.json": man}
	for _, rel := range files {
		if want[rel], err = fs.ReadFile(fsys, rel); err != nil {
			return false, err
		}
	}
	for rel, data := range want {
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if old, err := os.ReadFile(dst); err == nil && bytes.Equal(old, data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return changed, err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return changed, err
		}
		changed = true
	}
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if _, ok := want[filepath.ToSlash(rel)]; !ok {
			if err := os.Remove(p); err != nil {
				return err
			}
			changed = true
		}
		return nil
	})
	return changed, err
}

func sourceFiles(fsys fs.FS) ([]string, error) {
	var files []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		top := strings.SplitN(p, "/", 2)[0]
		if skip[top] {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() && path.Ext(p) != ".go" {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func writeTree(fsys fs.FS, dir string, files []string, manifest []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0o644); err != nil {
		return err
	}
	for _, rel := range files {
		data, err := fs.ReadFile(fsys, rel)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// zipTree zips dir's contents with paths relative to dir (manifest.json at
// the root, as both stores and "Load unpacked" expect), in sorted order.
func zipTree(dir, archive string) error {
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	var paths []string
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			paths = append(paths, p)
		}
		return err
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for _, p := range paths {
		rel, _ := filepath.Rel(dir, p)
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		r.Close()
		if err != nil {
			return err
		}
	}
	return zw.Close()
}
