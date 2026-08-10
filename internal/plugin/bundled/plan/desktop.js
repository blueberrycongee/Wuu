export async function activate(api) {
  const h = api.react.createElement;

  api.registerInspectorSection({
    id: "current-plan",
    title: "Plan",
    priority: 10,
    render({ snapshot }) {
      return renderPlan(h, snapshot.plan?.items);
    },
  });

  api.registerPresenter({
    id: "plan-tool",
    target: "conversation.tool-activity",
    key: "plan",
    mode: "replace",
    priority: 20,
    render({ snapshot, fallback }) {
      const items = parsePlan(snapshot.argumentsText);
      return items ? renderPlan(h, items) : fallback;
    },
  });

  api.registerCSSSnippet({
    id: "plan-layout",
    css: `
      .plugin-plan-list {
        display: grid;
        gap: calc(var(--wuu-space-unit, 4px) * 1.5 * var(--wuu-space-density, 1));
        margin: 0;
        padding: 0;
        list-style: none;
      }
      .plugin-plan-item {
        display: grid;
        grid-template-columns: 18px minmax(0, 1fr);
        gap: calc(var(--wuu-space-unit, 4px) * 2 * var(--wuu-space-density, 1));
        align-items: start;
        min-width: 0;
      }
      .plugin-plan-marker {
        color: var(--wuu-color-text-muted, currentColor);
        text-align: center;
      }
      .plugin-plan-item[data-status="in_progress"] .plugin-plan-marker {
        color: var(--wuu-color-accent, currentColor);
      }
      .plugin-plan-item[data-status="completed"] {
        color: var(--wuu-color-text-muted, currentColor);
      }
    `,
  });
}

function parsePlan(raw) {
  if (typeof raw !== "string" || raw.trim() === "") return undefined;
  try {
    const value = JSON.parse(raw);
    return Array.isArray(value?.plan) ? value.plan : undefined;
  } catch {
    return undefined;
  }
}

function renderPlan(h, items) {
  if (!Array.isArray(items) || items.length === 0) return null;
  return h(
    "ol",
    { className: "plugin-plan-list" },
    items.map((item, index) => h(
      "li",
      { className: "plugin-plan-item", "data-status": item.status, key: `${index}:${item.step}` },
      h("span", { className: "plugin-plan-marker", "aria-hidden": true }, marker(item.status)),
      h("span", null, item.step),
    )),
  );
}

function marker(status) {
  if (status === "completed") return "✓";
  if (status === "in_progress") return "●";
  return "○";
}
