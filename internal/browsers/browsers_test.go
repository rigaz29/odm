package browsers

import "testing"

func TestIdentify(t *testing.T) {
	cases := []struct {
		name, command      string
		key, family, title string
	}{
		{"Google Chrome", `"C:\Program Files\Google\Chrome\Application\chrome.exe"`, "chrome", Chromium, "Google Chrome"},
		{"Microsoft Edge", `"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"`, "edge", Chromium, "Microsoft Edge"},
		{"Brave", `"C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe"`, "brave", Chromium, "Brave"},
		{"Opera Stable", `"C:\Users\u\AppData\Local\Programs\Opera\launcher.exe"`, "opera", Chromium, "Opera Stable"},
		{"Chromium", `C:\Users\u\AppData\Local\Chromium\Application\chrome.exe`, "chromium", Chromium, "Chromium"},
		{"Mozilla Firefox", `"C:\Program Files\Mozilla Firefox\firefox.exe"`, "firefox", Firefox, "Mozilla Firefox"},
		{"Zen Browser", `"C:\Program Files\Zen Browser\zen.exe"`, "zen", Firefox, "Zen Browser"},
		{"", `C:\Program Files\Vivaldi\Application\vivaldi.exe --flag`, "vivaldi", Chromium, "Vivaldi"},
	}
	for _, c := range cases {
		b, ok := identify(c.name, c.command)
		if !ok || b.Key != c.key || b.Family != c.family || b.Name != c.title {
			t.Errorf("identify(%q, %q) = %+v, %v", c.name, c.command, b, ok)
		}
	}
	if b, _ := identify("Brave", `"C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe" --x`); b.Exe != `C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe` {
		t.Errorf("exe = %q", b.Exe)
	}
	if b, ok := identify("Internet Explorer", `C:\Program Files\Internet Explorer\iexplore.exe`); ok {
		t.Errorf("Internet Explorer listed: %+v", b)
	}
	if b, ok := identify("Launcher", `"C:\Games\launcher.exe"`); ok {
		t.Errorf("a launcher that is not Opera's listed: %+v", b)
	}
}

func TestUnique(t *testing.T) {
	list := unique([]Browser{
		{Key: "firefox", Name: "Mozilla Firefox", Exe: "user"},
		{Key: "chrome", Name: "Google Chrome"},
		{Key: "firefox", Name: "Mozilla Firefox", Exe: "machine"},
	})
	if len(list) != 2 || list[0].Key != "chrome" || list[1].Exe != "user" {
		t.Fatalf("unique: %+v", list)
	}
}

func TestExtensionsPage(t *testing.T) {
	for key, want := range map[string]string{"chrome": "chrome://extensions", "chromium": "chrome://extensions",
		"edge": "edge://extensions", "brave": "brave://extensions", "opera": "opera://extensions", "vivaldi": "vivaldi://extensions"} {
		if got := (Browser{Key: key, Family: Chromium}).ExtensionsPage(); got != want {
			t.Errorf("%s: %q", key, got)
		}
	}
	if got := (Browser{Key: "firefox", Family: Firefox}).ExtensionsPage(); got != "" {
		t.Errorf("firefox: %q", got)
	}
}
