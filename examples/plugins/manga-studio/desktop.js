export function activate(api) {
  const React = api.react;

  api.registerStyle({
    id: "manga-studio-design",
    order: 100,
    css: `
      .manga-shell {
        --manga-ink: #17130f;
        --manga-paper: #fffdf2;
        --manga-paper-deep: #f1ead8;
        --manga-pink: #ff3f7f;
        --manga-cyan: #39c7d4;
        --manga-yellow: #ffd83d;
        display: contents;
        color: var(--manga-ink);
        font-family: var(--wuu-font-family-ui, "Avenir Next Condensed", Arial, sans-serif);
      }
      body:has(.manga-shell) {
        --manga-ink: #17130f;
        --manga-paper: #fffdf2;
        --manga-paper-deep: #f1ead8;
        --manga-pink: #ff3f7f;
        --manga-cyan: #39c7d4;
        --manga-yellow: #ffd83d;
        color: var(--manga-ink);
        background-color: var(--manga-paper);
        background-image:
          radial-gradient(circle at 1px 1px, rgb(23 19 15 / .12) 1px, transparent 1.2px),
          linear-gradient(112deg, transparent 0 78%, rgb(255 63 127 / .07) 78% 79%, transparent 79%);
        background-size: 13px 13px, 100% 100%;
        font-family: var(--wuu-font-family-ui, "Avenir Next Condensed", Arial, sans-serif);
      }
      .manga-shell *, .manga-shell *::before, .manga-shell *::after { box-sizing: border-box; }
      .manga-shell button,
      .manga-shell input,
      .manga-shell textarea,
      .manga-shell select { font-family: inherit; }
      .manga-shell input,
      .manga-shell textarea,
      .manga-shell select {
        border-color: var(--manga-ink) !important;
        border-radius: 3px !important;
      }
      .manga-shell :focus-visible { outline: 3px solid var(--manga-pink) !important; outline-offset: 2px; }
      .manga-shell ::selection { color: var(--manga-ink); background: var(--manga-yellow); }
      .manga-shell ::-webkit-scrollbar { width: 10px; height: 10px; }
      .manga-shell ::-webkit-scrollbar-track { background: var(--manga-paper-deep); border-left: 1px solid var(--manga-ink); }
      .manga-shell ::-webkit-scrollbar-thumb { background: var(--manga-ink); border: 2px solid var(--manga-paper-deep); }
      .manga-shell [role="dialog"],
      .manga-shell [role="menu"],
      .manga-shell [role="listbox"] {
        border: 2px solid var(--manga-ink) !important;
        border-radius: 3px !important;
        box-shadow: 6px 6px 0 var(--manga-ink) !important;
      }
      body:has(.manga-shell) [data-wuu-component="dialog"],
      body:has(.manga-shell) [data-wuu-component="menu"],
      body:has(.manga-shell) [data-wuu-layer="popover"],
      body:has(.manga-shell) [data-wuu-layer="listbox"] {
        border: 2px solid var(--manga-ink) !important;
        border-radius: 3px !important;
        box-shadow: 6px 6px 0 var(--manga-ink) !important;
      }
      body:has(.manga-shell) [data-wuu-component="jump-to-latest"] {
        color: var(--manga-ink);
        background: var(--manga-yellow);
        border: 2px solid var(--manga-ink);
        border-radius: 3px;
        box-shadow: 4px 4px 0 var(--manga-pink);
        backdrop-filter: none;
        -webkit-backdrop-filter: none;
      }
      body:has(.manga-shell) [data-wuu-component="jump-to-latest"]::before {
        content: none;
      }
      body:has(.manga-shell) [data-wuu-component="jump-to-latest"]:hover {
        background: var(--manga-cyan);
        border-color: var(--manga-ink);
        box-shadow: 4px 4px 0 var(--manga-pink);
      }

      /* These data-wuu-* coordinates are the host's published styling API.
         Keep the real Wuu components mounted so Markdown, streaming, actions,
         keyboard behavior, and nested process layouts remain intact. */
      .manga-shell [data-wuu-component="sidebar"] {
        color: var(--manga-ink);
        background: rgb(255 253 242 / .96);
        border-right: 3px solid var(--manga-ink);
      }
      .manga-shell [data-wuu-component="app-shell"][data-wuu-sidebar-mode="drawer"]
        [data-wuu-component="sidebar"] {
        background-color: var(--manga-paper);
        background-image: radial-gradient(circle at 1px 1px, rgb(23 19 15 / .1) 1px, transparent 1.2px);
        background-size: 13px 13px;
        border-right-color: var(--manga-ink);
        box-shadow: 7px 0 0 var(--manga-cyan);
      }
      .manga-shell [data-wuu-component="conversation-pane"],
      .manga-shell [data-wuu-component="empty-session"],
      .manga-shell [data-wuu-component="settings-shell"],
      .manga-shell [data-wuu-component="launch-view"],
      .manga-shell [data-wuu-component="skills-catalog"] {
        color: var(--manga-ink);
        background-color: rgb(255 253 242 / .94);
        background-image: radial-gradient(circle at 1px 1px, rgb(23 19 15 / .08) 1px, transparent 1.2px);
        background-size: 13px 13px;
      }
      .manga-shell [data-wuu-component="conversation-pane"] {
        --wuu-message-actions-block-gap: 14px;
        --wuu-message-actions-overlay-gap: 8px;
        --wuu-message-actions-control-gap: 8px;
        --wuu-message-actions-inline-offset: 2px;
        --wuu-message-action-size: 28px;
        --wuu-message-action-radius: 3px;
        --wuu-message-user-background: var(--manga-yellow);
        --wuu-message-user-border: 2px solid var(--manga-ink);
        --wuu-message-user-color: var(--manga-ink);
        --wuu-message-user-radius: 3px;
        --wuu-message-user-shadow: 4px 4px 0 var(--manga-pink);
      }
      .manga-shell [data-wuu-component="plugin-navigation"] {
        border-block: 2px solid var(--manga-ink);
        background: rgb(67 229 237 / .16);
      }
      .manga-shell [data-wuu-component="plugin-navigation-item"][aria-current="page"] {
        color: var(--manga-ink);
        background: var(--manga-cyan);
        box-shadow: inset 5px 0 0 var(--manga-pink);
      }
      .manga-shell [data-wuu-component="settings-navigation-item"][aria-current="page"] {
        color: var(--manga-ink);
        background: var(--manga-yellow);
        border: 2px solid var(--manga-ink);
        box-shadow: 3px 3px 0 var(--manga-pink);
      }
      .manga-shell [data-wuu-component="settings-card"],
      .manga-shell [data-wuu-component="plugin-settings"] {
        border: 2px solid var(--manga-ink);
        background: var(--manga-paper);
        box-shadow: 4px 4px 0 var(--manga-cyan);
      }
      .manga-shell [data-wuu-component="settings-row"] + [data-wuu-component="settings-row"] {
        border-top: 2px solid var(--manga-ink);
      }
      .manga-shell [data-wuu-component="workspace-panel"],
      .manga-shell [data-wuu-component="workspace-review"],
      .manga-shell [data-wuu-component="workspace-terminal"],
      .manga-shell [data-wuu-component="workspace-browser"] {
        border: 2px solid var(--manga-ink);
        box-shadow: 4px 4px 0 var(--manga-cyan);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-panel"] {
        color: var(--manga-ink);
        background: var(--manga-paper);
        border-left: 3px solid var(--manga-ink);
        box-shadow: -7px 0 0 var(--manga-cyan);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-panel-header"] {
        color: var(--manga-paper);
        background: var(--manga-ink);
        border-bottom: 4px solid var(--manga-pink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-files"] {
        color: var(--manga-ink);
        background-color: var(--manga-paper);
        background-image: radial-gradient(circle at 1px 1px, rgb(23 19 15 / .09) 1px, transparent 1.2px);
        background-size: 13px 13px;
      }
      body:has(.manga-shell) [data-wuu-component="workspace-file-content"] {
        background: rgb(255 253 242 / .9);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-file-tree"] {
        --wuu-workspace-file-tree-color: var(--manga-ink);
        --wuu-workspace-file-tree-muted-color: rgb(23 19 15 / .62);
        --wuu-workspace-file-tree-background: transparent;
        --wuu-workspace-file-tree-muted-background: rgb(255 216 61 / .2);
        --wuu-workspace-file-tree-search-background: white;
        --wuu-workspace-file-tree-selected-color: var(--manga-ink);
        --wuu-workspace-file-tree-selected-background: var(--manga-yellow);
        --wuu-workspace-file-tree-selected-border: var(--manga-ink);
        --wuu-workspace-file-tree-border-color: var(--manga-ink);
        --wuu-workspace-file-tree-font-family: var(--wuu-font-family-ui, "Avenir Next Condensed", Arial, sans-serif);
        --wuu-workspace-file-tree-search-border: 2px solid var(--manga-ink);
        --wuu-workspace-file-tree-search-radius: 3px;
        background-color: rgb(241 234 216 / .82);
        background-image: radial-gradient(circle at 1px 1px, rgb(23 19 15 / .1) 1px, transparent 1.2px);
        background-size: 13px 13px;
        border-left: 3px solid var(--manga-ink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-empty-icon"] {
        color: var(--manga-ink);
        background: var(--manga-yellow);
        border: 2px solid var(--manga-ink);
        border-radius: 3px;
        box-shadow: 3px 3px 0 var(--manga-cyan);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-empty-state"] strong {
        color: var(--manga-ink);
        font-weight: 900;
      }
      body:has(.manga-shell) [data-wuu-component="workspace-review"] {
        color: var(--manga-ink);
        background-color: var(--manga-paper);
        background-image: radial-gradient(circle at 1px 1px, rgb(23 19 15 / .09) 1px, transparent 1.2px);
        background-size: 13px 13px;
      }
      body:has(.manga-shell) [data-wuu-component="workspace-review-navigation"] {
        background: rgb(241 234 216 / .86);
        border-left: 3px solid var(--manga-ink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-review-search"] {
        background: white;
        border: 2px solid var(--manga-ink);
        border-radius: 3px;
      }
      body:has(.manga-shell) [data-wuu-component="workspace-review-item"] {
        color: var(--manga-ink);
        border-bottom: 1px solid rgb(23 19 15 / .14);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-review-item"]:hover {
        background: rgb(57 199 212 / .28);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-review-item"][data-wuu-active="true"] {
        background: var(--manga-yellow);
        box-shadow: inset 4px 0 0 var(--manga-pink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-review-content"] {
        background: rgb(255 253 242 / .94);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-review-content-header"] {
        background: var(--manga-yellow);
        border-bottom: 3px solid var(--manga-ink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-terminal-layout"] {
        --wuu-workspace-terminal-background: var(--manga-ink);
        --wuu-workspace-terminal-foreground: var(--manga-paper);
        --wuu-workspace-terminal-black: #050403;
        --wuu-workspace-terminal-blue: var(--manga-cyan);
        --wuu-workspace-terminal-cursor: var(--manga-pink);
        --wuu-workspace-terminal-green: var(--manga-cyan);
        --wuu-workspace-terminal-red: var(--manga-pink);
        --wuu-workspace-terminal-selection: rgb(255 216 61 / .3);
        --wuu-workspace-terminal-yellow: var(--manga-yellow);
        --wuu-workspace-terminal-font-family: var(--wuu-font-family-mono, "SFMono-Regular", Consolas, monospace);
        background: var(--manga-ink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-terminal-navigation"] {
        color: var(--manga-ink);
        background-color: var(--manga-paper-deep);
        background-image: radial-gradient(circle at 1px 1px, rgb(23 19 15 / .1) 1px, transparent 1.2px);
        background-size: 13px 13px;
        border-right: 3px solid var(--manga-ink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-terminal-item"] {
        color: var(--manga-ink);
        border-bottom: 1px solid rgb(23 19 15 / .16);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-terminal-item"][data-wuu-active="true"] {
        background: var(--manga-yellow);
        box-shadow: inset 4px 0 0 var(--manga-pink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-terminal-content"],
      body:has(.manga-shell) [data-wuu-component="workspace-terminal-screen"] {
        background: var(--manga-ink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-browser"] {
        color: var(--manga-ink);
        background: var(--manga-paper);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-browser-toolbar"] {
        background: var(--manga-paper-deep);
        border-bottom: 3px solid var(--manga-ink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-browser-toolbar"] > button {
        color: var(--manga-ink);
        border: 2px solid var(--manga-ink);
        background: var(--manga-yellow);
        box-shadow: 2px 2px 0 var(--manga-cyan);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-browser-address"] {
        color: var(--manga-ink);
        background: white;
        border: 2px solid var(--manga-ink);
        border-radius: 3px;
      }
      body:has(.manga-shell) [data-wuu-component="workspace-browser-content"] {
        background-color: var(--manga-paper);
        background-image: radial-gradient(circle at 1px 1px, rgb(23 19 15 / .09) 1px, transparent 1.2px);
        background-size: 13px 13px;
      }
      body:has(.manga-shell) [data-wuu-component="workspace-browser-content"] > [data-wuu-component="workspace-empty-state"] {
        background-color: transparent;
        background-image: radial-gradient(circle at 1px 1px, rgb(23 19 15 / .09) 1px, transparent 1.2px);
        background-size: 13px 13px;
      }
      body:has(.manga-shell) [data-wuu-component="workspace-browser-statusbar"] {
        color: var(--manga-paper);
        background: var(--manga-ink);
        border-top: 3px solid var(--manga-pink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-tool-picker"] {
        gap: 16px;
        padding: 18px;
        background-color: var(--manga-paper);
        background-image: radial-gradient(circle at 1px 1px, rgb(23 19 15 / .1) 1px, transparent 1.2px);
        background-size: 13px 13px;
      }
      body:has(.manga-shell) [data-wuu-component="workspace-tool"] {
        color: var(--manga-ink);
        background: var(--manga-paper);
        border: 3px solid var(--manga-ink);
        border-radius: 3px;
        box-shadow: 5px 5px 0 var(--manga-ink);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-tool"]:nth-child(even) {
        box-shadow: 5px 5px 0 var(--manga-cyan);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-tool"]:hover,
      body:has(.manga-shell) [data-wuu-component="workspace-tool"].active {
        color: var(--manga-ink);
        background: var(--manga-yellow);
      }
      body:has(.manga-shell) [data-wuu-component="workspace-tool"] svg {
        color: var(--manga-ink);
      }
      .manga-shell [data-wuu-component="side-thread"] {
        color: var(--manga-ink);
        background: var(--manga-paper);
        border-left: 3px solid var(--manga-ink);
        box-shadow: inset 7px 0 0 rgb(57 199 212 / .2);
      }
      .manga-shell [data-wuu-component="side-thread"] > header {
        color: var(--manga-paper);
        background: var(--manga-ink);
        border-bottom: 4px solid var(--manga-cyan);
      }
      .manga-shell [data-wuu-component="side-thread"] > header button {
        color: var(--manga-paper);
        border: 1px solid var(--manga-paper);
        box-shadow: 2px 2px 0 var(--manga-pink);
      }
      .manga-shell [data-wuu-component="side-thread"] > [role="log"] {
        background-image: radial-gradient(circle at 1px 1px, rgb(23 19 15 / .09) 1px, transparent 1.2px);
        background-size: 13px 13px;
      }
      .manga-shell [data-wuu-component="side-thread"] > [role="separator"]::before {
        background: var(--manga-cyan);
      }
      .manga-shell [data-wuu-component="environment-panel"] {
        color: var(--manga-ink);
        background: var(--manga-paper);
        border: 3px solid var(--manga-ink);
        border-radius: 3px;
        box-shadow: 7px 7px 0 var(--manga-cyan);
        backdrop-filter: none;
      }
      .manga-shell [data-wuu-component="environment-panel"] > section {
        border-bottom: 3px solid var(--manga-ink);
      }
      .manga-shell [data-wuu-component="environment-panel"] ol > li > span:first-child {
        border-color: var(--manga-ink);
        border-radius: 2px;
        background: var(--manga-yellow);
      }
      .manga-shell [data-wuu-component="environment-panel"] ol > li:has(svg) > span:first-child {
        color: white;
        background: var(--manga-cyan);
      }
      .manga-shell [data-wuu-component="environment-panel"] button {
        border-bottom: 1px solid rgb(23 19 15 / .18);
      }
      .manga-shell [data-wuu-component="environment-panel"] button:not(:disabled):hover {
        color: var(--manga-ink);
        background: var(--manga-yellow);
      }
      .manga-shell [data-wuu-component="turn"] {
        width: 100%;
        min-width: 0;
        padding: 18px 18px 12px;
        border: 3px solid var(--manga-ink);
        background: rgb(255 253 242 / .92);
        box-shadow: 7px 7px 0 var(--manga-ink);
      }
      .manga-shell [data-wuu-component="turn"]:nth-of-type(even) {
        box-shadow: 7px 7px 0 var(--manga-cyan);
      }
      .manga-shell [data-wuu-component="message"] {
        position: relative;
        min-width: 0;
      }
      .manga-shell [data-wuu-component="message-bubble"][data-wuu-variant="user"] {
        padding: 10px 12px;
      }
      .manga-shell [data-wuu-component="message"][data-wuu-variant="agent"] {
        padding-left: 14px;
        border-left: 5px solid var(--manga-ink);
      }
      .manga-shell [data-wuu-component="message-actions"][data-wuu-placement="overlay"] {
        z-index: 2;
        padding: 5px 6px;
        background: var(--manga-paper);
        border: 2px solid var(--manga-ink);
        box-shadow: 3px 3px 0 var(--manga-cyan);
      }
      .manga-shell [data-wuu-component="message-actions"] > button {
        color: var(--manga-ink);
        background: var(--manga-yellow);
        border: 2px solid var(--manga-ink) !important;
        box-shadow: 2px 2px 0 var(--manga-pink);
      }
      .manga-shell [data-wuu-component="message-actions"] > button:hover,
      .manga-shell [data-wuu-component="message-actions"] > button[aria-pressed="true"] {
        color: var(--manga-ink);
        background: var(--manga-cyan);
        box-shadow: 2px 2px 0 var(--manga-pink);
      }
      .manga-shell [data-wuu-component="composer"] {
        border: 0;
        background: transparent;
        box-shadow: none;
      }
      .manga-shell [data-wuu-component="composer-frame"] {
        border: 3px solid var(--manga-ink);
        background: var(--manga-paper);
        box-shadow: 6px 6px 0 var(--manga-pink);
      }
      .manga-shell [data-wuu-component="composer-input"] {
        border: 0 !important;
        background: transparent;
      }
      .manga-shell [data-wuu-component="composer-toolbar"] {
        border-top: 0;
        background: transparent;
      }
      .manga-shell [data-wuu-component="composer-send"] {
        color: white;
        background: var(--manga-pink);
        border: 2px solid var(--manga-ink) !important;
        box-shadow: 2px 2px 0 var(--manga-ink);
      }
      .manga-shell [data-wuu-component="conversation-pane"] > .titlebar {
        color: var(--manga-paper);
        background: var(--manga-ink);
        border-bottom: 4px solid var(--manga-pink);
      }
      .manga-shell [data-wuu-component="sidebar-toggle"] {
        color: var(--manga-paper);
        background: #332d26;
        border: 1px solid var(--manga-paper) !important;
        box-shadow: 2px 2px 0 var(--manga-pink);
      }
      .manga-shell [data-wuu-component="conversation-pane"] > .titlebar [data-wuu-component="session-tab"] {
        color: var(--manga-paper);
        background: #332d26;
        border: 1px solid var(--manga-paper);
        border-radius: 3px;
        box-shadow: 2px 2px 0 var(--manga-pink);
      }
      .manga-shell [data-wuu-component="conversation-pane"] > .titlebar [data-wuu-component="session-tab"][data-wuu-active="true"] {
        color: var(--manga-ink);
        background: var(--manga-yellow);
      }
      .manga-shell [data-wuu-component="conversation-pane"] > .titlebar [data-wuu-component="session-tab-main"],
      .manga-shell [data-wuu-component="conversation-pane"] > .titlebar [data-wuu-component="session-tab-close"] {
        color: inherit;
        background: transparent;
        border: 0 !important;
        border-radius: 0 !important;
        box-shadow: none;
        transform: none !important;
      }
      .manga-shell [data-wuu-component="conversation-pane"] > .titlebar [data-wuu-component="session-tab-close"] {
        margin-right: 0;
        border-left: 1px solid currentColor !important;
      }
      .manga-shell [data-wuu-component="conversation-pane"] > .titlebar [data-wuu-component="session-tab-close"]:hover {
        background: rgb(255 63 127 / .3);
      }
      .manga-shell [data-wuu-component="conversation-pane"] > .titlebar [data-wuu-component="session-tab"][data-wuu-active="true"]::after {
        right: 8px;
        left: 8px;
        height: 4px;
        border-radius: 0;
        background: var(--manga-pink);
      }
    `,
  });

  api.registerSurface("app.shell", {
    id: "manga-shell",
    mode: "wrap",
    render(_context, fallback) {
      return React.createElement(
        "main",
        { className: "manga-shell" },
        fallback,
      );
    },
  });

}
