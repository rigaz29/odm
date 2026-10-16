import { createRoot } from "react-dom/client";
import { App } from "./App";
import { TooltipProvider } from "./components/ui/tooltip";
import "./styles.css";

createRoot(document.getElementById("root")!).render(
  <TooltipProvider delayDuration={400}>
    <App />
  </TooltipProvider>,
);
