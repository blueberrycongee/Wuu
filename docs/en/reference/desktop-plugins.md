# Customize the desktop UI with plugins

Trusted desktop-code plugins can register global styles and replace or wrap stable UI surfaces.
This supports substantial layout changes without relying on DOM monkey patches or private React
state.

Wuu always retains plugin management, approval, safe mode, crash recovery, and the native window
lifecycle. Every surface has a built-in fallback; a plugin render error records a diagnostic and
returns to that fallback.

## Package shape

```text
my-layout/
├── plugin.json
└── dist/
    └── desktop.js
```

```json
{
  "schemaVersion": 1,
  "id": "my-layout",
  "name": "My Layout",
  "version": "1.0.0",
  "desktop": { "entry": "dist/desktop.js" }
}
```

The entry is currently a self-contained ESM bundle with no relative imports and a 10 MiB limit.
Use the host React instance from `api.react` rather than bundling another copy.

```js
export async function activate(api) {
  const React = api.react;

  api.registerStyle({
    id: "layout",
    css: ".my-frame { border: 1px solid var(--wuu-hairline); }",
  });

  api.registerSurface("app.sidebar", {
    id: "sidebar-frame",
    mode: "wrap",
    render(_context, fallback) {
      return React.createElement("section", { className: "my-frame" }, fallback);
    },
  });
}
```

`mode: "replace"` replaces the fallback. `mode: "wrap"` receives the fallback and composes around
it. Registrations belong to one plugin generation and are removed together when the plugin is
disabled, removed, or upgraded.

## Available surfaces

| Surface | Host UI |
| --- | --- |
| `app.shell` | Entire React application shell |
| `app.sidebar` | Left navigation |
| `conversation.composer` | Main prompt composer |
| `conversation.timeline` | One conversation turn/orchestration group |
| `view.settings` | Settings shell |
| `view.catalog` | Skills, plugins, and Automations catalogs |

Contexts expose versioned, limited state and host actions. For example, the shell exposes navigation
actions, while the composer exposes its prompt, running/read-only state, `setPrompt`, `send`, and
`interrupt`. Do not depend on private Wuu class names as a compatibility contract.

## Declarative themes

Themes do not require desktop code. Add them under `contributes.themes` in `plugin.json`. Themes from
enabled plugins appear under **Settings → Appearance** and are removed cleanly when disabled or when
the user returns to a built-in theme.

Only allowlisted semantic tokens are accepted: `--wuu-paper`, `--wuu-ink`, `--wuu-ink-soft`,
`--wuu-hairline`, `--wuu-surface-muted`, `--wuu-accent`, `--wuu-accent-press`, and the published
`--hljs-*` syntax tokens.

See the installable [`examples/plugins/deep-ui`](../../../examples/plugins/deep-ui/) package for a
theme and wrappers covering all current surfaces.

## Loading and trust

The renderer never receives an absolute plugin path. Before every load, app-server reloads the
manifest, recalculates the whole-package fingerprint, and confirms that the exact version is still
approved and enabled for the workspace. Electron verifies the source digest again and imports it
through a content-addressed `wuu-plugin:` URL. The Content Security Policy does not enable
`unsafe-eval`.

Desktop code runs in the renderer and may register unrestricted CSS, so install it only from a
trusted source. Any package change produces a new fingerprint and requires review and approval.
