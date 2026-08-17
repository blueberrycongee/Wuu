export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  api.registerLocale({ id: "subagent-en", locale: "en-US", entries: {
    "subagent.aliases.intro": "Route delegated child tasks to specific models by naming aliases here.",
    "subagent.aliases.title": "Model aliases",
    "subagent.aliases.help": "A JSON object mapping alias names to model identifiers, applied to newly spawned child tasks.",
    "subagent.aliases.label": "Aliases (JSON)",
    "subagent.aliases.save": "Save aliases",
    "subagent.aliases.saved": "Saved",
    "subagent.aliases.invalid": "Aliases must be a JSON object."
  } });
  api.registerLocale({ id: "subagent-zh", locale: "zh-CN", entries: {
    "subagent.aliases.intro": "通过别名把委派的子任务路由到指定模型。",
    "subagent.aliases.title": "模型别名",
    "subagent.aliases.help": "一个 JSON 对象，把别名映射到模型标识，对之后创建的子任务生效。",
    "subagent.aliases.label": "别名配置（JSON）",
    "subagent.aliases.save": "保存别名",
    "subagent.aliases.saved": "已保存",
    "subagent.aliases.invalid": "别名配置必须是 JSON 对象。"
  } });
  api.registerStyle({ id: "subagent-status", css: `
    .plugin-subagent-settings { min-width:0; }
    .plugin-subagent-intro { margin:0; max-width:52ch; color:var(--wuu-color-text-muted,var(--ink-soft)); font-size:var(--font-ui,13px); line-height:1.5; }
    .plugin-subagent-field textarea { width:100%; min-height:180px; resize:vertical; font:12px/1.6 var(--wuu-font-mono,ui-monospace,monospace); font-variant-numeric:tabular-nums; }
    .plugin-subagent-actions { align-items:center; gap:8px; }
    .plugin-subagent-saved { color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:var(--font-sm,12px); animation:plugin-subagent-fade 2.4s ease both; }
    @keyframes plugin-subagent-fade { 0% { opacity:0; } 12% { opacity:1; } 70% { opacity:1; } 100% { opacity:0; } }
    .plugin-subagent-settings-error { color:var(--wuu-color-danger); font-size:var(--font-ui,13px); overflow-wrap:anywhere; }
  ` });

  const emptyStatusItems = Object.freeze([]);
  const statusItemsByThread = new Map();
  const statusListenersByThread = new Map();
  const refreshGenerationByThread = new Map();
  const statusState = (state) => {
    if (state === "running") return "running";
    if (state === "queued" || state === "pending") return "queued";
    if (state === "failed") return "error";
    if (state === "waiting_children" || state === "awaiting_report") return "waiting";
    return "idle";
  };
  const refreshStatus = async (threadId) => {
    if (!threadId) return;
    const generation = (refreshGenerationByThread.get(threadId) || 0) + 1;
    refreshGenerationByThread.set(threadId, generation);
    let tasks = [];
    try {
      const value = await api.invokeRuntime("status.list", { parent_session_id: threadId });
      tasks = Array.isArray(value?.sessions) ? value.sessions : [];
    } catch {
      tasks = [];
    }
    if (refreshGenerationByThread.get(threadId) !== generation) return;
    const items = tasks
      .filter((task) => task && task.session_id && task.state && task.state !== "completed")
      .map((task) => Object.freeze({
        id: task.session_id,
        label: task.name || "Child task",
        state: statusState(task.state),
        secondaryText: String(task.state).replaceAll("_", " "),
        tooltip: task.name || "Child task",
        updatedAt: typeof task.updated_at === "string" ? task.updated_at : undefined,
        action: Object.freeze({ kind: "open-session", sessionId: task.session_id }),
      }));
    statusItemsByThread.set(threadId, Object.freeze(items));
    for (const listener of statusListenersByThread.get(threadId) || []) listener();
  };
  api.onHostEvent((event) => {
    const method = event && typeof event === "object" && event.message && typeof event.message === "object" ? event.message.method : "";
    if (typeof method === "string" && [
      "thread/started", "thread/resumed", "thread/updated", "turn/started", "turn/queued",
      "turn/dequeued", "turn/held", "turn/completed", "turn/error",
    ].includes(method)) {
      for (const threadId of statusListenersByThread.keys()) void refreshStatus(threadId);
    }
  });
  api.registerComposerStatusSource({
    id: "subagent-status",
    order: 30,
    getSnapshot: ({ threadId }) => threadId ? statusItemsByThread.get(threadId) || emptyStatusItems : emptyStatusItems,
    subscribe: ({ threadId }, listener) => {
      if (!threadId) return () => {};
      let listeners = statusListenersByThread.get(threadId);
      if (!listeners) {
        listeners = new Set();
        statusListenersByThread.set(threadId, listeners);
      }
      listeners.add(listener);
      void refreshStatus(threadId);
      return () => {
        listeners.delete(listener);
        if (listeners.size === 0) {
          statusListenersByThread.delete(threadId);
          refreshGenerationByThread.set(threadId, (refreshGenerationByThread.get(threadId) || 0) + 1);
        }
      };
    },
  });

  function ModelAliases({ host, translate }) {
    const { Page, Section, Stack, Row, Button, TextArea } = api.ui;
    const tr = typeof translate === "function" ? translate : (key) => key;
    const settings = host.settings;
    const aliases = settings ? settings.getValue("runtime.modelAliases") : {};
    const [draft, setDraft] = React.useState(() => JSON.stringify(aliases, null, 2));
    const [error, setError] = React.useState("");
    const [busy, setBusy] = React.useState(false);
    const [savedTick, setSavedTick] = React.useState(0);
    React.useEffect(() => setDraft(JSON.stringify(aliases, null, 2)), [JSON.stringify(aliases)]);
    const save = async () => {
      setBusy(true);
      try {
        const value = JSON.parse(draft);
        if (!value || Array.isArray(value) || typeof value !== "object") throw new Error(tr("subagent.aliases.invalid"));
        if (!settings) throw new Error("Settings update is unavailable.");
        await settings.updateValue("runtime.modelAliases", value);
        setError("");
        setSavedTick((tick) => tick + 1);
      } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)); }
      finally { setBusy(false); }
    };
    return h(Page, { className: "plugin-subagent-settings" }, h(Stack, { gap: "large" },
      h("p", { className: "plugin-subagent-intro" }, tr("subagent.aliases.intro")),
      h(Section, { title: tr("subagent.aliases.title"), description: tr("subagent.aliases.help") }, h(Stack, null,
        h("div", { className: "plugin-subagent-field" },
          h(TextArea, { label: tr("subagent.aliases.label"), value: draft, disabled: busy, spellCheck: false, onChange: (event) => setDraft(event.target.value) })),
        h(Row, { className: "plugin-subagent-actions" },
          h(Button, { variant: "primary", disabled: busy, onClick: () => void save() }, tr("subagent.aliases.save")),
          savedTick ? h("span", { key: savedTick, className: "plugin-subagent-saved" }, tr("subagent.aliases.saved")) : null),
        error ? h("div", { className: "plugin-subagent-settings-error", role: "alert" }, error) : null))));
  }
  api.registerViewType({ id: "subagent.settings", title: "Subagent", icon: "bot", defaultRegion: "settings", persistence: "durable", render: ModelAliases });
}
