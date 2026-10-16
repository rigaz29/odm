package gui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"odm/internal/engine"
)

func TestSchedulerSavesEachQueue(t *testing.T) {
	svc, mgr := newTestService(t)
	if err := svc.AddQueue("Night"); err != nil {
		t.Fatal(err)
	}
	data := svc.Scheduler()
	main, ok := data.Schedules[engine.DefaultQueue]
	if len(data.Queues) != 2 || !ok || main.StartAt != "23:00" || main.StopAt != "07:30" || len(main.Days) != 7 || main.RepeatMinutes != 60 {
		t.Fatalf("scheduler %+v", data)
	}
	night := data.Schedules["Night"]
	night.StartEnabled, night.StartAt = true, "22:30"
	night.RepeatEnabled, night.RepeatMinutes = true, 30
	night.Finish, night.Parallel = engine.ActionShutdown, 2
	night.Days = []bool{false, true, true, true, true, true, false}
	data.Schedules["Night"] = night
	data.Limits = Limits{On: true, MB: 700, Hours: 4, Warning: true}
	if r := svc.SaveScheduler(data.Schedules, data.Limits); r != nil {
		t.Fatalf("refused %+v", r)
	}
	st := mgr.Settings()
	sc := st.Schedules["Night"]
	if !sc.StartEnabled || sc.StartAt != "22:30" || sc.RepeatMinutes != 30 || sc.FinishAction() != engine.ActionShutdown || sc.Parallel != 2 || sc.Days[0] || !sc.Days[1] {
		t.Fatalf("Night %+v", sc)
	}
	if !st.Limits.Enabled || st.Limits.MB != 700 || st.Limits.Hours != 4 || st.Limits.NoWarning {
		t.Fatalf("limits %+v", st.Limits)
	}
	if got := svc.Scheduler().Schedules["Night"]; !got.RepeatEnabled || got.Finish != engine.ActionShutdown || got.StartAt != "22:30" {
		t.Fatalf("read back %+v", got)
	}

	for _, c := range []struct {
		edit  func(*Schedule)
		field string
	}{
		{func(s *Schedule) { s.StartEnabled, s.StartAt = true, "25:00" }, "start"},
		{func(s *Schedule) { s.StopEnabled, s.StopAt = true, "soon" }, "stop"},
		{func(s *Schedule) { s.RepeatEnabled, s.RepeatMinutes = true, 0 }, "repeat"},
	} {
		d := svc.Scheduler()
		m := d.Schedules[engine.DefaultQueue]
		c.edit(&m)
		d.Schedules[engine.DefaultQueue] = m
		if r := svc.SaveScheduler(d.Schedules, d.Limits); r == nil || r.Field != c.field || r.Queue != engine.DefaultQueue {
			t.Errorf("%s: %+v", c.field, r)
		}
	}
	if mgr.Settings().Schedules[engine.DefaultQueue].StartEnabled {
		t.Fatal("a refused schedule was saved")
	}
}

func TestQueuesComeAndGo(t *testing.T) {
	svc, mgr := newTestService(t)
	if err := svc.AddQueue(" "); err == nil {
		t.Fatal("a queue without a name")
	}
	if err := svc.AddQueue("Night"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddQueue("night"); err == nil {
		t.Fatal("the same queue twice")
	}
	d := mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/n.zip", FileName: "n.zip", Queue: "Night"})
	if err := svc.RemoveQueue(engine.DefaultQueue); err == nil {
		t.Fatal("the main queue was removed")
	}
	if err := svc.RemoveQueue("Night"); err != nil {
		t.Fatal(err)
	}
	if got, _ := mgr.Get(d.ID); got.Queue != engine.DefaultQueue {
		t.Fatalf("the download stayed in %q", got.Queue)
	}
}

func TestSchedulerRequests(t *testing.T) {
	svc, mgr := newTestService(t)
	w := &stubWindows{}
	svc.windows = w
	st := mgr.Settings()
	st.Limits = engine.DownloadLimits{Enabled: true, MB: 300, Hours: 2}
	mgr.UpdateSettings(st)
	svc.handleAction(engine.Action{Kind: engine.ActionLimit})
	svc.handleAction(engine.Action{Kind: engine.ActionShutdown})
	if got := w.got(); got != "notice hours=2&kind=limit&mb=300, notice action=shutdown&kind=finish" {
		t.Fatalf("asked %q", got)
	}
	quit := 0
	svc.quit = func() { quit++ }
	if err := svc.RunFinish("bogus"); err == nil || quit != 0 {
		t.Fatal("an unknown action ran")
	}
	if err := svc.RunFinish(engine.ActionExit); err != nil || quit != 1 {
		t.Fatalf("exit: %v, quit %d", err, quit)
	}
}

func TestExportAndImport(t *testing.T) {
	svc, mgr := newTestService(t)
	w := &stubWindows{}
	svc.windows = w
	mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/e.iso", FileName: "e.iso"})
	if path, err := svc.ExportList(); path != "" || err != nil {
		t.Fatalf("a cancelled export wrote %q, %v", path, err)
	}
	w.save = filepath.Join(t.TempDir(), "list")
	path, err := svc.ExportList()
	if err != nil || path != w.save+".json" {
		t.Fatalf("exported to %q: %v", path, err)
	}

	other, otherMgr := newTestService(t)
	other.windows = &stubWindows{open: path}
	if n, err := other.ImportList(); n != 1 || err != nil {
		t.Fatalf("imported %d: %v", n, err)
	}
	if list := otherMgr.List(); len(list) != 1 || list[0].FileName != "e.iso" {
		t.Fatalf("list %+v", list)
	}
	if n, _ := svc.ImportList(); n != -1 {
		t.Fatalf("a cancelled import gave %d", n)
	}
}

func TestGrabAddsTheFilesASiteLinks(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<a href="%s/files/a.zip">a</a> <a href="/files/b.pdf">b</a> <a href="/page.html">p</a>`, srv.URL)
		case "/page.html":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<a href="/files/c.zip">c</a>`)
		default:
			fmt.Fprint(w, "x")
		}
	}))
	t.Cleanup(srv.Close)
	svc, mgr := newTestService(t)
	var mu sync.Mutex
	events := 0
	svc.emit = func(name string, _ any) {
		if name == GrabEvent {
			mu.Lock()
			events++
			mu.Unlock()
		}
	}
	res, err := svc.Grab(GrabRequest{URL: srv.URL + "/", Depth: 1, MaxPages: 10, MaxFiles: 10, SameHost: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Files != 3 || res.Pages < 2 || len(mgr.List()) != 3 {
		t.Fatalf("grabbed %+v, list %d", res, len(mgr.List()))
	}
	mu.Lock()
	if events == 0 {
		t.Fatal("no progress was sent")
	}
	mu.Unlock()
	if _, err := svc.Grab(GrabRequest{URL: " "}); err == nil {
		t.Fatal("a crawl without a start page")
	}
	if err := svc.OpenURL("file:///C:/Windows"); err == nil || !strings.Contains(err.Error(), "web") {
		t.Fatalf("OpenURL took a file: %v", err)
	}
}
