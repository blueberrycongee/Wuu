# Roadmap

[简体中文](ROADMAP_zh.md)

wuu is a pre-1.0 project. The current priority is to make its existing coding
workflows reliable and easy to inspect before expanding into larger workspace
and multi-agent features.

This is a direction, not a release schedule. Follow the linked issues for full
designs and progress. See the [changelog](CHANGELOG.md) for shipped work.

## Long-term direction

wuu aims to become an open, composable GUI agent platform rather than a single
fixed agent workflow. The application should remain useful out of the box, while
letting users choose how an agent works and which product capabilities appear in
the workbench.

The long-term architecture separates a small **plugin kernel** from replaceable
product behavior:

- The kernel owns durable sessions and events, reliable input admission,
  execution ownership, cancellation, recovery, provider and tool protocol
  correctness, final permission decisions, plugin lifecycle, and recoverable
  application chrome.
- Agent loop drivers decide how inputs are consumed, prompts are assembled,
  tools are scheduled, and an agent retries, plans, delegates, continues, or
  stops. The bundled loop will become the default first-party driver rather than
  a permanent definition of how every wuu agent must operate.
- First-party and community plugins should use the same public contracts for
  tools, prompts, services, events, storage, sessions, views, settings, and
  presentation. Product-specific behavior should not require private branches
  in the kernel.
- Functional plugins and appearance plugins should compose independently. The
  host retains layout, accessibility, overflow, safe areas, and recovery paths;
  plugins contribute capabilities and content through stable semantic surfaces.

This direction is informed by open-source projects including Pi, Cordis, and
HashiCorp go-plugin. wuu does not aim to copy their runtime implementations or
claim compatibility. It adapts the general ideas of small loops, service
composition, scoped lifecycle ownership, and supervised process plugins to a Go
core and an Electron shell.

## Plugin architecture migration

The migration is designed to preserve a usable application at every stage. It
is not a promise that every item will ship in one release.

1. **Unify plugin ownership.** Bring runtime processes, background work, event
   subscriptions, session ownership, and workbench contributions under one
   generation-scoped lifecycle. Disabling or updating a plugin must stop new
   work, settle in-flight work, and remove all contributions predictably.
2. **Make dependencies explicit.** Build one validated activation graph for the
   core and workbench sides of a plugin. A plugin waits for required services,
   fails clearly when they are unavailable, and cannot depend on accidental load
   order.
3. **Stabilize service and event contracts.** Expose versioned, product-neutral
   services for sessions, providers, tools, permissions, checkpoints, storage,
   and UI contributions. Keep safety and protocol invariants in the host.
4. **Introduce a loop-driver contract.** Run the existing agent behavior through
   the new contract first, without changing the user experience. Bind each
   session to a driver identity and checkpoint version so recovery never guesses
   how to interpret another driver's state.
5. **Move the default loop into a bundled plugin.** Once recovery and lifecycle
   contracts are proven, migrate the bundled loop and the behavior that truly
   belongs to it. Current implementation placement is not automatically a
   permanent kernel boundary.
6. **Prove replaceability.** Ship or maintain a second, structurally different
   driver that can create, run, resume, and present a session without changes to
   the kernel. This is the acceptance test for a real loop ecosystem.
7. **Grow the public ecosystem carefully.** Add compatibility gates, diagnostics,
   development tooling, documentation, and distribution features based on real
   external plugins. Marketplace and signing decisions will follow demonstrated
   needs rather than speculative APIs.

The migration follows several durable rules:

- Mechanisms stay in the kernel; policies belong to drivers and product plugins.
- Anything visible to a model must be reconstructable from durable session facts.
- Plugin updates are transactional: a failed candidate never replaces a working
  generation.
- Missing or incompatible plugins must not make session history inaccessible;
  the host provides a safe read-only fallback and explicit migration choices.
- Executable plugins are trusted local application code. Fingerprint approval,
  least-privilege host services, explicit permissions, and recovery paths remain
  mandatory even as extension points become more powerful.

## Current focus

- **Converge the plugin runtime around scoped lifecycle ownership.** Existing
  extension points already support substantial agent and workbench additions,
  but lifecycle, dependencies, and activation are not yet one coherent runtime
  model. The next architecture phase will establish that foundation before the
  default loop becomes replaceable.

- **Make background-work lifecycles predictable.** Background commands and
  processes that survive an app-server restart currently have conflicting
  ownership and recovery rules, making it hard to know whether work is still
  alive or controllable. We want one clear lifecycle.
  ([#157](https://github.com/blueberrycongee/wuu/issues/157))

- **Make background commands easier to review.** Command output can now be
  revisited in the terminal workspace, but the environment panel still cannot
  list live background processes for the current session or open their terminal
  resources directly.
  ([#103](https://github.com/blueberrycongee/wuu/issues/103))

- **Complete repository state in the environment panel.** The environment panel
  still does not show enough upstream, PR, or CI state.
  ([#57](https://github.com/blueberrycongee/wuu/issues/57))

- **Keep model support current and usage understandable.** The bundled model
  catalog is fixed at build time, so new or corrected model information requires
  another wuu release. Provider token totals also cannot explain which request
  components produced fresh input. We want runtime catalog updates and useful
  attribution without storing prompt content.
  ([#148](https://github.com/blueberrycongee/wuu/issues/148),
  [#119](https://github.com/blueberrycongee/wuu/issues/119))

## Planned

- **Reduce setup work when moving from another coding agent.** Existing project
  instructions, preferences, and other useful settings currently have to be
  found and recreated by hand. wuu should discover compatible settings, explain
  their source and destination, and let the user choose what to import without
  silently copying credentials or enabling executable extensions.
  ([#153](https://github.com/blueberrycongee/wuu/issues/153))

- **Give generated work a persistent place beside the conversation.** Today,
  interactive results are limited to the message flow and office documents do
  not have a first-class preview workspace. We want chat-driven web, DOCX, and
  PPTX creation with the current artifact visible beside the conversation while
  files remain the source of truth.
  ([#154](https://github.com/blueberrycongee/wuu/issues/154),
  [#20](https://github.com/blueberrycongee/wuu/issues/20))

## Exploring

This problem is important, but the solution is not scheduled:

- **The embedded webview cannot reuse a user's existing browser profile and
  limits deeper agent integration.** Explore a fuller browser surface with
  explicit credential and permission controls.
  ([#96](https://github.com/blueberrycongee/wuu/issues/96))

Priorities may change when core bugs, security issues, or user feedback reveal a
more important problem. Suggestions are welcome in
[GitHub Issues](https://github.com/blueberrycongee/wuu/issues).
