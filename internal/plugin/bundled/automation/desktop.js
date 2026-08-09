export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Page, Panel, Card, Section, Stack, Row, Button, TextInput, TextArea, Checkbox, EmptyState } = api.ui;

  api.registerLocale({ id: "automation-en", locale: "en-US", entries: {
    "automation.title": "Automations", "automation.new": "New automation", "automation.empty": "No automations yet",
    "automation.emptyHelp": "Scheduled prompts that wake the Agent will show up here.",
    "automation.name": "Name", "automation.prompt": "Prompt", "automation.schedule": "Cron schedule", "automation.timezone": "Timezone",
    "automation.description": "Wake the main agent on a schedule with a saved prompt.",
    "automation.recurring": "Repeat", "automation.recurringHelp": "Keep scheduling future runs after this automation fires.",
    "automation.create": "Create automation", "automation.cancel": "Cancel", "automation.pause": "Pause", "automation.resume": "Resume", "automation.remove": "Remove",
    "automation.paused": "Paused"
  }});
  api.registerLocale({ id: "automation-zh", locale: "zh-CN", entries: {
    "automation.title": "自动化", "automation.new": "新建自动化", "automation.empty": "还没有自动化任务",
    "automation.emptyHelp": "按时间唤醒 Agent 的提示词会显示在这里。",
    "automation.name": "名称", "automation.prompt": "提示词", "automation.schedule": "Cron 时间", "automation.timezone": "时区",
    "automation.description": "按设定时间用保存的提示词唤醒主 Agent。",
    "automation.recurring": "重复执行", "automation.recurringHelp": "任务触发后继续安排下一次运行。",
    "automation.create": "创建自动化", "automation.cancel": "取消", "automation.pause": "暂停", "automation.resume": "继续", "automation.remove": "删除",
    "automation.paused": "已暂停"
  }});
  api.registerStyle({ id: "automation-catalog", css: `
    .plugin-automation { height:100%; overflow:auto; container-type:inline-size; color:var(--wuu-color-text, var(--ink, #181818)); background:var(--wuu-color-canvas, var(--paper, #fff)); }
    .plugin-automation-head { align-items:flex-start; gap:16px; }
    .plugin-automation-head-copy { display:flex; flex-direction:column; gap:4px; flex:1; min-width:0; }
    .plugin-automation-title { margin:0; color:var(--wuu-color-text-strong, var(--ink-strong, var(--ink))); font-size:var(--font-heading,18px); font-weight:var(--weight-semibold,600); letter-spacing:-0.01em; line-height:1.3; }
    .plugin-automation-intro { margin:0; max-width:56ch; color:var(--wuu-color-text-muted, var(--ink-muted, #666)); font-size:var(--font-ui,13px); line-height:1.5; }
    .plugin-automation-new { flex:none; }

    /* The create form is the one enclosed surface on the page: a quiet
     * hairline panel so the in-progress draft reads as a temporary zone. */
    .plugin-automation-form { display:flex; flex-direction:column; gap:14px; }
    .plugin-automation-form-grid { display:grid; grid-template-columns:1fr 1fr; gap:14px; }
    @container (max-width: 560px) { .plugin-automation-form-grid { grid-template-columns:1fr; } }
    .plugin-automation-form .plugin-ui-checkbox { padding:2px 0; }
    .plugin-automation-form-actions { justify-content:flex-end; gap:8px; }

    .plugin-automation-list { display:grid; gap:10px; }
    .plugin-automation-card { display:flex; flex-direction:column; gap:8px; transition:border-color var(--motion-fast,120ms) ease; }
    .plugin-automation-card:hover { border-color:var(--wuu-color-border-strong, var(--hairline-strong)); }
    .plugin-automation-card[data-paused="true"] { opacity:0.72; }
    .plugin-automation-card-head { display:flex; align-items:center; gap:8px; min-width:0; }
    .plugin-automation-card-title { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--wuu-color-text, var(--ink)); font-size:var(--font-ui,13px); font-weight:var(--weight-semibold,600); }
    .plugin-automation-badge { flex:none; padding:1px 8px; border-radius:var(--radius-pill,999px); background:var(--wuu-color-surface-muted, var(--surface-2)); color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:11px; font-weight:var(--weight-medium,500); line-height:1.7; }
    .plugin-automation-card-head .plugin-ui-button { min-height:26px; padding:0 8px; font-size:var(--font-sm,12px); }
    .plugin-automation-meta { display:flex; flex-wrap:wrap; gap:6px 10px; color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:var(--font-sm,12px); }
    .plugin-automation-meta code { padding:1px 6px; border-radius:6px; background:var(--wuu-color-surface-muted, var(--surface-2)); color:var(--wuu-color-text, var(--ink-soft)); font:11px/1.6 var(--wuu-font-mono, ui-monospace, monospace); font-variant-numeric:tabular-nums; }
    .plugin-automation-prompt { margin:0; display:-webkit-box; -webkit-box-orient:vertical; -webkit-line-clamp:2; overflow:hidden; color:var(--wuu-color-text-muted, var(--ink-soft)); font-size:var(--font-ui,13px); line-height:1.55; white-space:pre-wrap; }
    .plugin-automation-error { color:var(--wuu-color-danger, #b42318); font-size:var(--font-ui,13px); }
  ` });

  function Catalog(props) {
    const tr = props.translate;
    const [tasks, setTasks] = React.useState([]);
    const [creating, setCreating] = React.useState(false);
    const [busy, setBusy] = React.useState(false);
    const [error, setError] = React.useState("");
    const [draft, setDraft] = React.useState({ title: "", prompt: "", schedule: "0 9 * * 1-5", timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC", mode: "new_thread", recurring: true });
    const refresh = React.useCallback(() => api.invokeRuntime("automation.list", {}).then((value) => setTasks(Array.isArray(value?.tasks) ? value.tasks : [])).catch((reason) => setError(String(reason))), []);
    React.useEffect(() => { void refresh(); }, [refresh]);
    const act = async (method, input) => { setBusy(true); setError(""); try { await api.invokeRuntime(method, input); await refresh(); } catch (reason) { setError(String(reason)); } finally { setBusy(false); } };
    return h("main", { className: "plugin-automation" }, h(Page, null, h(Stack, { gap: "large" },
      h(Row, { className: "plugin-automation-head" },
        h("div", { className: "plugin-automation-head-copy" },
          h("h1", { className: "plugin-automation-title" }, tr("automation.title")),
          h("p", { className: "plugin-automation-intro" }, tr("automation.description"))),
        h(Button, { className: "plugin-automation-new", variant: creating ? "ghost" : "primary", onClick: () => setCreating(!creating) }, tr(creating ? "automation.cancel" : "automation.new"))),
      creating ? h(Panel, null, h("div", { className: "plugin-automation-form" },
        h(TextInput, { label: tr("automation.name"), value: draft.title, onChange: (event) => setDraft({ ...draft, title: event.target.value }) }),
        h(TextArea, { label: tr("automation.prompt"), value: draft.prompt, onChange: (event) => setDraft({ ...draft, prompt: event.target.value }) }),
        h("div", { className: "plugin-automation-form-grid" },
          h(TextInput, { label: tr("automation.schedule"), value: draft.schedule, onChange: (event) => setDraft({ ...draft, schedule: event.target.value }) }),
          h(TextInput, { label: tr("automation.timezone"), value: draft.timezone, onChange: (event) => setDraft({ ...draft, timezone: event.target.value }) })),
        h(Checkbox, { label: tr("automation.recurring"), description: tr("automation.recurringHelp"), checked: draft.recurring, onChange: (event) => setDraft({ ...draft, recurring: event.target.checked }) }),
        h(Row, { className: "plugin-automation-form-actions" },
          h(Button, { variant: "ghost", onClick: () => setCreating(false) }, tr("automation.cancel")),
          h(Button, { variant: "primary", disabled: busy || !draft.prompt.trim(), onClick: async () => { await act("automation.create", { ...draft, durable: true }); setCreating(false); } }, tr("automation.create"))))) : null,
      error ? h("div", { className: "plugin-automation-error", role: "alert" }, error) : null,
      tasks.length === 0
        ? h(EmptyState, { title: tr("automation.empty"), description: tr("automation.emptyHelp") })
        : h(Section, { className: "plugin-automation-list" }, tasks.map((task) => h(Card, { className: "plugin-automation-card", key: task.id, "data-paused": task.paused ? "true" : "false" },
          h("div", { className: "plugin-automation-card-head" },
            h("span", { className: "plugin-automation-card-title" }, task.title || task.prompt),
            task.paused ? h("span", { className: "plugin-automation-badge" }, tr("automation.paused")) : null,
            h(Button, { variant: "ghost", disabled: busy, onClick: () => act("automation.update", { ...task, paused: !task.paused }) }, tr(task.paused ? "automation.resume" : "automation.pause")),
            h(Button, { variant: "danger", disabled: busy, onClick: () => act("automation.remove", { id: task.id }) }, tr("automation.remove"))),
          h("div", { className: "plugin-automation-meta" },
            h("code", null, task.cron),
            h("span", null, `${task.timezone} · ${task.mode}`)),
          h("p", { className: "plugin-automation-prompt" }, task.prompt))))
    )));
  }

  api.registerViewType({ id: "automation.catalog", title: "Automations", icon: "clock", defaultRegion: "primary", persistence: "durable", render: (props) => h(Catalog, props) });
}
