export async function activate(api) {
  const h = api.react.createElement;

  api.registerPresenter({
    id: "command-tool-card",
    target: "conversation.tool-activity",
    key: "command.bash",
    mode: "replace",
    priority: 40,
    render({ snapshot, fallback }) {
      if (!isToolActivitySnapshot(snapshot)) return fallback;
      return h(
        "section",
        {
          className: "tool-card-skin",
          "data-status": snapshot.status,
          "data-contract-version": String(snapshot.contractVersion),
        },
        h(
          "header",
          { className: "tool-card-skin-header" },
          h("span", { className: "tool-card-skin-signal", "aria-hidden": true }),
          h("span", { className: "tool-card-skin-title" }, snapshot.kind || "Command"),
          h("span", { className: "tool-card-skin-status" }, statusLabel(snapshot.status)),
        ),
        h("div", { className: "tool-card-skin-content" }, fallback),
      );
    },
  });

  api.registerCSSSnippet({
    id: "tool-card-skin",
    priority: 40,
    css: `
      .tool-card-skin {
        overflow: hidden;
        border: var(--wuu-border-subtle, 1px solid var(--wuu-color-border-subtle));
        border-radius: var(--wuu-radius-panel, 10px);
        background: var(--wuu-color-surface, transparent);
        box-shadow: var(--wuu-elevation-panel, none);
      }
      .tool-card-skin-header {
        display: grid;
        grid-template-columns: auto minmax(0, 1fr) auto;
        align-items: center;
        gap: calc(var(--wuu-space-unit, 4px) * 2 * var(--wuu-space-density, 1));
        min-width: 0;
        padding: calc(var(--wuu-space-unit, 4px) * 2 * var(--wuu-space-density, 1));
        border-bottom: 1px solid var(--wuu-color-border-subtle, currentColor);
        color: var(--wuu-color-text-muted, currentColor);
        font: 600 var(--wuu-font-size-caption, 12px)/1.3 var(--wuu-font-family-ui, sans-serif);
      }
      .tool-card-skin-signal {
        width: 8px;
        height: 8px;
        border-radius: 999px;
        background: var(--wuu-color-text-muted, currentColor);
      }
      .tool-card-skin[data-status="running"] .tool-card-skin-signal {
        background: var(--wuu-color-accent, currentColor);
      }
      .tool-card-skin[data-status="failed"] .tool-card-skin-signal {
        background: var(--wuu-color-danger, currentColor);
      }
      .tool-card-skin-title {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .tool-card-skin-status {
        white-space: nowrap;
      }
      .tool-card-skin-content {
        min-width: 0;
        padding: calc(var(--wuu-space-unit, 4px) * 2 * var(--wuu-space-density, 1));
      }
    `,
  });
}

function isToolActivitySnapshot(value) {
  return value !== null
    && typeof value === "object"
    && value.contractVersion === 1
    && typeof value.status === "string";
}

function statusLabel(status) {
  if (status === "running") return "Running";
  if (status === "failed") return "Failed";
  return "Completed";
}
