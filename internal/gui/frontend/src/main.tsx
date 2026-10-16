// Every ODM window loads this page; ?view= says which one it is: the main
// window (none), New Download (add), a download's progress (progress) or
// the download complete window (complete). See internal/gui/windows.go.

import { createRoot } from "react-dom/client";
import { App } from "./App";
import { useAppearance } from "./appearance";
import { TooltipProvider } from "./components/ui/tooltip";
import "./styles.css";
import { AddWindow } from "./views/AddWindow";
import { CompleteWindow } from "./views/CompleteWindow";
import { ProgressWindow } from "./views/ProgressWindow";

const q = new URLSearchParams(location.search);

function Window() {
  const appearance = useAppearance();
  if (!appearance) return null;
  switch (q.get("view")) {
    case "add":
      return <AddWindow token={q.get("token") ?? ""} />;
    case "progress":
      return <ProgressWindow id={q.get("id") ?? ""} />;
    case "complete":
      return <CompleteWindow id={q.get("id") ?? ""} />;
  }
  return <App />;
}

createRoot(document.getElementById("root")!).render(
  <TooltipProvider delayDuration={400}>
    <Window />
  </TooltipProvider>,
);
