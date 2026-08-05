# Deep UI Customization

## Product Goal

Let a user reshape almost every visible part of Wuu without forking the app.
Plugins may change the visual language, add UI beside built-in controls, replace
major views, or wrap built-in views with a different layout. The result should
feel like a different product while continuing to use Wuu's sessions, tools,
settings, permissions, and native integrations.

Deep customization is a supported composition model, not permission to depend
on Wuu's private DOM structure. Plugins use stable themes, zones, surfaces,
contexts, and actions. A Wuu update may change the built-in implementation
without breaking those public contracts.

## Host-Owned Safety Kernel

The following remains host-owned and cannot be replaced by a plugin:

- plugin approval, enable/disable, update, and removal;
- safe mode and crash recovery;
- permission and trust prompts;
- native window lifecycle and emergency restart;
- the error boundary that isolates a plugin contribution;
- accessibility escape routes, including keyboard access to settings and safe
  mode.

These controls may be opened from a customized UI through public actions, but a
plugin cannot remove the underlying recovery path. Production still starts
without third-party desktop code when the previous boot did not complete.

## Composition Layers

### Themes

Themes are declarative and safe by default. They may override the complete
public semantic token set: color, type, spacing, radius, elevation, motion,
density, and syntax highlighting. Theme selection, preview, persistence,
fallback, and unload are host-owned.

Themes do not contain selectors. A trusted desktop code plugin may additionally
register attributed CSS when tokens are not enough. Registered CSS unloads with
the plugin generation. Depending on private class names is allowed for local
experiments but is not a compatibility guarantee.

### Zones

A zone is an ordered list of contributions rendered around a host-owned or
plugin-owned view. Zones are for additions such as buttons, status, navigation
items, message decorations, panels, and settings sections.

Each public zone defines:

- stable ID and scope: app, workspace, thread, turn, or message;
- typed public context and actions;
- ordering and optional grouping;
- empty behavior and error isolation;
- whether contributions are inline, stacked, overlaid, or tabbed.

The initial public zones cover the app header, sidebar start/end, workspace
header, conversation header/empty state, message before/after/actions, composer
before/toolbar/after, activity, details, status, catalog, and settings.

### Surfaces

A surface is a replaceable unit with a host fallback. A plugin contribution can
replace the fallback or wrap it. Only one replacement wins; deterministic
priority and user preference choose it. Wrappers compose in deterministic
order.

The surface renderer receives:

- a stable context snapshot;
- supported host actions;
- the fallback React node for wrappers;
- public UI primitives and React from the host;
- namespaced settings, storage, locale, and app-server RPC.

The initial surface hierarchy is:

```text
app.shell
├── app.sidebar
├── app.main
│   ├── view.launch
│   ├── view.conversation
│   │   ├── conversation.timeline
│   │   ├── conversation.message
│   │   └── conversation.composer
│   ├── view.workspace
│   ├── view.catalog
│   └── view.settings
├── app.auxiliary
└── app.status
```

Replacing `app.shell` is the deepest supported customization. The customized
shell receives the built-in shell as its fallback and cannot replace the safety
kernel outside the React application root.

## Public State and Actions

Plugins do not receive mutable internal App state or component instances. The
host publishes small immutable context snapshots for each scope and stable
actions such as opening a thread, selecting a workspace view, sending a prompt,
opening settings, toggling a panel, and running a registered command.

Context versions are explicit. New optional fields are backward compatible;
removing or changing a field requires a new context version. Plugins should
subscribe only to the scopes they render so streaming updates do not rerender
the entire customized shell.

## Activation and Recovery

An enabled desktop entry is read only after its exact package fingerprint is
approved. Wuu serves the entry through a content-addressed, Wuu-owned Electron
protocol. The renderer never receives an absolute package path or Node access.

Activation is transactional per plugin generation:

1. load the content-addressed module;
2. register contributions into a pending generation;
3. atomically swap the generation after registration succeeds;
4. dispose old zones, surfaces, styles, locales, and host resources;
5. record boot progress and diagnostics.

A failed generation leaves the previous generation active when possible. A
renderer-blocking plugin is suppressed on the next launch, and safe mode starts
with all third-party desktop entries disabled.

## Compatibility Rules

- Public IDs, contexts, actions, and UI primitives are versioned SDK contracts.
- Private component imports and private DOM selectors have no compatibility
  guarantee.
- Plugin bundles use the host React runtime; shipping a second React copy is not
  supported.
- Every contribution is attributed to one plugin generation and is removable.
- Declarative themes work in future shells when they support the same tokens;
  desktop code and React surfaces are Electron-shell contributions.
- A plugin may make Wuu look entirely different, but it may not silently bypass
  trust, recovery, accessibility, or native security boundaries.

## Delivery Order

1. Add the approved, content-addressed desktop bundle read/load path.
2. Mount one global plugin host and activate inventory generations.
3. Add surface registration and a `PluginSurface` fallback renderer.
4. Connect the app shell and major views as surfaces; connect local controls as
   zones.
5. Apply declarative themes through a host-owned theme manager.
6. Publish the typed SDK, UI primitives, example plugin, diagnostics, safe mode,
   and compatibility tests.

