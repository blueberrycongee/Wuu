/**
 * 苔径 Herbarium — desktop entry.
 *
 * The manifest's declarative themes carry every color, type, radius,
 * border, elevation and motion decision. This module adds the structural
 * decoration layer (sidebar spine, folder tabs, letterpress composer,
 * double-rule panel headers, bookplate dialogs) as one CSS snippet.
 *
 * Snippet rules are scoped to `:root[data-herbarium]`, an attribute this
 * module stamps only while one of its own themes is the applied theme,
 * so enabling the plugin never touches the built-in light/dark themes.
 * The applied theme is detected by fingerprinting the public tokens the
 * host stamps inline on <html> — no host-private storage or class names.
 */

const THEME_FINGERPRINTS = {
  paper: {
    "--wuu-color-canvas": "#f3eee0",
    "--wuu-color-accent": "#3f6b4f",
  },
  night: {
    "--wuu-color-canvas": "#1b1f1a",
    "--wuu-color-accent": "#92b795",
  },
};

const SCOPE_ATTRIBUTE = "data-herbarium";

function detectHerbariumTheme(root) {
  for (const [id, tokens] of Object.entries(THEME_FINGERPRINTS)) {
    const matches = Object.entries(tokens).every(
      ([name, value]) => root.style.getPropertyValue(name).trim().toLowerCase() === value,
    );
    if (matches) {
      return id;
    }
  }
  return "";
}

export async function activate(api) {
  const root = document.documentElement;

  const syncScopeAttribute = () => {
    const active = detectHerbariumTheme(root);
    if (active) {
      if (root.getAttribute(SCOPE_ATTRIBUTE) !== active) {
        root.setAttribute(SCOPE_ATTRIBUTE, active);
      }
    } else {
      root.removeAttribute(SCOPE_ATTRIBUTE);
    }
  };

  // The host stamps theme tokens as inline custom properties on <html>,
  // so `style` mutations cover theme select/deselect. Watching our own
  // attribute as well lets the scope recover if an older generation's
  // cleanup removes it after this generation has already stamped it.
  const observer = new MutationObserver(syncScopeAttribute);
  observer.observe(root, {
    attributes: true,
    attributeFilter: ["style", SCOPE_ATTRIBUTE],
  });
  syncScopeAttribute();

  api.registerCleanup(() => {
    observer.disconnect();
    root.removeAttribute(SCOPE_ATTRIBUTE);
  });

  api.registerCSSSnippet({
    id: "herbarium-print-details",
    priority: 20,
    css: `
/* 苔径 Herbarium — structural decoration layer.
   Every color below resolves through public --wuu-* tokens, so the same
   rules serve both the Paper and the Night edition. */

:root[data-herbarium] ::selection {
  background: color-mix(in srgb, var(--wuu-color-accent) 24%, transparent);
}

/* --- Left rail: book gutter — hairline edge with a moss spine -------- */
:root[data-herbarium] [data-wuu-component="sidebar"] {
  background: var(--wuu-color-surface-muted);
  border-right: 1px solid var(--wuu-color-border-subtle);
  box-shadow: inset -2px 0 0 var(--wuu-color-accent);
}

:root[data-herbarium] [data-wuu-component="plugin-navigation-item"] {
  border-radius: var(--wuu-radius-control);
  letter-spacing: 0.03em;
}

:root[data-herbarium] [data-wuu-component="plugin-navigation-item"]:hover {
  background: color-mix(in srgb, var(--wuu-color-accent) 10%, transparent);
}

/* --- Navigation tabs: one folder-card family across split panes -------- */
:root[data-herbarium] [data-wuu-component="session-tab"],
:root[data-herbarium] [data-wuu-component="workspace-panel-header"] [role="tab"] {
  background: var(--wuu-color-surface);
  border: 1px solid var(--wuu-color-border-subtle);
  border-radius: var(--wuu-radius-control);
  letter-spacing: 0.02em;
}

:root[data-herbarium] [data-wuu-component="session-tab"][data-wuu-active="true"],
:root[data-herbarium] [data-wuu-component="workspace-panel-header"] [role="tab"][aria-selected="true"] {
  background: var(--wuu-color-surface-elevated);
  border-color: var(--wuu-color-border-strong);
  color: var(--wuu-color-text);
}

:root[data-herbarium] [data-wuu-component="session-tab-close"] {
  border-radius: 50%;
}

/* --- Composer: a letterpress writing card with a wax-seal send -------- */
:root[data-herbarium] [data-wuu-component="composer-frame"] {
  background: var(--wuu-color-surface-elevated);
  border: 1px solid var(--wuu-color-border-strong);
  border-radius: var(--wuu-radius-panel);
  box-shadow:
    0 1px 0 var(--wuu-color-border-subtle),
    inset 0 1px 2px color-mix(in srgb, var(--wuu-color-text) 6%, transparent);
}

:root[data-herbarium] [data-wuu-component="composer-input"] {
  caret-color: var(--wuu-color-accent);
}

:root[data-herbarium] [data-wuu-component="composer-input"]::placeholder {
  font-style: italic;
}

:root[data-herbarium] [data-wuu-component="composer-send"] {
  border-radius: 50%;
  box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--wuu-color-on-accent) 35%, transparent);
}

:root[data-herbarium] [data-wuu-component="composer-send"][data-wuu-state="stop"] {
  background: var(--wuu-color-danger);
  color: var(--wuu-color-on-accent);
}

/* --- Message actions: one control family, moss on hover --------------- */
:root[data-herbarium] [data-wuu-component="message-actions"] > button:hover {
  background: color-mix(in srgb, var(--wuu-color-accent) 12%, transparent);
  color: var(--wuu-color-accent);
}

/* Information and environment cards share this public host boundary. */
:root[data-herbarium] [data-wuu-component="environment-panel"] {
  background: var(--wuu-color-surface);
  border-color: var(--wuu-color-border-strong);
  color: var(--wuu-color-text);
}

/* --- Right panel: printed masthead with the classic double rule ------- */
:root[data-herbarium] [data-wuu-component="workspace-panel"] {
  background: var(--wuu-color-canvas);
  border-left: 1px solid var(--wuu-color-border-subtle);
}

:root[data-herbarium] [data-wuu-component="workspace-panel-header"] {
  border-bottom: 1px solid var(--wuu-color-border-strong);
  box-shadow: 0 1px 0 var(--wuu-color-border-subtle);
}

/* Panel button families: one bordered, paper-raised control style. */
:root[data-herbarium] [data-wuu-component="workspace-browser-toolbar"] button,
:root[data-herbarium] [data-wuu-component="workspace-terminal-navigation"] button,
:root[data-herbarium] [data-wuu-component="workspace-review-navigation"] button {
  background: var(--wuu-color-surface-elevated);
  border: 1px solid var(--wuu-color-border-subtle);
  border-radius: var(--wuu-radius-control);
}

:root[data-herbarium] [data-wuu-component="workspace-browser-toolbar"] button:hover,
:root[data-herbarium] [data-wuu-component="workspace-terminal-navigation"] button:hover,
:root[data-herbarium] [data-wuu-component="workspace-review-navigation"] button:hover {
  border-color: var(--wuu-color-accent);
  color: var(--wuu-color-accent);
}

/* Public bridges into the file tree's shadow DOM and the xterm screen. */
:root[data-herbarium] {
  --wuu-workspace-file-tree-color: var(--wuu-color-text);
  --wuu-workspace-file-tree-selected-background: color-mix(in srgb, var(--wuu-color-accent) 16%, transparent);
  --wuu-workspace-file-tree-font-family: var(--wuu-font-family-ui);
  --wuu-workspace-file-tree-search-background: var(--wuu-control-field-background);
  --wuu-workspace-file-tree-search-border: 1px solid var(--wuu-color-border-subtle);
  --wuu-workspace-file-tree-search-radius: var(--wuu-radius-control);
}

/* The terminal is the one place that goes dark even in the Paper edition:
   a deep pine screen, like ink soaking into the page. */
:root[data-herbarium="paper"] {
  --wuu-workspace-terminal-background: #263129;
  --wuu-workspace-terminal-foreground: #e8e4d0;
  --wuu-workspace-terminal-cursor: #7fae87;
  --wuu-workspace-terminal-selection: rgba(146, 183, 149, 0.32);
  --wuu-workspace-terminal-font-family: var(--wuu-font-family-mono);
}

:root[data-herbarium="night"] {
  --wuu-workspace-terminal-background: #141815;
  --wuu-workspace-terminal-foreground: #e6e1cd;
  --wuu-workspace-terminal-cursor: #a4caa7;
  --wuu-workspace-terminal-selection: rgba(146, 183, 149, 0.28);
  --wuu-workspace-terminal-font-family: var(--wuu-font-family-mono);
}

/* --- Settings: same spine and bookmark-tab navigation. The host owns the
   flat section rhythm; themes should not turn structural row groups back into
   framed cards because that changes layout rather than appearance. -------- */
:root[data-herbarium] [data-wuu-component="settings-shell"] {
  background: var(--wuu-color-canvas);
}

:root[data-herbarium] [data-wuu-component="settings-sidebar"] {
  background: var(--wuu-color-surface-muted);
  border-right: 1px solid var(--wuu-color-border-subtle);
  box-shadow: inset -2px 0 0 var(--wuu-color-accent);
}

:root[data-herbarium] [data-wuu-component="settings-navigation-item"] {
  border-radius: var(--wuu-radius-control);
}

:root[data-herbarium] [data-wuu-component="settings-navigation-item"]:hover {
  background: color-mix(in srgb, var(--wuu-color-accent) 8%, transparent);
}

:root[data-herbarium] [data-wuu-component="settings-navigation-item"][aria-current="page"] {
  background: color-mix(in srgb, var(--wuu-color-accent) 12%, transparent);
  box-shadow: inset 2px 0 0 var(--wuu-color-accent);
  color: var(--wuu-color-text);
}

/* --- Layers: bookplate dialogs, framed menus, annotated tooltips ------ */
:root[data-herbarium] [data-wuu-layer="dialog"],
:root[data-herbarium] [data-wuu-layer="modal"] {
  border: 1px solid var(--wuu-color-border-strong);
  border-radius: var(--wuu-radius-overlay);
  outline: 1px solid var(--wuu-color-border-subtle);
  outline-offset: -5px;
  box-shadow: var(--wuu-elevation-overlay);
}

:root[data-herbarium] [data-wuu-layer="menu"],
:root[data-herbarium] [data-wuu-layer="popover"] {
  border: 1px solid var(--wuu-color-border-strong);
  border-radius: var(--wuu-radius-overlay);
  box-shadow: var(--wuu-elevation-overlay);
}

:root[data-herbarium] [data-wuu-layer="tooltip"] {
  font-style: italic;
  letter-spacing: 0.01em;
}

:root[data-herbarium] [data-wuu-layer="notice"] {
  border-left: 3px solid var(--wuu-color-accent);
}

/* --- Empty states: a soft page glow ------------------------------------ */
:root[data-herbarium] [data-wuu-component="launch-view"],
:root[data-herbarium] [data-wuu-component="empty-session"] {
  background: radial-gradient(60% 50% at 50% 38%, var(--wuu-color-surface-elevated) 0%, transparent 100%);
}

/* --- Plugin UI Kit: same printed-card treatment ------------------------ */
:root[data-herbarium] [data-wuu-component="plugin-ui-panel"],
:root[data-herbarium] [data-wuu-component="plugin-ui-card"] {
  border: 1px solid var(--wuu-color-border-subtle);
  border-radius: var(--wuu-radius-panel);
  box-shadow: 0 1px 0 var(--wuu-color-border-subtle);
}
    `,
  });
}
