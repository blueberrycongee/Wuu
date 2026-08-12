export async function activate(api) {
  const h = api.react.createElement;

  api.registerInspectorSection({
    id: "current-todo",
    title: "TODO",
    priority: 10,
    when(snapshot) {
      return Array.isArray(snapshot.todo?.items) && snapshot.todo.items.length > 0;
    },
    render({ snapshot }) {
      return renderTodos(h, snapshot.todo?.items);
    },
  });

  api.registerPresenter({
    id: "todo-tool",
    target: "conversation.tool-activity",
    key: "todo",
    mode: "replace",
    priority: 20,
    render({ snapshot, fallback }) {
      const items = parseTodos(snapshot.argumentsText);
      return items ? renderTodos(h, items) : fallback;
    },
  });

  api.registerCSSSnippet({
    id: "todo-layout",
    css: `
      .plugin-todo-list {
        display: grid;
        gap: calc(var(--wuu-space-unit, 4px) * 1.5 * var(--wuu-space-density, 1));
        margin: 0;
        padding: 0;
        list-style: none;
      }
      .plugin-todo-item {
        display: grid;
        grid-template-columns: 18px minmax(0, 1fr);
        gap: calc(var(--wuu-space-unit, 4px) * 2 * var(--wuu-space-density, 1));
        align-items: start;
        min-width: 0;
      }
      .plugin-todo-marker {
        color: var(--wuu-color-text-muted, currentColor);
        text-align: center;
      }
      .plugin-todo-item[data-status="in_progress"] .plugin-todo-marker {
        color: var(--wuu-color-accent, currentColor);
      }
      .plugin-todo-item[data-status="completed"] {
        color: var(--wuu-color-text-muted, currentColor);
      }
    `,
  });
}

function parseTodos(raw) {
  if (typeof raw !== "string" || raw.trim() === "") return undefined;
  try {
    const value = JSON.parse(raw);
    return Array.isArray(value?.todos) ? value.todos : undefined;
  } catch {
    return undefined;
  }
}

function renderTodos(h, items) {
  if (!Array.isArray(items) || items.length === 0) return null;
  return h(
    "ol",
    { className: "plugin-todo-list" },
    items.map((item, index) => h(
      "li",
      { className: "plugin-todo-item", "data-status": item.status, key: `${index}:${item.content}` },
      h("span", { className: "plugin-todo-marker", "aria-hidden": true }, marker(item.status)),
      h("span", null, item.content),
    )),
  );
}

function marker(status) {
  if (status === "completed") return "✓";
  if (status === "in_progress") return "●";
  return "○";
}
