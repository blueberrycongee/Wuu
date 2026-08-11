# Desktop UI plugins

A Desktop plugin is a trusted ESM module loaded into the Wuu Renderer. It can use the
host React runtime and UI Kit to register components, pages, styles, and actions at
stable UI boundaries without forking Wuu.

This page establishes the UI composition model. See the
[plugin authoring reference](plugin-authoring.md) for exact types and manifests, and
the [Desktop plugin recipes](plugin-recipes.md) for copyable implementations.

## How plugins extend Wuu UI

Wuu retains final ownership of the App Shell, window safe areas, navigation structure,
scrolling, tabs, plugin management, and recovery paths. It exposes semantic boundaries
at several levels:

```text
Wuu Desktop
├── Sidebar
│   ├── sidebar.primary                 Slot
│   └── sidebar.footer                  Slot
├── Workspace
│   ├── workspace.header                Slot
│   └── View regions                    View
└── Conversation
    ├── conversation.header             Slot / Presenter
    ├── conversation.timeline           Surface
    │   └── conversation.message        Surface / Presenter
    │       ├── conversation.message.before   Slot
    │       └── conversation.message.after    Slot
    └── conversation.composer           Presenter
        ├── composer.above               Slot
        └── composer.toolbar             Slot
```

Plugins register contributions at runtime. Wuu composes them with native UI and
removes every contribution from a generation together on disable, upgrade, or unload.

## Choose by goal

| Goal | Use | Example |
| --- | --- | --- |
| Add a small control at a fixed location | Slot | Composer button, message badge, header status |
| Change one product concept | Presenter | Composer, conversation item, tool activity card |
| Wrap or replace a larger semantic region | Surface | Conversation timeline or message boundary |
| Add a full page or complex tool | View | Dashboard, history, editor, visualization |
| Show a short environment summary | Inspector Section | Run status, plan, repository summary |
| Show temporary interaction in a conversation | Conversation Card | Status and actions for one command |
| Change appearance only | Theme tokens / CSS snippets | Colors, type, density, semantic decoration |

Prefer the smallest boundary that fits. Do not replace the Composer to add one button,
and do not put a long list in a toolbar.

## Slot: add content at a fixed location

Current production Slots:

| Slot | Location |
| --- | --- |
| `sidebar.primary` | Main sidebar content |
| `sidebar.footer` | Sidebar footer |
| `workspace.header` | Workspace header |
| `conversation.header` | Conversation header |
| `conversation.message.before` | Before each message |
| `conversation.message.after` | After each message |
| `composer.above` | Above the Composer |
| `composer.toolbar` | Composer toolbar |

Slots are additive. Multiple plugins can compose in `order` without taking ownership
of the native boundary.

## Presenter: change a product concept

A Presenter receives a versioned snapshot, the actions available at that boundary,
the native fallback, and a target plus optional match key. `mode: "wrap"` preserves and
wraps the current result; `mode: "replace"` owns the entire boundary.

Built-in targets include:

- `conversation.item`
- `conversation.process`
- `conversation.tool-activity`
- `conversation.composer`
- `header.conversation`
- `header.workspace`
- `navigation.primary`
- `app.status`
- `content.preview`
- `settings`

Actions are not global permissions. A plugin may invoke only actions listed in the
current `host.actions`. The Composer boundary exposes draft, submission, stop, or
attachment actions as its current state allows; Wuu still validates read-only and
disabled states.

## Surface: wrap a larger semantic boundary

Current production Surfaces:

| Surface | Scope |
| --- | --- |
| `conversation.timeline` | One turn's timeline and orchestration group |
| `conversation.message` | One sanitized message boundary |

Use a Surface for structural wrappers or strong visual changes, not for a single
button. Every Surface retains a native fallback, and render failure falls back only at
the affected boundary.

## View: add a full page

Register a component with `registerViewType`, then request an initial region with
`registerViewPlacement`:

- `navigation`
- `primary`
- `auxiliary`
- `inspector`
- `settings`
- `overlay`

Wuu owns tabs, closing, scrolling, persistence, and region layout. Plugins do not
receive arbitrary parent nodes, split trees, or panel dimensions. User-facing
navigation, workspace-tool, and settings entries are declared in the manifest and
point to Views registered by the same plugin.

## Standard Web APIs are available

Desktop plugins are trusted Renderer code and may use standard APIs such as
`selectionchange`, `window.getSelection()`, `ResizeObserver`, and
`IntersectionObserver`.

Standard Web APIs do not make private host structure stable. Use public semantic
anchors when checking location or applying CSS:

```text
data-wuu-component="turn"
data-wuu-component="message"
data-wuu-component="composer"
data-wuu-component="composer-input"
data-wuu-component="composer-send"
data-wuu-slot="..."
data-wuu-surface="..."
```

Do not depend on private class names, React fibers, component nesting, or simulated
keyboard events. Production tests protect semantic anchors; private DOM is suitable
only for local experiments.

## Host-owned boundaries remain

Plugins cannot replace or hide plugin management, safe mode, crash recovery,
permission limits, or native window lifecycle. Desktop code and unrestricted CSS are
high-trust capabilities. Install only trusted sources; every package change creates a
new fingerprint and requires approval.

Complete the [Desktop plugin quickstart](desktop-plugin-quickstart.md), then choose a
real implementation from the [Desktop plugin recipes](plugin-recipes.md).
