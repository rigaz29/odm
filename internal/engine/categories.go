package engine

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// CategoryDef is one category: a name, which is also its folder, and the
// file extensions filed under it.
type CategoryDef struct {
	Name string   `json:"name"`
	Exts []string `json:"exts,omitempty"` // lower case, without dots
}

// OtherCategory takes every file no other category claims. It is always
// last and lists no extensions.
const OtherCategory = "Other"

// categoryOrder is the order of the built-in categories.
var categoryOrder = []string{
	"Video", "Audio", "Documents", "Programs", "Compressed", "Images", "Disk Images", OtherCategory,
}

// DefaultCategories is the built-in set of categories.
func DefaultCategories() []CategoryDef {
	out := make([]CategoryDef, 0, len(categoryOrder))
	for _, name := range categoryOrder {
		var exts []string
		for ext, c := range categoryExt {
			if c == name {
				exts = append(exts, strings.TrimPrefix(ext, "."))
			}
		}
		sort.Strings(exts)
		out = append(out, CategoryDef{Name: name, Exts: exts})
	}
	return out
}

// normalizeCategories cleans a category list as typed: names trimmed and
// unique (ignoring case), extensions lower case without dots, each claimed
// by the first category that lists it, and Other last with none. A list
// that comes out the same as the built-in one becomes nil, so a later
// version's defaults reach it.
func normalizeCategories(defs []CategoryDef) []CategoryDef {
	if len(defs) == 0 {
		return nil
	}
	seenName := map[string]bool{}
	seenExt := map[string]bool{}
	var out []CategoryDef
	for _, d := range defs {
		name := strings.TrimSpace(d.Name)
		key := strings.ToLower(name)
		if name == "" || seenName[key] || strings.EqualFold(name, OtherCategory) || strings.ContainsAny(name, `\/:*?"<>|`) {
			continue
		}
		seenName[key] = true
		var exts []string
		for _, e := range d.Exts {
			e = strings.ToLower(strings.Trim(strings.TrimSpace(e), "."))
			if e == "" || seenExt[e] {
				continue
			}
			seenExt[e] = true
			exts = append(exts, e)
		}
		sort.Strings(exts)
		out = append(out, CategoryDef{Name: name, Exts: exts})
	}
	out = append(out, CategoryDef{Name: OtherCategory})
	if sameCategories(out, DefaultCategories()) {
		return nil
	}
	return out
}

func sameCategories(a, b []CategoryDef) bool {
	return slices.EqualFunc(a, b, func(x, y CategoryDef) bool {
		return x.Name == y.Name && slices.Equal(x.Exts, y.Exts)
	})
}

// categoryTable is the categories in use. It is replaced as a whole when the
// settings change, so readers need no lock.
type categoryTable struct {
	defs  []CategoryDef
	byExt map[string]string // ".mp4" -> "Video"
}

func newCategoryTable(defs []CategoryDef) *categoryTable {
	if len(defs) == 0 {
		defs = DefaultCategories()
	}
	t := &categoryTable{defs: defs, byExt: map[string]string{}}
	for _, d := range defs {
		for _, e := range d.Exts {
			if _, taken := t.byExt["."+e]; !taken {
				t.byExt["."+e] = d.Name
			}
		}
	}
	return t
}

var (
	categoryState   atomic.Pointer[categoryTable]
	defaultCategory = sync.OnceValue(func() *categoryTable { return newCategoryTable(nil) })
)

// setCategories makes defs (nil: the built-in set) the categories in use.
func setCategories(defs []CategoryDef) { categoryState.Store(newCategoryTable(defs)) }

func currentCategories() *categoryTable {
	if t := categoryState.Load(); t != nil {
		return t
	}
	return defaultCategory()
}

// Category maps a file name to its category, which is also its subfolder.
func Category(name string) string {
	if c, ok := currentCategories().byExt[strings.ToLower(filepath.Ext(name))]; ok {
		return c
	}
	return OtherCategory
}

// Categories returns the category names in use, in display order.
func Categories() []string {
	defs := currentCategories().defs
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}

// CategoriesInUse returns a copy of the categories in use, extensions
// included, for the Options window to edit.
func CategoriesInUse() []CategoryDef {
	defs := currentCategories().defs
	out := make([]CategoryDef, len(defs))
	for i, d := range defs {
		out[i] = CategoryDef{Name: d.Name, Exts: slices.Clone(d.Exts)}
	}
	return out
}

// CategoryExtensions lists the extensions filed under cat, without dots.
func CategoryExtensions(cat string) []string {
	for _, d := range currentCategories().defs {
		if d.Name == cat {
			return slices.Clone(d.Exts)
		}
	}
	return nil
}

// knownCategory reports whether cat is one of the categories in use.
func knownCategory(cat string) bool {
	for _, d := range currentCategories().defs {
		if d.Name == cat {
			return true
		}
	}
	return false
}
