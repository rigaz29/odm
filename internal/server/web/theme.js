// Applies the saved theme before the page paints, so it does not flash.
// "system" follows the browser's light or dark setting.
try {
  document.documentElement.dataset.theme = localStorage.getItem("odm-theme") || "system";
} catch (e) {
  document.documentElement.dataset.theme = "system";
}
