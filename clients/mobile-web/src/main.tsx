import { createRoot } from "react-dom/client";

import App from "./App";
import "./styles.css";

const root = document.getElementById("root");
if (!root) throw new Error("missing #root");
root.dataset.wuuUiRoot = "true";
root.dataset.wuuComponent = "ui-root";

createRoot(root).render(<App />);
