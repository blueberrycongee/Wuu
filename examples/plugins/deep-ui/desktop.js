export async function activate(api) {
  const React = api.react;

  api.registerStyle({
    id: "deep-ui-example",
    css: `
      .deep-ui-frame {
        position: relative;
        min-width: 0;
        min-height: 0;
      }
      .deep-ui-frame::before {
        content: "";
        position: absolute;
        inset: 0;
        pointer-events: none;
        border: 1px solid color-mix(in srgb, var(--wuu-accent) 24%, transparent);
        border-radius: 12px;
        z-index: 1;
      }
      .deep-ui-timeline {
        padding-inline: 6px;
      }
      .deep-ui-catalog {
        max-width: 1180px;
        margin-inline: auto;
      }
    `,
  });

  for (const [surface, className] of [
    ["app.sidebar", "deep-ui-frame"],
    ["conversation.composer", "deep-ui-frame"],
    ["conversation.timeline", "deep-ui-timeline"],
    ["view.settings", "deep-ui-frame"],
    ["view.catalog", "deep-ui-catalog"],
  ]) {
    api.registerSurface(surface, {
      id: `frame-${surface}`,
      mode: "wrap",
      render(_context, fallback) {
        return React.createElement("div", { className }, fallback);
      },
    });
  }
}
