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
const { orderMedia, mediaLabel, ago } = self.odmMediaList;
const mediaBox = document.getElementById("media");

chrome.tabs.query({ active: true, currentWindow: true }).then(([tab]) => {
  if (!tab) return;
  chrome.runtime.sendMessage({ type: "odm-tab-media", tabId: tab.id }, (list) => {
    if (chrome.runtime.lastError || !Array.isArray(list) || list.length === 0) return;
    const now = Date.now();
    mediaBox.replaceChildren();
    for (const it of orderMedia(list)) {
      const row = document.createElement("button");
      row.title = it.url;
      const name = document.createElement("span");
      name.textContent = mediaLabel(it);
      const when = document.createElement("span");
      when.className = "when";
      when.textContent = ago(it.at, now);
      row.append(name, when);
      row.onclick = () => {
        status.textContent = "Sending…";
        chrome.runtime.sendMessage({ type: "odm-grab", item: it, tabId: tab.id }, (reply) => {
          if (chrome.runtime.lastError || !reply) status.textContent = "ODM is not reachable";
          else if (!reply.ok) status.textContent = `ODM: ${reply.error}`;
          else status.textContent = reply.message === "already in the list" ? "Already in ODM" : "Sent to ODM";
        });
      };
      mediaBox.appendChild(row);
    }
  });
});
