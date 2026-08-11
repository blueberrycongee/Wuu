# Wuu Plugins

A Wuu Plugin is an installable, reviewable, and upgradeable extension package. It can
provide one capability or combine an Agent runtime, Desktop UI, themes, settings,
Skills, Hooks, MCP servers, and commands.

If you are not sure that you need a plugin, start with [Extend Wuu](index.md). A
repeatable instruction set may only need a Skill, and an existing tool service may
only need MCP. Use a Wuu Plugin when you need code lifecycle, host services, or
desktop UI.

The platform is local-first: there is no marketplace or central registry. Authors
normally develop and release plugins from their own GitHub repositories, and users
install a directory or zip without forking Wuu.

## What one package can contain

| Type | What it contributes | Code required? |
| --- | --- | --- |
| Declarative contributions | Themes, settings, Skills, Hooks, MCP servers, and commands | Depends on the contribution |
| Agent plugin | Tools, context, request transforms, turn observers, provided and consumed services | Yes, separate process |
| Desktop plugin | Views, Slots, Presenters, Surfaces, styles, and interactive cards | Yes, Renderer code |

One package may declare both `runtime` and `desktop.entry`. For example, the runtime
can query a private service while the Desktop module presents the result. Both share
the plugin ID, approval state, and generation lifecycle.

Plugin management, approval, safe mode, crash recovery, permission limits, and native
window lifecycle remain host-owned. Plugins cannot replace those recovery paths.

Choose a first path:

- [Agent plugin quickstart](plugin-quickstart.md) — register a model-visible tool and use Storage;
- [Desktop plugin quickstart](desktop-plugin-quickstart.md) — add a real Composer control;
- [Desktop UI extension map](desktop-plugins.md) — choose a View, Slot, Presenter, or Surface;
- [Desktop plugin recipes](plugin-recipes.md) — compose selection UI, draft actions, and full panels.

## Get and install plugins

Clone a repository, download a directory, or download a release zip, then install it
from the Desktop app or CLI:

```bash
wuu plugin install ./path/to/plugin
wuu plugin install ./my-plugin-1.0.0.zip
```

In the Desktop app, open **Skills & Plugins**, choose **Install local plugin**, and
select a directory or zip. Wuu installs packages under `~/.wuu/plugins/`, or below
`WUU_HOME` when set. It recalculates the whole-package fingerprint before every load.

## Approval and enabling

Installed code does not activate immediately. Review the source, package contents,
requested capabilities, and fingerprint before approval. Any package change produces
a new fingerprint and invalidates the old approval. Reinstalling the same ID stages a
pending update while the approved generation keeps running.

```bash
wuu plugin list
wuu plugin inspect ./path/to/plugin
wuu plugin approve my-plugin
wuu plugin reject my-plugin
wuu plugin enable my-plugin
wuu plugin disable my-plugin
wuu plugin remove my-plugin
```

## Common capabilities

- **Change themes:** declarative token themes appear under **Settings → Appearance**
  and are removed cleanly when disabled.
- **Add settings:** schema fields create host-rendered controls stored in the plugin
  namespace. Settings and Storage remain available after reinstall by default.
- **Extend the agent:** a managed runtime can register model-visible tools, contribute
  context, transform supported request fields, observe lifecycle events, and provide or
  consume versioned services.
- **Customize Desktop UI:** add persistent Views, insert fixed Slots, wrap or replace
  semantic Presenters and Surfaces, show conversation cards, and register styles.
- **Compose extension types:** carry Skills, Hooks, MCP servers, commands, Agent code,
  and Desktop code in one package with one install and upgrade lifecycle.

## Trust boundary

- Agent runtime processes run with the same user authority as Wuu.
- Desktop modules run in the Renderer and may register arbitrary CSS.
- Hooks have the same risk as running the declared local command directly.
- The Renderer never receives an absolute plugin path. App-server rechecks the
  fingerprint and approval state; Electron verifies the digest and imports through a
  content-addressed `wuu-plugin:` URL. CSP does not enable `unsafe-eval`.

Install code plugins only from trusted sources and review every changed generation.

## Current compatibility boundary

Keeping plugins working across Wuu minor releases without a fork is the platform's
current completion gate, but the compatibility matrix has not yet been verified.
Declare `minimum_wuu_version` and retest after Wuu upgrades.

Simple package relationships are available: missing `requires` blocks activation,
`breaks` prevents both plugins from being enabled, and `conflicts` shows a warning.
There is no version-range solver or automatic conflict resolution today.

## Develop and publish

```bash
wuu plugin create --type agent my-agent
wuu plugin create --type desktop my-ui
wuu plugin create --type full my-extension

wuu plugin validate ./my-extension
wuu plugin build ./my-extension
wuu plugin test ./my-extension
wuu plugin dev ./my-extension
wuu plugin pack ./my-extension
```

See the [plugin authoring reference](plugin-authoring.md) for the complete manifest,
Agent protocol, Desktop API, generations, and security boundaries. See the
[plugin system architecture](plugin-system.md) for the design rationale and host
ownership model.
