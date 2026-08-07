export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  api.registerLocale({ id: "subagent-en", locale: "en-US", entries: {
    "subagent.ultra.enable": "Enable proactive delegation",
    "subagent.ultra.disable": "Disable proactive delegation"
  } });
  api.registerLocale({ id: "subagent-zh", locale: "zh-CN", entries: {
    "subagent.ultra.enable": "开启主动委派",
    "subagent.ultra.disable": "关闭主动委派"
  } });
  api.registerStyle({ id: "subagent-status", css: `
    .plugin-subagent-status { display:flex; gap:6px; flex-wrap:wrap; margin:0 2px 8px; }
    .plugin-subagent-chip { padding:3px 7px; border:1px solid var(--wuu-color-border-subtle); border-radius:999px; background:var(--wuu-color-surface-muted); color:var(--wuu-color-text-muted); font-size:11px; }
    .plugin-subagent-chip[data-status="running"] { color:var(--wuu-color-text); }
    .plugin-subagent-settings { display:grid; gap:10px; padding:14px; border:1px solid var(--wuu-color-border-subtle); border-radius:10px; }
    .plugin-subagent-settings textarea { width:100%; min-height:120px; resize:vertical; font:12px/1.5 ui-monospace,monospace; color:var(--wuu-color-text); background:var(--wuu-color-surface-muted); border:1px solid var(--wuu-color-border-subtle); border-radius:7px; padding:9px; }
    .plugin-subagent-settings button { justify-self:start; }
    .plugin-subagent-settings-error { color:var(--wuu-color-danger); font-size:12px; }
    .plugin-subagent-ultra { display:inline-grid; place-items:center; width:28px; height:28px; padding:0; border:1px solid transparent; border-radius:8px; background:transparent; color:var(--wuu-color-text-muted); cursor:pointer; font:600 11px/1 var(--wuu-font-family-ui); }
    .plugin-subagent-ultra:hover { background:var(--wuu-color-surface-muted); color:var(--wuu-color-text); }
    .plugin-subagent-ultra[aria-pressed="true"] { border-color:var(--wuu-color-accent); color:var(--wuu-color-accent); background:var(--wuu-color-surface-muted); }
    .plugin-subagent-ultra:disabled { opacity:.45; cursor:default; }
  ` });
  function ChildTaskStatus(props) {
    const threadId = typeof props.threadId === "string" ? props.threadId : "";
    const [tasks, setTasks] = React.useState([]);
    const refresh = React.useCallback(() => {
      if (!threadId) { setTasks([]); return Promise.resolve(); }
      return api.invokeRuntime("status.list", { parent_session_id: threadId })
		.then((value) => setTasks(Array.isArray(value?.sessions) ? value.sessions : []))
		.catch(() => setTasks([]));
    }, [threadId]);
    React.useEffect(() => { void refresh(); }, [refresh]);
    React.useEffect(() => {
      const subscription = api.onHostEvent((event) => {
        const method = event && typeof event === "object" && event.message && typeof event.message === "object" ? event.message.method : "";
        if (typeof method === "string" && (method.startsWith("turn/") || method.startsWith("thread/") || method.startsWith("agent/"))) void refresh();
      });
      return () => subscription.dispose();
    }, [refresh]);
    const visible = tasks.filter((task) => task && task.state && task.state !== "completed");
    if (!props.mainConversation || visible.length === 0) return null;
    return h("div", { className: "plugin-subagent-status" }, visible.map((task) => h("span", {
      className: "plugin-subagent-chip", "data-status": task.state, key: task.session_id,
    }, `${task.name || "Child task"} · ${task.state}`)));
  }
  function ModelAliases({ context }) {
    const active = context && context.activePage === "advanced";
    const aliases = context && context.modelAliases ? context.modelAliases : {};
    const [draft, setDraft] = React.useState(() => JSON.stringify(aliases, null, 2));
    const [error, setError] = React.useState("");
    React.useEffect(() => setDraft(JSON.stringify(aliases, null, 2)), [JSON.stringify(aliases)]);
    if (!active) return null;
    const save = async () => {
      try {
        const value = JSON.parse(draft);
        if (!value || Array.isArray(value) || typeof value !== "object") throw new Error("Aliases must be a JSON object.");
        if (typeof context.onSaveModelAliases !== "function") throw new Error("Settings update is unavailable.");
        await context.onSaveModelAliases(value);
        setError("");
      } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)); }
    };
    return h("section", { className: "plugin-subagent-settings" },
      h("strong", null, "Subagent model aliases"),
      h("textarea", { value: draft, disabled: Boolean(context.busy), onChange: (event) => setDraft(event.target.value), "aria-label": "Subagent model aliases JSON" }),
      error ? h("div", { className: "plugin-subagent-settings-error" }, error) : null,
      h("button", { type: "button", disabled: Boolean(context.busy), onClick: save }, "Save aliases"));
  }
  function ProactiveDelegation(props) {
    const translate = typeof props.translate === "function" ? props.translate : (key) => key;
    const [enabled, setEnabled] = React.useState(false);
    const [busy, setBusy] = React.useState(true);
    React.useEffect(() => {
      let active = true;
      api.invokeRuntime("ultra.get", {}).then((value) => {
        if (active) setEnabled(Boolean(value?.enabled));
      }).finally(() => { if (active) setBusy(false); });
      return () => { active = false; };
    }, []);
    const labelKey = enabled ? "subagent.ultra.disable" : "subagent.ultra.enable";
    const label = translate(labelKey);
    const toggle = async () => {
      const next = !enabled;
      setBusy(true);
      try {
        const value = await api.invokeRuntime("ultra.update", { enabled: next });
        setEnabled(Boolean(value?.enabled));
      } finally { setBusy(false); }
    };
    return h("button", { type:"button", className:"plugin-subagent-ultra", disabled:busy, "aria-label":label, title:label, "aria-pressed":enabled, onClick:()=>void toggle() }, "A+");
  }
  api.registerSlot("composer.above", { id: "subagent-status", order: 30, render: (context) => h(ChildTaskStatus, context) });
  api.registerSlot("composer.toolbar", { id: "subagent-ultra", order: 30, render: (context) => h(ProactiveDelegation, context) });
  api.registerSlot("settings.plugin", { id: "subagent-settings", order: 30, render: (context) => h(ModelAliases, { context }) });
}
