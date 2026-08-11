# Desktop plugin quickstart

This tutorial adds an interactive control to the Wuu Composer and walks through
creation, build, hot reload, packaging, and installation. A Desktop plugin runs trusted
React code in the Wuu Renderer without requiring a fork.

## Prerequisites

- The `wuu` CLI (`wuu plugin --help` must work)
- Node.js 18+
- A running Wuu Desktop app

## Step 1: generate a Desktop skeleton

```bash
wuu plugin create --type desktop focus-mode
cd focus-mode
npm install
```

The package contains `plugin.json`, TypeScript configuration, and `src/index.ts`. The
manifest points its Desktop entry at the compiled `dist/index.js`:

```json
{
  "schema_version": 1,
  "id": "focus-mode",
  "name": "focus-mode",
  "version": "0.1.0",
  "desktop": {
    "entry": "dist/index.js"
  }
}
```

The Desktop entry exports `activate(api)`. Registrations belong to that plugin
generation and are reclaimed together on disable, upgrade, or removal.

## Step 2: add a Composer control

Replace `src/index.ts` with:

```ts
import type { PluginGenerationApi } from "@wuu/plugin-sdk";

export function activate(api: PluginGenerationApi): void {
  const React = api.react;
  const ToolbarToggle = api.ui.ToolbarToggle as unknown as (
    props: Readonly<Record<string, unknown>>,
  ) => unknown;

  function FocusToggle() {
    const [enabled, setEnabled] = React.useState(false);
    return React.createElement(
      ToolbarToggle,
      {
        pressed: enabled,
        "aria-label": "Toggle focus mode",
        onClick: () => setEnabled((value) => !value),
      },
      enabled ? "Focused" : "Focus",
    );
  }

  api.registerSlot("composer.toolbar", {
    id: "focus-toggle",
    order: 20,
    render() {
      return React.createElement(FocusToggle, null);
    },
  });
}
```

This uses three core capabilities:

- `api.react` uses the host React instance; do not bundle another copy.
- `api.ui` inherits Wuu themes, density, and accessibility behavior.
- `composer.toolbar` adds content to the host-owned Composer instead of querying
  private DOM.

## Step 3: build and check

```bash
npm run build
wuu plugin validate .
wuu plugin test .
```

The Desktop entry must be a self-contained ESM file inside the package. Type-only
imports disappear at compile time. Do not leave relative imports to plugin source or
bundle another React runtime.

## Step 4: hot reload in the real app

```bash
wuu plugin dev .
```

Development mode authorizes the current directory. Each save builds and activates an
atomic candidate generation; a failed candidate leaves the previous generation
running. Click **Focus** in the Composer toolbar and verify that its state changes.

No Wuu source rebuild is needed when only plugin code changes.

## Step 5: package and install

```bash
wuu plugin pack .
wuu plugin inspect ./focus-mode-0.1.0.zip
wuu plugin install ./focus-mode-0.1.0.zip
wuu plugin approve focus-mode
wuu plugin enable focus-mode
```

Any package change creates a new fingerprint and requires review. Development-directory
authorization never transfers with the zip.

## Next steps

- Use the [Desktop UI extension map](desktop-plugins.md) to choose a View, Slot,
  Presenter, or Surface.
- Follow the [Desktop plugin recipes](plugin-recipes.md) for selection UI, draft
  updates, and full panels.
- Look up manifest, settings, Storage, and complete APIs in the
  [plugin authoring reference](plugin-authoring.md).
- Add Agent tools with a `--type full` package or the
  [Agent plugin quickstart](plugin-quickstart.md).
