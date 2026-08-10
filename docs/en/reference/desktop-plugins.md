# Customize the desktop UI with plugins

> The complete plugin authoring guide (manifest, Agent plugins, desktop contributions,
> and the local development loop) lives in
> [Writing plugins](../customize/plugin-authoring.md). For installing and managing
> plugins, see [Plugins](../customize/plugins.md). This page covers the
> desktop-code surface.

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

  api.registerSurface("conversation.timeline", {
    id: "timeline-frame",
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
| `conversation.timeline` | One conversation turn/orchestration group |
| `conversation.message` | One sanitized conversation message boundary |

App Shell, navigation, primary Session UI, Composer, auxiliary container, Settings, launch, and
plugin management are protected Host roots. A plugin can add a View, Slot, or Presenter around
their public semantic contracts, but cannot replace or hide those recovery paths.

Additive contributions use `registerSlot`. Current production slots are `sidebar.primary`,
`sidebar.footer`, `workspace.header`, `conversation.header`, `conversation.message.before`,
`conversation.message.after`, `composer.above`, and `composer.toolbar`. Slot
contexts contain only frozen summary fields; they do not contain private host records. Slots compose
with native UI and semantic presenters rather than replacing their ownership boundary.

## View placements, not arbitrary layouts

A plugin can register a View and request its initial placement in one stable host-owned semantic
region: `navigation`, `primary`, `auxiliary`, `inspector`, `settings`, or `overlay`.

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
UI. `registerViewPlacement` is the only placement API.

## Inspector summaries

Use `registerInspectorSection` for a short, scan-friendly summary in the host environment panel:

```js
api.registerInspectorSection({
  id: "run-summary",
  title: "Run summary",
  priority: 20,
  render({ snapshot, host }) {
    return api.react.createElement(
      api.ui.Button,
      { onClick: () => host.openView("my-plugin.details", { region: "auxiliary" }) },
      snapshot.session.status === "running" ? "Running" : "Idle",
    );
  },
});
```

The versioned snapshot contains only public Session, Turn, Workspace, Git, and Plan summaries.
Actions are limited to opening a registered View or executing a registered Command. The host caps
each Section height, owns overflow, and isolates each contribution with its own error fallback.
Long lists, editors, and complex interaction belong in a `primary` or `auxiliary` View.

## Host-owned discovery entries

A registered View does not need to draw its own navigation or tabs. Declare where users should
discover it in `plugin.json`:

```json
{
  "contributes": {
    "navigation": [
      { "id": "dashboard", "view": "product.dashboard", "title": "Dashboard", "icon": "gauge" }
    ],
    "workspaceTools": [
      { "id": "inspector", "view": "product.inspector", "title": "Inspector" }
    ],
    "settingsPages": [
      { "id": "advanced", "view": "product.advanced", "title": "Advanced" }
    ]
  }
}
```

`navigation` appears in the scrolling plugin group in the left sidebar, `workspaceTools` appears
in the right-panel tool picker and opens as a native workspace tab, and `settingsPages` appears in
the Plugins group in Settings. Wuu owns selection, close behavior, persistence, and overflow. Each
entry must reference a View registered by the same plugin. Standard `contributes.settings` entries
automatically receive a host-rendered Settings page; use a custom settings View only for content
that cannot be expressed by the standard schema.

Declare a top-level `icon` to brand the plugin catalog and detail view. It may be a public semantic
name, `{ "path": "assets/icon.svg" }`, or a themed
`{ "light": "assets/icon-light.svg", "dark": "assets/icon-dark.svg" }` pair. Entry-level `icon`
uses the same contract but is independent from the package artwork. Host navigation does not
inherit the brand icon when an entry omits `icon`. Import `PUBLIC_ICON_NAMES`,
`PublicIconName`, and `PluginManifestIcon` from `@wuu/plugin-sdk` for authoring support.

Package artwork accepts SVG, PNG, and WebP up to 256 KiB per file. Paths must remain inside the
package and cannot be symbolic links. SVG scripts, event attributes, embedded documents, and
external references are rejected. Wuu owns sizing, active state, accessibility, theme switching,
and load-failure fallback; desktop modules cannot inject icon components into host chrome.

## Host-owned UI Kit

Custom Views receive `api.ui`, a deliberately small set of host-owned React components:
`Page`, `Panel`, `Card`, `Section`, `Stack`, `Row`, `Button`, `ToolbarToggle`, `TextInput`, `TextArea`, `Checkbox`,
`EmptyState`, `LoadingState`, `ErrorState`, and `LiveDuration`.
They preserve Wuu's spacing and interaction behavior while inheriting the active appearance theme.
`Page` accepts `density: "comfortable" | "compact"`; state components own ARIA status, loading
motion, error treatment, responsive spacing, and overflow behavior. Use `ToolbarToggle` for binary
Composer-toolbar controls so the host owns `aria-pressed`, hit targets, focus, and active styling.
`LiveDuration` renders accumulated milliseconds and, while active, updates from an optional running
start time without requiring the plugin to own an interval.

```js
const { Button, Card, Page, Section, Stack } = api.ui;

function Dashboard() {
  return api.react.createElement(Page, null,
    api.react.createElement(Section, { title: "Dashboard" },
      api.react.createElement(Card, null,
        api.react.createElement(Stack, null,
          "Ready",
          api.react.createElement(Button, { variant: "primary" }, "Run"),
        ),
      ),
    ),
  );
}
```

Use these components for ordinary product UI so appearance plugins also affect Views added by
other plugins. They are not a page DSL: complex Views may still render arbitrary React, and
specialized canvases, terminals, and previews remain explicit theme boundaries. The UI Kit owns
common layout rhythm; plugins should not override its internal class names.

## Commands and conversation cards

Declare a `runtime_action` under `contributes.commands` and register a desktop command with the same
ID to expose an approved plugin action in the Composer slash menu. The entry is available only while
the plugin is enabled and its desktop generation has registered the command; the manifest alone never
executes plugin code.

Use `api.showConversationCard` when that action, a host event, or plugin-owned asynchronous work needs
to display temporary interaction at the bottom of a conversation. Omitting `threadId` targets the
current conversation. The returned handle can update the card state or dismiss it. Cards are not
written to conversation history or model context, and the host removes them when the generation is
disposed or the app restarts.

```js
api.registerCommand({
  id: "show-status",
  title: "Show status",
  execute(input) {
    return api.showConversationCard({
      threadId: input?.threadId,
      title: "Plugin status",
      state: { status: "ready" },
      render({ state, dismiss }) {
        return api.react.createElement(
          api.ui.Stack,
          { gap: "small" },
          api.react.createElement("span", null, state.status),
          api.react.createElement(api.ui.Button, { onClick: dismiss }, "Close"),
        );
      },
    });
  },
});
```

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

Shared neutral UI uses coarse semantic tokens so an appearance plugin does not need private DOM
selectors: `--wuu-control-secondary-background` styles secondary actions,
`--wuu-control-field-background` styles text fields and selects,
`--wuu-control-icon-background` styles compact icon tiles,
`--wuu-badge-neutral-background` styles neutral status and permission badges, and
`--wuu-inline-code-background` styles Markdown inline code. Their text and font continue to inherit
the corresponding public color and typography tokens. Compact controls keep host-owned border widths
so a strong panel border cannot change their box model or break wrapping.

Common host dialogs, menus, popovers, tooltips, notices, and floating navigation now render through
the protected Layer Host and expose stable `data-wuu-component`, `data-wuu-layer`, and
`data-wuu-state` attributes. Drag previews, PDF ShadowRoot content, and plugin View pane mounts remain
specialized rendering boundaries rather than appearance layers. Trusted code plugins that need
supplemental CSS should target only the published attributes and tokens, not private class names or
DOM structure.

Native workspace tabs expose `workspace-tool-tab` and `workspace-tool-tab-close` anchors. A tab uses
`data-wuu-active="true" | "false"` for selection and `data-wuu-state="closing" | "dragging"` for
transient lifecycle state; selection, closing, dragging, and overflow remain host-owned.

Message controls expose one host-owned `message-actions` group anchor. Action bars distinguish
`data-wuu-placement="persistent" | "overlay"`.
Themes may tune their rhythm with `--wuu-message-actions-block-gap`,
`--wuu-message-actions-overlay-gap`, `--wuu-message-actions-control-gap`,
`--wuu-message-actions-inline-offset`, `--wuu-message-action-size`, and
`--wuu-message-action-radius`. Wuu retains ownership of keyboard order, hit targets, responsive
overflow, and the two placement semantics. Style its direct buttons as one control family instead
of targeting individual action identities. The user-query surface is separately exposed as
`message-bubble` with variant `user`; its visual properties use the matching
`--wuu-message-user-*` tokens.

The UI Kit exposes coarse anchors for `plugin-ui-page`, `plugin-ui-panel`, `plugin-ui-card`,
`plugin-ui-section`, `plugin-ui-stack`, `plugin-ui-row`, `plugin-ui-button`, `plugin-ui-field`,
`plugin-ui-input`, `plugin-ui-empty-state`, `plugin-ui-loading-state`, `plugin-ui-error-state`, and
`plugin-ui-live-duration`.
Appearance plugins should prefer public tokens and use these boundaries only when a structural
treatment is necessary.

Settings exposes `settings-shell`, `settings-sidebar`, `settings-content`, and `settings-page` as
coarse layout boundaries. Themes can give the navigation rail and content canvas different material
treatments without targeting private Settings classes; the shared sidebar divider inherits the
public `--wuu-border-subtle` token.

Sidebar navigation hover shares one public treatment across the main rail, plugin entries, project
and thread rows, the settings rail, and the settings back button:
`--wuu-nav-item-hover-background` paints hovered/expanded rows and `--wuu-nav-item-hover-ring`
tints their inset ring. Both fall back to the host glass material, so themes opt in without
touching private row classes.

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
