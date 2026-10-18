// Command odm is Ochi Download Manager: a segmented, resumable download engine
// with a desktop UI, a browser native-messaging relay and a headless mode.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"odm/internal/browserreg"
	"odm/internal/clipmon"
	"odm/internal/engine"
	"odm/internal/ipc"
	"odm/internal/native"
	"odm/internal/osutil"
	"odm/internal/quarantine"
	"odm/internal/server"
	"odm/internal/sysproxy"
	"odm/internal/tools"
	"odm/internal/torrent"
	"odm/internal/ui"
	"odm/internal/version"
)

func main() {
	// A browser starting the native host passes its own arguments (origin
	// or manifest path), never our flags, so recognise that before parsing;
	// otherwise ODM would open as a second full instance.
	if native.LaunchedByBrowser(os.Args[1:]) {
		runNativeHost(configBaseDir())
		return
	}
	// A finished file is marked as coming from the internet, as a browser
	// marks it, and scanned by the registered antivirus program.
	engine.MarkFile = quarantine.Mark

	addr := flag.String("addr", "127.0.0.1:8721", "web UI/API address used with -serve")
	serve := flag.Bool("serve", false, "also serve the web UI and API on -addr (token-protected)")
	dir := flag.String("dir", "", "download folder (blank = saved setting / ~/Downloads)")
	conns := flag.Int("conns", 0, "connections per download (0 = saved setting)")
	files := flag.Int("files", 0, "simultaneous downloads (0 = saved setting)")
	add := flag.String("add", "", "add a URL, wait for it to finish, then exit")
	checksum := flag.String("checksum", "", "with -add: the hash the file must have (sha256:<hex>, sha1, sha512, md5, or the hex alone)")
	nativeHost := flag.Bool("native-messaging", false, "run as a browser native-messaging host")
	open := flag.Bool("open", true, "open the UI window on start")
	noUI := flag.Bool("no-ui", false, "never open a UI window")
	showVersion := flag.Bool("version", false, "print the version and exit")
	hidden := flag.Bool("hidden", false, "start in the system tray without showing the window (used at login)")
	regBrowser := flag.Bool("register-browser", false, "register the browser extension's native host for this user and exit")
	unregBrowser := flag.Bool("unregister-browser", false, "remove the browser native host registration and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s %s (%s/%s, %s)\n", version.Name, version.Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}
	// A magnet link or a .torrent file given to ODM (opening one with it)
	// goes to the New Download window.
	openURL := openArg(flag.Args())

	baseDir := configBaseDir()
	if *nativeHost {
		runNativeHost(baseDir)
		return
	}
	if *regBrowser || *unregBrowser {
		os.Exit(runBrowserRegistration(*regBrowser, baseDir))
	}

	if f, err := os.OpenFile(filepath.Join(baseDir, "odm.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		log.SetOutput(f)
		defer f.Close()
	}

	// Claim the control socket before anything touches state: it is the
	// single-instance lock, and a second instance must not resume the same
	// downloads even briefly.
	sock, err := ipc.Listen(ipc.SocketPath(baseDir))
	if errors.Is(err, ipc.ErrRunning) {
		// Launching ODM again (e.g. from the Start menu while it sits in the
		// tray) should bring the running window forward, not do nothing.
		// -hidden (login, a browser download) asks for no window.
		if openURL != "" {
			// The running ODM opens its New Download window for it.
			body, _ := json.Marshal(map[string]any{"url": openURL, "prompt": true})
			resp, perr := ipc.Client(ipc.SocketPath(baseDir), 10*time.Second).Post(ipc.BaseURL+"/api/downloads", "application/json", bytes.NewReader(body))
			if perr == nil {
				resp.Body.Close()
			}
		} else if !*hidden {
			resp, perr := ipc.Client(ipc.SocketPath(baseDir), 3*time.Second).Post(ipc.BaseURL+"/api/show", "", nil)
			if perr == nil {
				resp.Body.Close()
			}
		}
		fmt.Fprintln(os.Stderr, "ODM is already running.")
		return
	}
	if err != nil {
		// Without the socket ODM still works; only browser hand-off and the
		// single-instance check are lost (e.g. Windows older than 10 1803).
		fmt.Fprintln(os.Stderr, "control socket unavailable, browser integration disabled:", err)
		log.Println("control socket:", err)
	}
	settingsPath := filepath.Join(baseDir, "settings.json")
	// No settings yet: ODM's first start for this user, when the desktop
	// UI offers its Setup window.
	_, serr := os.Stat(settingsPath)
	firstRun := errors.Is(serr, fs.ErrNotExist)
	fallback := defaultDownloadDir()
	settings := engine.LoadSettings(settingsPath, fallback)
	// Browsers hand downloads over through the control socket, so the
	// integration is set up only by the instance that holds it.
	if sock != nil && !settings.NoBrowserRegister {
		registerBrowsersIfNeeded(baseDir)
	}
	if *dir != "" {
		settings.Dir = *dir
	}
	if *conns > 0 {
		settings.Connections = *conns
	}
	if *files > 0 {
		settings.Concurrent = *files
	}

	store, err := engine.NewStore(filepath.Join(baseDir, "state"))
	if err != nil {
		log.Println("cannot create state dir:", err)
		os.Exit(1)
	}

	engine.SystemProxy = sysproxy.Func()
	mgr := engine.NewManager(settings, settingsPath)
	mgr.SetStore(store)
	if states, err := store.Load(); err == nil {
		mgr.Restore(states)
	}

	srv := server.New(mgr)
	mgr.SetOnChange(srv.Broadcast)
	srv.OnBrowser = mgr.NoteExtension
	showUI := make(chan struct{}, 1)
	srv.OnShow = func() {
		select {
		case showUI <- struct{}{}:
		default: // a show is already pending
		}
	}

	// yt-dlp, ffmpeg and odm-libtorrent: ODM's own copies, downloaded when
	// needed once the person agreed (Settings.ToolUpdates) and kept up to
	// date silently.
	keeper := tools.NewKeeper(&tools.Installer{Dir: tools.Dir(), Client: mgr.HTTPClient()},
		func() string { return mgr.Settings().ToolUpdates })
	keeper.Log = log.Printf
	engine.EnsureTool = keeper.Ensure

	// Torrents run in odm-libtorrent, started when the first one needs it.
	// Set before the queue starts, which may resume torrents.
	engine.TorrentDir = filepath.Join(baseDir, "torrents")
	torrents := &torrent.Session{
		Program:  func(ctx context.Context) (string, error) { return libtorrentProgram(ctx, keeper) },
		State:    filepath.Join(engine.TorrentDir, "session.dat"),
		Settings: func() map[string]any { return torrentSettings(mgr) },
		Log:      log.Printf,
	}
	engine.Torrents = torrents
	// The program saves every torrent's resume data before it exits.
	defer torrents.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mgr.Begin(ctx)
	go mgr.Start(ctx)
	go keeper.Run(ctx, 24*time.Hour)

	// Scheduler "when done" actions for headless runs. The desktop UI installs
	// its own handler, which asks for confirmation before shutting down.
	mgr.SetActionHandler(func(act engine.Action) {
		log.Printf("scheduler: %s (queue %q)", act.Kind, act.Queue)
		switch act.Kind {
		case engine.ActionOpen:
			_ = osutil.Open(act.Path)
		case engine.ActionSleep, engine.ActionHibernate:
			if err := osutil.Suspend(act.Kind == engine.ActionHibernate); err != nil {
				log.Printf("%s: %v", act.Kind, err)
			}
		case engine.ActionShutdown:
			if err := osutil.Shutdown(); err != nil {
				log.Println("shutdown:", err)
			}
			stop()
		case engine.ActionExit:
			stop()
		}
	})

	// The watcher always runs and reads the setting on each change, so the
	// Options toggle applies immediately. Without a native clipboard API
	// (Linux, macOS) the desktop UI polls the clipboard instead.
	if clipmon.Native {
		go clipmon.Watch(ctx, clipboardConfig(mgr), func(u string) {
			mgr.AddAuto(engine.AddOptions{URL: u, Start: true})
		})
	}

	// Control channel for the browser relay: same API, over the socket only.
	var servers []*http.Server
	if sock != nil {
		ctrl := &http.Server{Handler: srv.Handler()}
		servers = append(servers, ctrl)
		go func() {
			if err := ctrl.Serve(sock); err != nil && err != http.ErrServerClosed {
				log.Println("control socket:", err)
			}
		}()
	}
	stopServers := func() {
		for _, s := range servers {
			shutdown(s)
		}
	}

	if *add != "" {
		sum, err := engine.ParseChecksum(*checksum)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			stopServers()
			os.Exit(2)
		}
		o := engine.AddOptions{URL: *add, Dir: settings.Dir, Connections: settings.Connections, Checksum: sum, Start: true}
		if engine.IsTorrentFile(*add) {
			// A .torrent, by address or path, is downloaded as a torrent.
			if _, err := os.Stat(*add); err == nil {
				if abs, err := filepath.Abs(*add); err == nil {
					o.URL = abs
				}
			}
			o.Torrent = &engine.TorrentJob{}
		}
		info := mgr.Create(o)
		fmt.Printf("added %s (%s)\n", info.FileName, info.ID)
		waitFor(ctx, mgr, info.ID)
		stopServers()
		return
	}

	// The TCP listener is opt-in. Desktop builds need it only for -serve;
	// headless builds have no other UI, so asking for one starts it too.
	webUI := !ui.Available && *open && !*noUI
	webURL := ""
	if *serve || webUI {
		token := newToken()
		ln, err := net.Listen("tcp", *addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot serve the web UI on %s: %v\n", *addr, err)
			log.Println("web bind:", err)
			stopServers()
			return
		}
		web := &http.Server{Handler: server.Guard(token, srv.Handler())}
		servers = append(servers, web)
		go func() {
			if err := web.Serve(ln); err != nil && err != http.ErrServerClosed {
				log.Println("web server:", err)
			}
		}()
		webURL = "http://" + *addr + "/?token=" + token
		fmt.Printf("web UI: %s\n", webURL)
	}

	fmt.Printf("state: %s\ndownloads: %s\n", baseDir, settings.Dir)

	// On Windows the UI is a native Win32 window (no browser). The loopback API
	// stays up as an internal control channel for the browser relay.
	// Any build with the desktop UI compiled in uses it; only headless builds
	// fall back to the browser page.
	if ui.Available && *open && !*noUI {
		// Browser hand-offs open the New Download window. The hook exists
		// only while the window runs; headless ODM adds them directly.
		promptUI := make(chan engine.AddOptions, 16)
		if openURL != "" {
			promptUI <- openOptions(openURL)
		}
		srv.OnPrompt = func(o engine.AddOptions) bool {
			select {
			case promptUI <- o:
				return true
			default: // a burst of hand-offs: add the rest directly
				return false
			}
		}
		if err := ui.Run(ctx, mgr, ui.Options{Show: showUI, Prompt: promptUI, Hidden: *hidden, Tools: keeper, FirstRun: firstRun}); err != nil {
			log.Println("ui:", err)
		}
		stopServers()
		return
	}

	if openURL != "" {
		mgr.AddAuto(openOptions(openURL)) // no window to ask in
	}
	if webUI {
		openUI(webURL)
	}
	<-ctx.Done()
	stopServers()
}

// openArg is the magnet link or .torrent file among ODM's arguments (an
// absolute path), "" for none.
func openArg(args []string) string {
	if len(args) != 1 {
		return ""
	}
	a := strings.TrimSpace(args[0])
	switch {
	case engine.IsMagnet(a):
		return a
	case engine.IsTorrentFile(a):
		if abs, err := filepath.Abs(a); err == nil {
			return abs
		}
		return a
	}
	return ""
}

// openOptions is the download for a magnet link or .torrent file ODM was
// opened with.
func openOptions(u string) engine.AddOptions {
	o := engine.AddOptions{URL: u, Start: true}
	if !engine.IsMagnet(u) {
		o.Torrent = &engine.TorrentJob{}
	}
	return o
}

// libtorrentProgram is the odm-libtorrent to run: ODM_LIBTORRENT when set
// (developers), else ODM's own copy, downloaded first when the person
// allowed that.
func libtorrentProgram(ctx context.Context, keeper *tools.Keeper) (string, error) {
	if p := os.Getenv("ODM_LIBTORRENT"); p != "" {
		return p, nil
	}
	if p := tools.Find(tools.Libtorrent().Files[0]); p != "" {
		return p, nil
	}
	return keeper.Ensure(ctx, "odm-libtorrent")
}

// torrentSettings is odm-libtorrent's settings from ODM's. The port is
// picked the first time and kept, so that a rule for it in a router or a
// firewall lasts.
func torrentSettings(mgr *engine.Manager) map[string]any {
	st := mgr.Settings()
	if st.Torrent.Port == 0 {
		st.Torrent.Port = torrent.RandomPort()
		st = mgr.UpdateSettings(st)
	}
	return torrent.SettingsFrom(st, engine.SystemProxy)
}

// clipboardConfig reads the clipboard monitor's settings on every call.
func clipboardConfig(mgr *engine.Manager) func() clipmon.Config {
	return func() clipmon.Config {
		st := mgr.Settings()
		return clipmon.Config{Enabled: st.Clipboard, FileTypes: st.FileTypes}
	}
}

// selfExe is this ODM's executable path as registrations record it.
func selfExe() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	return exe, err
}

// registerBrowsersIfNeeded registers this ODM with the browsers at start-up
// when browserreg.NeedsRegister says so, so the extension works without
// running -register-browser first, and again after ODM is moved. Results
// go to the log; Options -> General shows the status.
func registerBrowsersIfNeeded(baseDir string) {
	exe, err := selfExe()
	if err != nil {
		log.Println("browser registration:", err)
		return
	}
	exists := func(p string) bool {
		fi, err := os.Stat(p)
		return err == nil && fi.Mode().IsRegular()
	}
	if !browserreg.NeedsRegister(browserreg.Check(), exe, exists) {
		return
	}
	for _, r := range browserreg.Register(exe, baseDir) {
		log.Println("browser registration:", r)
	}
}

// runBrowserRegistration handles -register-browser / -unregister-browser and
// returns the process exit code.
func runBrowserRegistration(register bool, baseDir string) int {
	var results []browserreg.Result
	if register {
		exe, err := selfExe()
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot locate the ODM executable:", err)
			return 1
		}
		results = browserreg.Register(exe, baseDir)
	} else {
		results = browserreg.Unregister()
	}
	code := 0
	for _, r := range results {
		fmt.Println(r)
		if r.Err != nil {
			code = 1
		}
	}
	if err := rememberBrowserChoice(baseDir, !register); err != nil {
		fmt.Fprintln(os.Stderr, "cannot save the setting:", err)
		code = 1
	}
	return code
}

// rememberBrowserChoice records whether ODM may register itself at
// start-up (off after -unregister-browser), in the running ODM when there
// is one, which would otherwise write its own settings over the file.
func rememberBrowserChoice(baseDir string, off bool) error {
	body := fmt.Sprintf(`{"no_browser_register":%t}`, off)
	req, _ := http.NewRequest(http.MethodPatch, ipc.BaseURL+"/api/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if resp, err := ipc.Client(ipc.SocketPath(baseDir), 3*time.Second).Do(req); err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		return fmt.Errorf("ODM returned %s", resp.Status)
	}
	path := filepath.Join(baseDir, "settings.json")
	st := engine.LoadSettings(path, defaultDownloadDir())
	st.NoBrowserRegister = off
	return engine.SaveSettings(path, st)
}

// newToken returns a random per-run secret for the web UI.
func newToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic("odm: no randomness for the web token: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func waitFor(ctx context.Context, mgr *engine.Manager, id string) {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	var last int64 = -1
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			info, ok := mgr.Get(id)
			if !ok {
				fmt.Println("download disappeared")
				return
			}
			if info.Done != last {
				last = info.Done
				fmt.Printf("\r  %s  %5.1f%%  %d/%d", info.Status, info.Progress, info.Done, info.Total)
			}
			switch info.Status {
			case engine.StatusComplete:
				fmt.Printf("\ndone: %s\n", info.FilePath)
				return
			case engine.StatusError:
				fmt.Printf("\nfailed: %s\n", info.Error)
				return
			}
		}
	}
}

func shutdown(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Downloads")
}

func configBaseDir() string {
	cfg, err := os.UserConfigDir()
	if err != nil {
		cfg = os.TempDir()
	}
	return filepath.Join(cfg, "odm")
}

// openUI shows the control panel. On Windows it launches Microsoft Edge in
// application mode, which renders the page with the installed WebView2 runtime.
func openUI(url string) {
	switch runtime.GOOS {
	case "windows":
		if edge := findEdge(); edge != "" {
			profile := filepath.Join(os.TempDir(), "odm-webview")
			if err := exec.Command(edge, "--app="+url, "--user-data-dir="+profile, "--no-first-run").Start(); err == nil {
				return
			}
		}
		_ = exec.Command("cmd", "/c", "start", "", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}

func findEdge() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
