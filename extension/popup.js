const enabled = document.getElementById("enabled");
const status = document.getElementById("status");

chrome.storage.local.get("enabled").then(({ enabled: v = true }) => (enabled.checked = v));

enabled.onchange = () => chrome.storage.local.set({ enabled: enabled.checked });

document.getElementById("test").onclick = () => {
  try {
    const port = chrome.runtime.connectNative("com.ochi.odm");
    port.onMessage.addListener((msg) => {
      status.textContent = msg && msg.ok ? "ODM is reachable" : `ODM: ${(msg && msg.error) || "no reply"}`;
      port.disconnect();
    });
    port.onDisconnect.addListener(() => {
      if (chrome.runtime.lastError) status.textContent = "Cannot reach ODM (is it running?)";
    });
    port.postMessage({ type: "ping" });
  } catch (e) {
    status.textContent = String(e);
  }
};

// Media the tab has loaded, from every frame: also what the in-page button
// cannot offer (audio-only pages, canvas players, a frame without a large
// <video>, or the button closed with ✕).
const { menuEntries, sitePage, siteName, siteTitle, ago } = self.odmMediaList;
const mediaBox = document.getElementById("media");

chrome.tabs.query({ active: true, currentWindow: true }).then(([tab]) => {
  if (!tab) return;
  chrome.runtime.sendMessage({ type: "odm-tab-media", tabId: tab.id }, (found) => {
    let list = chrome.runtime.lastError || !Array.isArray(found) ? [] : found;
    // The video of a site's page (YouTube, TikTok), which yt-dlp downloads.
    // A live stream its player reads keeps the page, where yt-dlp finds
    // its new address when this one expires.
    const page = sitePage(tab.url || "");
    if (page) {
      list = list.map((it) => (it.kind === "live" && !it.sitePage ? { ...it, sitePage: page } : it));
      list = [{ kind: "site", site: siteName(page), url: page, title: siteTitle(tab.title), frameId: 0 }, ...list];
    }
    if (list.length === 0) return;
    // ODM's replies about the streams (their qualities), by playlist URL.
    const infos = new Map();
    const render = () => {
      const now = Date.now();
      mediaBox.replaceChildren();
      for (const e of menuEntries(list, infos)) {
        const row = document.createElement("button");
        if (e.heading) row.className = "head";
        if (e.sub) row.classList.add("sub");
        row.disabled = !e.item;
        if (e.item) row.title = e.item.url;
        const name = document.createElement("span");
        name.textContent = e.text;
        const when = document.createElement("span");
        when.className = "when";
        when.textContent = e.sub ? "" : ago(e.at, now);
        row.append(name, when);
        if (e.item) row.onclick = () => grab(e.item);
        mediaBox.appendChild(row);
      }
    };
    const grab = (item) => {
      status.textContent = "Sending…";
      chrome.runtime.sendMessage({ type: "odm-grab", item, tabId: tab.id }, (reply) => {
        if (chrome.runtime.lastError || !reply) status.textContent = "ODM is not reachable";
        else if (!reply.ok) status.textContent = `ODM: ${reply.error}`;
        else status.textContent = reply.message === "already in the list" ? "Already in ODM" : "Sent to ODM";
      });
    };
    for (const it of list) {
      if (it.kind === "file" || it.kind === "live") continue;
      chrome.runtime.sendMessage({ type: "odm-media-info", item: it, tabId: tab.id }, (reply) => {
        infos.set(it.url, chrome.runtime.lastError || !reply ? { ok: false, error: "ODM is not reachable" } : reply);
        render();
      });
    }
    render();
  });
});
