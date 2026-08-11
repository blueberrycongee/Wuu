export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  api.registerLocale({ id: "subagent-en", locale: "en-US", entries: {
    "subagent.ultra.enable": "Enable proactive delegation",
    "subagent.ultra.disable": "Disable proactive delegation",
    "subagent.aliases.intro": "Route delegated child tasks to specific models by naming aliases here.",
    "subagent.aliases.title": "Model aliases",
    "subagent.aliases.help": "A JSON object mapping alias names to model identifiers, applied to newly spawned child tasks.",
    "subagent.aliases.label": "Aliases (JSON)",
    "subagent.aliases.save": "Save aliases",
    "subagent.aliases.saved": "Saved",
    "subagent.aliases.invalid": "Aliases must be a JSON object."
  } });
  api.registerLocale({ id: "subagent-zh", locale: "zh-CN", entries: {
    "subagent.ultra.enable": "开启主动委派",
    "subagent.ultra.disable": "关闭主动委派",
    "subagent.aliases.intro": "通过别名把委派的子任务路由到指定模型。",
    "subagent.aliases.title": "模型别名",
    "subagent.aliases.help": "一个 JSON 对象，把别名映射到模型标识，对之后创建的子任务生效。",
    "subagent.aliases.label": "别名配置（JSON）",
    "subagent.aliases.save": "保存别名",
    "subagent.aliases.saved": "已保存",
    "subagent.aliases.invalid": "别名配置必须是 JSON 对象。"
  } });
  api.registerStyle({ id: "subagent-status", css: `
    .plugin-subagent-status { display:flex; gap:6px; flex-wrap:wrap; margin:0 2px 8px; }
    .plugin-subagent-chip { padding:3px 7px; border:1px solid var(--wuu-color-border-subtle); border-radius:var(--wuu-radius-control, 999px); background:var(--wuu-color-surface-muted); color:var(--wuu-color-text-muted); font-size:11px; }
    .plugin-subagent-chip[data-status="running"] { color:var(--wuu-color-text); }
    .plugin-subagent-settings { min-width:0; }
    .plugin-subagent-intro { margin:0; max-width:52ch; color:var(--wuu-color-text-muted,var(--ink-soft)); font-size:var(--font-ui,13px); line-height:1.5; }
    .plugin-subagent-field textarea { width:100%; min-height:180px; resize:vertical; font:12px/1.6 var(--wuu-font-mono,ui-monospace,monospace); font-variant-numeric:tabular-nums; }
    .plugin-subagent-actions { align-items:center; gap:8px; }
    .plugin-subagent-saved { color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:var(--font-sm,12px); animation:plugin-subagent-fade 2.4s ease both; }
    @keyframes plugin-subagent-fade { 0% { opacity:0; } 12% { opacity:1; } 70% { opacity:1; } 100% { opacity:0; } }
    .plugin-subagent-settings-error { color:var(--wuu-color-danger); font-size:var(--font-ui,13px); overflow-wrap:anywhere; }
    .plugin-subagent-ultra svg { width:18px; height:18px; fill:none; stroke:currentColor; stroke-width:1.65; stroke-linecap:round; stroke-linejoin:round; }
    .plugin-subagent-ultra .plugin-subagent-delegation-node { transition:fill var(--wuu-motion-duration-fast) var(--wuu-motion-easing-standard); }
    .plugin-subagent-ultra[aria-pressed="true"] .plugin-subagent-delegation-node { fill:currentColor; }
  ` });
  function ChildTaskStatus(props) {
    const threadId = typeof props.threadId === "string" ? props.threadId : "";
    const [taskState, setTaskState] = React.useState({ threadId: "", tasks: [] });
    const refreshGeneration = React.useRef(0);
    const refresh = React.useCallback(() => {
      const generation = ++refreshGeneration.current;
      if (!threadId) { setTaskState({ threadId, tasks: [] }); return Promise.resolve(); }
      return api.invokeRuntime("status.list", { parent_session_id: threadId })
		.then((value) => {
		  if (refreshGeneration.current === generation) setTaskState({ threadId, tasks: Array.isArray(value?.sessions) ? value.sessions : [] });
		})
		.catch(() => {
		  if (refreshGeneration.current === generation) setTaskState({ threadId, tasks: [] });
		});
    }, [threadId]);
    React.useEffect(() => {
      void refresh();
      return () => { refreshGeneration.current += 1; };
    }, [refresh]);
    React.useEffect(() => {
      const subscription = api.onHostEvent((event) => {
        const method = event && typeof event === "object" && event.message && typeof event.message === "object" ? event.message.method : "";
        if (typeof method === "string" && [
          "thread/started",
          "thread/resumed",
          "thread/updated",
          "turn/started",
          "turn/queued",
          "turn/dequeued",
          "turn/held",
          "turn/completed",
          "turn/error",
        ].includes(method)) void refresh();
      });
      return () => subscription.dispose();
    }, [refresh]);
    const tasks = taskState.threadId === threadId ? taskState.tasks : [];
    const visible = tasks.filter((task) => task && task.state && task.state !== "completed");
    if (!props.mainConversation || visible.length === 0) return null;
    return h("div", { className: "plugin-subagent-status" }, visible.map((task) => h("span", {
      className: "plugin-subagent-chip", "data-status": task.state, key: task.session_id,
    }, `${task.name || "Child task"} · ${task.state}`)));
  }
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
  function ProactiveDelegationIcon() {
    return h("svg", { viewBox:"0 0 20 20", "aria-hidden":"true" },
      h("circle", { className:"plugin-subagent-delegation-node", cx:"7", cy:"4", r:"2" }),
      h("path", { d:"M7 6v2.5M3.5 11V9.5h7V11" }),
      h("circle", { className:"plugin-subagent-delegation-node", cx:"3.5", cy:"13.5", r:"2" }),
      h("circle", { className:"plugin-subagent-delegation-node", cx:"10.5", cy:"13.5", r:"2" }),
      h("path", { d:"M15.5 3v5M13 5.5h5" })
    );
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
    return h(api.ui.ToolbarToggle, { pressed:enabled, className:"plugin-subagent-ultra", disabled:busy, "aria-label":label, title:label, onClick:()=>void toggle() }, h(ProactiveDelegationIcon));
  }
  api.registerSlot("composer.above", { id: "subagent-status", order: 30, render: (context) => h(ChildTaskStatus, context) });
  api.registerSlot("composer.toolbar", { id: "subagent-ultra", order: 30, render: (context) => h(ProactiveDelegation, context) });
  api.registerViewType({ id: "subagent.settings", title: "Subagent", icon: "bot", defaultRegion: "settings", persistence: "durable", render: ModelAliases });
}
