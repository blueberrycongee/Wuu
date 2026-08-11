import { type Plugin } from "@wuu-v2/client-runtime";

const styles = `
:root {
  color-scheme: light;
  --paper: #f7f7f5;
  --paper-solid: #ffffff;
  --surface-2: rgba(31, 35, 40, 0.045);
  --surface-3: rgba(31, 35, 40, 0.075);
  --surface-4: rgba(31, 35, 40, 0.12);
  --hairline: rgba(31, 35, 40, 0.08);
  --hairline-strong: rgba(31, 35, 40, 0.12);
  --ink: #202423;
  --ink-muted: #747a77;
  --wuu-accent: #222725;
  --wuu-color-on-accent: #ffffff;
  --danger: #b1271b;
}

@media (prefers-color-scheme: dark) {
  :root {
    color-scheme: dark;
    --paper: #1d2024;
    --paper-solid: #24282d;
    --surface-2: rgba(255, 255, 255, 0.045);
    --surface-3: rgba(255, 255, 255, 0.075);
    --surface-4: rgba(255, 255, 255, 0.13);
    --hairline: rgba(255, 255, 255, 0.08);
    --hairline-strong: rgba(255, 255, 255, 0.13);
    --ink: #eceeec;
    --ink-muted: #9ca39f;
    --wuu-accent: #eceeec;
    --wuu-color-on-accent: #202423;
    --danger: #ff8577;
  }
}
`;

const defaultThemeClient: Plugin = function defaultTheme(client) {
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "theme-default";
    style.textContent = styles;
    document.head.append(style);
    return () => style.remove();
  }, "install default theme");
};

export default defaultThemeClient;
