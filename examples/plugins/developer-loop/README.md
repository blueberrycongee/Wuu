# Developer Loop Example

This independently buildable plugin uses only the published `@wuu/plugin-sdk` package. It keeps the
SDK v2 request transform and tool while contributing a durable workbench view, layout, complete
theme token sample, scoped CSS, command, status item, locales, and the conversation-header slot.
The view uses the React instance owned by the host; React is neither a dependency nor part of the
bundle. It reads all four setting kinds and restores its counter from plugin-namespaced storage.

Run the complete developer loop from this directory:

```sh
npm install
npm run build
npm test
wuu plugin test .
wuu plugin dev .
```

Install the built directory directly, or create a portable package first:

```sh
wuu plugin install .
wuu plugin pack .
wuu plugin install ./developer-loop-example-1.0.0.zip
```

`npm test` runs type checking, builds both entries, and then performs static contract checks. Those
checks reject private Wuu source imports and React bundling, and verify every acceptance
contribution. `wuu plugin test` starts the executable runtime and checks protocol negotiation, the
v2 capability descriptor, and executable tool registration. `wuu plugin dev` builds, validates,
and publishes isolated development generations; a failed refresh keeps the previous generation.

## Trust boundary

The manifest and renderer are untrusted plugin inputs. Wuu owns React, workbench lifecycle,
settings, namespaced storage, contribution disposal, and generation swaps. This plugin receives
only the public SDK API and view-scoped host methods. It cannot import Desktop or core internals,
and it must not assume access to host classes, global storage keys, or private DOM structure. The
executable runtime is a separate Node process speaking JSONL over standard input/output; only its
declared request transform and tool cross that boundary.
