# Desktop plugin recipes

This page organizes common combinations by user outcome. Read the
[Desktop UI extension map](desktop-plugins.md) first and generate a working package
with the [Desktop plugin quickstart](desktop-plugin-quickstart.md).

## Add a Composer button

Use the `composer.toolbar` Slot with `api.ui.ToolbarToggle` or `api.ui.Button`. Wuu owns
toolbar layout, theming, and accessibility; the plugin owns its state and behavior.

```ts
api.registerSlot("composer.toolbar", {
  id: "my-action",
  render() {
    return api.react.createElement(
      api.ui.Button,
      { onClick: () => console.log("clicked") },
      "Run",
    );
  },
});
```

If the control must read or change the draft, use a `conversation.composer` Presenter
instead of querying the textarea.

## Add selected message text to the Composer

This feature combines three pieces:

1. the standard Selection API reads text and screen coordinates;
2. the public `data-wuu-component="message"` anchor confirms the selection is in chat;
3. a Composer Presenter invokes the host `set-draft` action.

It does not replace message rendering or mutate private React state:

```ts
import type {
  ComposerSnapshotV1,
  PluginGenerationApi,
  PresenterProps,
} from "@wuu/plugin-sdk";

const SET_DRAFT = "conversation.composer.set-draft";

export function activate(api: PluginGenerationApi): void {
  const React = api.react;

  function SelectionToolbar(input: Readonly<Record<string, unknown>>) {
    const props = input.presenter as PresenterProps;
    const snapshot = props.snapshot as ComposerSnapshotV1;
    const [selection, setSelection] = React.useState<null | {
      text: string;
      left: number;
      top: number;
    }>(null);

    React.useEffect(() => {
      const update = () => {
        const current = window.getSelection();
        if (!current || current.isCollapsed || current.rangeCount === 0) {
          setSelection(null);
          return;
        }

        const node = current.anchorNode;
        const element = node instanceof Element ? node : node?.parentElement;
        const text = current.toString().trim();
        if (!text || !element?.closest('[data-wuu-component="message"]')) {
          setSelection(null);
          return;
        }

        const rect = current.getRangeAt(0).getBoundingClientRect();
        setSelection({ text, left: rect.left, top: rect.top - 40 });
      };

      document.addEventListener("selectionchange", update);
      return () => document.removeEventListener("selectionchange", update);
    }, []);

    const append = async () => {
      if (!selection || !props.host.actions.includes(SET_DRAFT)) return;
      const current = snapshot.draftText?.trimEnd() ?? "";
      const quote = `> ${selection.text.replaceAll("\n", "\n> ")}`;
      await props.host.invoke(SET_DRAFT, current ? `${current}\n\n${quote}\n\n` : `${quote}\n\n`);
      setSelection(null);
    };

    return React.createElement(
      "div",
      { style: { display: "contents" } },
      props.fallback,
      selection && React.createElement(
        "div",
        {
          style: {
            position: "fixed",
            left: selection.left,
            top: selection.top,
            zIndex: 1000,
          },
          onMouseDown: (event: { preventDefault(): void }) => event.preventDefault(),
        },
        React.createElement(api.ui.Button, { onClick: append }, "Add to Composer"),
      ),
    );
  }

  api.registerPresenter({
    id: "selection-toolbar",
    target: "conversation.composer",
    mode: "wrap",
    render: (props) => React.createElement(SelectionToolbar, { presenter: props }),
  });
}
```

`onMouseDown.preventDefault()` keeps the browser from clearing the selection before
the click. The plugin stores plain text and coordinates, not host nodes or React
objects. A production implementation should also clamp to the viewport and handle
scrolling, long selections, and localization.

Translation, summary, and explanation buttons can reuse the selection and place
different prompt templates in the draft. Let the user review and add their query by
default. For intentional automatic submission, verify that `submit` appears in
`host.actions` before invoking `conversation.composer.submit`.

## Add a full workspace tool

Use a View for lists, editors, charts, and management UI:

```ts
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
});
```

Declare a `workspaceTools`, `navigation`, or `settingsPages` entry in `plugin.json`
that points to the registered View. Wuu owns opening, closing, tabs, and persistence.

## Change messages or tool activity cards

- Add content around a message with `conversation.message.before/after` Slots.
- Wrap one item with a `conversation.item` Presenter.
- Wrap the whole message boundary with the `conversation.message` Surface.
- Present tool cards by stable capability with a `conversation.tool-activity` Presenter.

Read public snapshots and retain a useful fallback. Do not parse private ThreadItems,
guess tool-internal state, or depend on host class names.

## Add a code-free theme

For colors, radii, or syntax highlighting, declare a theme in the manifest instead of
loading Desktop code:

```json
{
  "contributes": {
    "themes": [
      {
        "id": "calm-night",
        "name": "Calm Night",
        "base": "dark",
        "tokens": {
          "--wuu-color-canvas": "#151820",
          "--wuu-color-accent": "#8fa7ff"
        }
      }
    ]
  }
}
```

Run `wuu plugin validate .`, then install, approve, and enable the package. The theme
appears under **Settings → Appearance**; selecting a built-in theme removes its
overrides. See the [theme token reference](theme-surface-matrix.md) for available
tokens and [plugin themes and settings](themes-settings.md) for user actions.

## Show a result after background work

A Desktop module may register a Command or call `showConversationCard` from a host
event or asynchronous task. Use a Card for short-lived interaction; use a View for
persistent, navigable, or complex UI.

See the [plugin authoring reference](plugin-authoring.md) for settings, Storage,
Runtime communication, and packaging rules.
