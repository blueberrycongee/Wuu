# Customize the desktop UI with plugins

Trusted desktop-code plugins can register global styles and replace or wrap stable UI surfaces.
This supports coherent visual systems and bounded structural changes without relying on DOM monkey
patches or private React state.

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
| `app.main` | Main content region |
| `app.auxiliary` | Auxiliary workspace region |
| `app.status` | Additive application status region |
| `view.launch` | Launch and runtime-loading view |
| `view.conversation` | Active conversation view |
| `view.workspace` | Workspace side-panel view |
| `conversation.composer` | Main prompt composer |
| `conversation.timeline` | One conversation turn/orchestration group |
| `conversation.message` | One sanitized conversation message boundary |
| `view.settings` | Settings shell |
| `view.catalog` | Skills, plugins, and Automations catalogs |

Contexts expose versioned, limited state and host actions. For example, the shell exposes navigation
actions, while the composer exposes its prompt, running/read-only state, `setPrompt`, `send`, and
`interrupt`. Do not depend on private Wuu class names as a compatibility contract.

Additive contributions use `registerSlot`. Current production slots are `sidebar.primary`,
`sidebar.footer`, `workspace.header`, `conversation.header`, `conversation.message.before`,
`conversation.message.after`, `composer.above`, `composer.toolbar`, and `settings.plugin`. Slot
contexts contain only frozen summary fields; they do not contain private host records. Slots compose
with native UI and semantic presenters rather than replacing their ownership boundary.

## View placements, not arbitrary layouts

A plugin can register a View and request its initial placement in one stable host-owned region:
`main`, `sidebar`, or `auxiliary`.

```js
api.registerViewType({
  id: "my-plugin.dashboard",
  title: "Dashboard",
  persistence: "durable",
  render: Dashboard,
});

api.registerViewPlacement({
  id: "dashboard-default",
  view: "my-plugin.dashboard",
  region: "auxiliary",
  priority: 10,
});
```

`priority` only resolves the initial active View when a region has no user choice. User activation
and dismissal win and are persisted. Placement does not expose the shell DOM, arbitrary parent
nodes, split-tree construction, panel dimensions, protected chrome, plugin management, or recovery
UI. The old `registerLayoutContribution` method remains as a compatibility adapter; its
`parentId`, `size`, and `minSize` fields were never implemented as layout-tree controls and are not
used. New plugins should use `registerViewPlacement`.

## Semantic presenters

Use `registerPresenter` when a plugin needs to replace a product concept rather than a broad layout
region. Wuu supplies a frozen, versioned snapshot, the native fallback, and a host object containing
only the actions available at that render boundary.

```js
export async function activate(api) {
  api.registerPresenter({
    id: "assistant-card",
    target: "conversation.item",
    key: "assistant-message",
    mode: "wrap",
    render({ snapshot, host, fallback }) {
      const copy = host.actions.includes("conversation.item.copy")
        ? api.react.createElement("button", {
            onClick: () => host.invoke("conversation.item.copy"),
          }, "Copy")
        : null;
      return api.react.createElement("article", null, fallback, copy);
    },
  });
}
```

Built-in targets are:

| Presenter target | Stable match key |
| --- | --- |
| `conversation.item` | Item kind such as `assistant-message`, `reasoning`, or `attachment` |
| `conversation.process` | Complete process shape: `reasoning`, `tool-group`, or `mixed` |
| `conversation.tool-activity` | Stable Tool capability, before any execution-name rewrite |
| `conversation.composer` | None |
| `header.conversation`, `header.workspace` | None |
| `navigation.primary` | None |
| `app.status` | None |
| `content.preview` | Exact MIME type |
| `settings` | None |

The public SDK defines each V1 snapshot and its dotted Action IDs. An Action is usable only when it
appears in `host.actions`; unsupported or invalid input is rejected by Wuu. Plugins never receive
private thread items, protocol messages, the host React tree, or arbitrary callbacks.

`mode: "replace"` owns the complete semantic boundary. `mode: "wrap"` composes around the current
result. Presenters are generation-scoped: candidate activation is atomic, failed activation keeps the
previous generation, render failure falls back locally, and disable, upgrade, or unload disposes every
registration. `registerToolActivityPresenter` remains available as the compatible Tool-specific
adapter.

## Declarative themes

Themes do not require desktop code. Add them under `contributes.themes` in `plugin.json`. Themes from
enabled plugins appear under **Settings → Appearance** and are removed cleanly when disabled or when
the user returns to a built-in theme.

[`config/desktop-theme-contract.json`](../../../config/desktop-theme-contract.json) is the single
registry for public tokens and generates the manifest, SDK, and Desktop validators. Stable families
cover semantic colors, typography, spacing, density, radius, borders, elevation, motion, content
width, and `--wuu-syntax-*` syntax colors. Early names such as `--wuu-paper`, `--wuu-ink`,
`--wuu-accent`, and `--hljs-*` remain compatible and map to the current semantic contract. New themes
should prefer the current `--wuu-color-*` and `--wuu-font-*` names.

Common host dialogs, menus, popovers, tooltips, notices, and floating navigation now render through
the protected Layer Host and expose stable `data-wuu-component`, `data-wuu-layer`, and
`data-wuu-state` attributes. Drag previews, PDF ShadowRoot content, and plugin View pane mounts remain
specialized rendering boundaries rather than appearance layers. Trusted code plugins that need
supplemental CSS should target only the published attributes and tokens, not private class names or
DOM structure.

See the installable [`examples/plugins/deep-ui`](../../../examples/plugins/deep-ui/) package for a
theme and wrappers covering all current surfaces.

The [`examples/plugins/developer-loop`](../../../examples/plugins/developer-loop/) package is the
public-SDK acceptance example for multi-surface presenters, host actions, generation replacement,
failure recovery, disposal, and unload.

## Loading and trust

The renderer never receives an absolute plugin path. Before every load, app-server reloads the
manifest, recalculates the whole-package fingerprint, and confirms that the exact version is still
approved and enabled for the workspace. Electron verifies the source digest again and imports it
through a content-addressed `wuu-plugin:` URL. The Content Security Policy does not enable
`unsafe-eval`.

Desktop code runs in the renderer and may register unrestricted CSS, so install it only from a
trusted source. Any package change produces a new fingerprint and requires review and approval.
