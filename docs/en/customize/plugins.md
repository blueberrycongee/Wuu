# Plugins

Plugins let you reassemble Wuu while keeping its secure host core intact:
change themes, add settings, extend the agent's tools and policies, and make
bounded structural contributions to the desktop UI. The plugin platform is
currently local-first — there is no marketplace or central registry, and
plugins are installed locally as directories or zip packages.

Plugins fall into three categories with different trust costs:

| Type | What it does | Code required? |
| --- | --- | --- |
| Declarative themes and settings | Declared in `plugin.json`; no plugin code executes | No |
| Agent plugins | Register tools, contribute system prompts and pre-model context, transform requests, observe turn lifecycle | Yes, separate process |
| Desktop plugins | Register styles, replace or wrap stable UI surfaces | Yes, Renderer code |

Plugin management, approval, safe mode, crash recovery, permission bottom
lines, and native window lifecycle are always controlled by Wuu; plugins
cannot replace these recovery paths. A strong-style appearance plugin can
restyle the whole product through public tokens, the UI Kit, and semantic
anchors, but window safe areas, navigation structure, tabs, scrolling,
overflow, and recovery entries remain host-owned.

For how feature plugins, appearance plugins, and the host compose, see the
[plugin system architecture](../../zh-cn/customize/plugin-system.md) document
(currently in Chinese).

## Getting and installing

Plugin authors usually maintain plugins in their own GitHub repositories. You
can clone a repository, download a directory, or download a release package
(zip), then install it from the desktop app or the CLI:

```bash
wuu plugin install ./path/to/plugin
wuu plugin install ./my-plugin-1.0.0.zip
```

In the desktop app, open **Skills & Plugins** and click **Install local
plugin**, then pick a directory or zip package. Plugin files are installed
under `~/.wuu/plugins/` (or below `WUU_HOME` when set). Before every load, the
installed package's whole-package fingerprint is recomputed and its approval
state is checked.

## Approval and enabling

Installed code is not activated immediately. Wuu shows the package's source,
content, and fingerprint; the plugin is enabled only after you review and
approve it. No plugin code runs before approval. Any file change in the
package produces a new fingerprint, invalidating the previous approval and
requiring a fresh review and approval. Installing the same plugin again
stages it as a pending update; the installed version keeps running until you
approve the new package.

Common management commands:

```bash
wuu plugin list                       # list installed plugins and their state
wuu plugin approve my-plugin          # review then approve
wuu plugin reject my-plugin
wuu plugin enable my-plugin
wuu plugin disable my-plugin
wuu plugin remove my-plugin
wuu plugin inspect ./path/to/plugin   # inspect package content and fingerprint before installing
```

`wuu plugin inspect` is useful for seeing what a package will do and which
permissions it requests before you install it.

## What plugins can do

- **Change themes**: themes of approved and enabled plugins appear under
  **Settings → Appearance**; tokens are removed completely when the plugin is
  disabled or the user returns to a built-in theme. Themes need only a
  `plugin.json` declaration, no code.
- **Add settings**: plugins can declare their own settings (booleans, text,
  numbers, enums) that generate controls in the settings UI, stored under the
  plugin namespace. Disabling, upgrading, or uninstalling a plugin keeps its
  settings and Storage by default so they can be restored later; data is not
  implicitly deleted today.
- **Extend the agent**: Agent plugins run as separate processes and can
  register model-visible tools, contribute system prompts and persistent
  pre-step context, participate in request rewriting and summary compaction,
  and observe the turn lifecycle. A package can also declare standalone
  commands or model hooks through the manifest; they are not runtime
  capabilities — see [Hooks](../../zh-cn/customize/hooks.md) (Chinese) for
  events and trust boundaries.
- **Customize the desktop UI**: register global styles, place persistent
  Views in stable regions, replace or wrap semantic Presenters (messages,
  tool activity, navigation, settings, and more), and insert content into
  fixed Slots. A render failure only falls back within the current boundary;
  settings, disabling, and default-UI recovery always remain available.

## Trust boundary

- Declarative themes can only modify public semantic tokens and are safe to
  install directly.
- Agent plugin runtime processes share the same user authority as Wuu;
  desktop plugins can register arbitrary CSS. Install only from sources you
  trust for these two categories, and review package content before enabling.
- Plugin packages are treated as untrusted input: the Renderer never reads
  absolute plugin paths. Before loading, app-server recomputes the
  fingerprint and confirms approval and enablement; the Electron main process
  verifies the digest again and loads through a content-addressed
  `wuu-plugin:` URL. The Content Security Policy does not enable
  `unsafe-eval`.
- Hooks declared by plugins carry the same risk as running a third-party
  local command directly.

## Current boundaries

The compatibility promise that plugins keep working across Wuu minor
versions (developers keep up without forking) is the platform's current
completion gate, but it has not been verified yet. Until then, plugins may
need adjustments as Wuu upgrades; declaring `minimum_wuu_version` when
publishing prevents incompatible combinations from activating.

Plugins can also declare simple package relationships: a plugin missing its
`requires` does not start; when `breaks` hits, Wuu refuses to enable both;
`conflicts` only shows a potential-conflict hint and never picks or
auto-disables a plugin for you. There is no version range, SAT solving, or
combination scoring today.

## Writing plugins

To develop your own plugins, read [Writing plugins](plugin-authoring.md):
package structure and manifest, the Agent and Desktop plugin APIs, the local
development loop (`wuu plugin create/build/test/dev/pack`), and the
installable, developable examples in the repository.
