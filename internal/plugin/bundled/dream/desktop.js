export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Page, Section, Stack, Row, Button, Checkbox } = api.ui;
  api.registerLocale({ id: "dream-en", locale: "en-US", entries: {
    "dream.description": "Consolidate completed sessions into durable workspace memory in the background.",
    "dream.settings": "Background consolidation",
    "dream.enabled": "Enable background consolidation", "dream.enabledHelp": "Dream runs only while Wuu is open.",
    "dream.interval": "Interval (days)", "dream.intervalHelp": "Days between automatic runs.",
    "dream.minimum": "Completed sessions before a run", "dream.minimumHelp": "A run starts only after this many sessions completed.",
    "dream.model": "Model alias (optional)", "dream.modelHelp": "Overrides the default model used for consolidation.",
    "dream.save": "Save changes", "dream.saved": "Saved", "dream.run": "Run now", "dream.activity": "Activity",
    "dream.candidates": "Pending sessions", "dream.status": "Last status",
    "dream.status.running": "Running", "dream.status.completed": "Completed",
    "dream.status.failed": "Failed", "dream.status.skipped": "Skipped"
  }});
  api.registerLocale({ id: "dream-zh", locale: "zh-CN", entries: {
    "dream.description": "在后台把已完成会话整合为工作区长期记忆。",
    "dream.settings": "后台整合",
    "dream.enabled": "启用后台记忆整合", "dream.enabledHelp": "仅在 Wuu 运行期间执行。",
    "dream.interval": "运行间隔（天）", "dream.intervalHelp": "每隔多少天自动运行一次。",
    "dream.minimum": "触发前累计完成会话数", "dream.minimumHelp": "累计完成会话达到该数量后才运行。",
    "dream.model": "模型别名（可选）", "dream.modelHelp": "不填时使用默认模型进行整合。",
    "dream.save": "保存更改", "dream.saved": "已保存", "dream.run": "立即运行", "dream.activity": "运行情况",
    "dream.candidates": "待整理会话", "dream.status": "上次状态",
    "dream.status.running": "运行中", "dream.status.completed": "已完成",
    "dream.status.failed": "运行失败", "dream.status.skipped": "已跳过"
  }});
  api.registerStyle({ id: "dream-settings", css: `
    .plugin-dream { min-width:0; }
    .plugin-dream-intro { margin:0; max-width:52ch; color:var(--wuu-color-text-muted,var(--ink-soft)); font-size:var(--font-ui,13px); line-height:1.5; }

    /* Form rows on the canvas: label left, compact control pinned right,
     * full-width hairline separators — the native settings row grammar. */
    .plugin-dream-rows { padding:0 16px; border:1px solid var(--hairline); border-radius:var(--session-composer-radius); display:flex; flex-direction:column; }
    .plugin-dream-row { display:grid; grid-template-columns:minmax(0,1fr) auto; align-items:center; gap:24px; padding:16px 0; border-bottom:1px solid var(--hairline-soft,var(--hairline)); }
    .plugin-dream-rows > .plugin-ui-checkbox { padding:16px 0; border-bottom:1px solid var(--hairline-soft); }
    .plugin-dream-row:last-child { border-bottom:0; }
    .plugin-dream-row-copy { display:flex; flex-direction:column; gap:2px; min-width:0; }
    .plugin-dream-row-title { color:var(--wuu-color-text,var(--ink)); font-size:var(--font-ui,13px); font-weight:var(--weight-medium,500); line-height:1.4; }
    .plugin-dream-row-help { color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:var(--font-sm,12px); line-height:1.5; }
    .plugin-dream-num { max-width:100%; width:104px; text-align:right; font-variant-numeric:tabular-nums; }
    .plugin-dream-model { width:200px; max-width:100%; }

    .plugin-dream-actions { justify-content:flex-start; align-items:center; gap:8px; }
    .plugin-dream-saved { color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:var(--font-sm,12px); animation:plugin-dream-fade 2.4s ease both; }
    @keyframes plugin-dream-fade { 0% { opacity:0; } 12% { opacity:1; } 70% { opacity:1; } 100% { opacity:0; } }

    .plugin-dream-stats { padding:0 16px; border:1px solid var(--hairline); border-radius:var(--session-composer-radius); display:flex; flex-direction:column; }
    .plugin-dream-stat { display:grid; grid-template-columns:minmax(0,1fr) minmax(0,1fr); align-items:start; gap:24px; padding:16px 0; border-bottom:1px solid var(--hairline-soft,var(--hairline)); }
    .plugin-dream-stat:last-child { border-bottom:0; }
    .plugin-dream-stat-label { color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:var(--font-ui,13px); }
    .plugin-dream-stat-value { overflow-wrap:anywhere; color:var(--wuu-color-text,var(--ink)); font-size:var(--font-ui,13px); font-weight:var(--weight-medium,500); font-variant-numeric:tabular-nums; text-align:right; }
    .plugin-dream-status-dot { display:inline-block; width:7px; height:7px; margin-right:6px; border-radius:50%; background:var(--wuu-color-text-muted,var(--ink-muted)); vertical-align:1px; }
    .plugin-dream-status-dot[data-tone="ok"] { background:var(--wuu-color-success,var(--success,#4cc38a)); }
    .plugin-dream-status-dot[data-tone="bad"] { background:var(--wuu-color-danger,var(--danger)); }
    .plugin-dream-status-dot[data-tone="busy"] { background:var(--wuu-color-warning,var(--warning,#d9a84e)); }

    .plugin-dream-error { color:var(--wuu-color-danger,var(--danger)); font-size:var(--font-ui,13px); }
    .plugin-dream-loading { color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:var(--font-ui,13px); }
    @container settings-page (max-width:480px) {
      .plugin-dream-row { grid-template-columns:minmax(0,1fr); gap:12px; }
      .plugin-dream-num, .plugin-dream-model { width:100%; text-align:left; }
      .plugin-dream-stat { grid-template-columns:minmax(0,1fr); gap:6px; }
      .plugin-dream-stat-value { overflow-wrap:anywhere; text-align:left; }
    }
  `});
  function DreamSettings(props) {
    const tr = props.translate;
    const [state, setState] = React.useState(null);
    const [draft, setDraft] = React.useState(null);
    const [busy, setBusy] = React.useState(false);
    const [savedTick, setSavedTick] = React.useState(0);
    const [error, setError] = React.useState("");
    const refresh = React.useCallback(async () => {
      const value = await api.invokeRuntime("dream.get", {});
      setState(value);
      setDraft(value.settings);
    }, []);
    React.useEffect(() => { void refresh().catch((reason) => setError(String(reason))); }, [refresh]);
    const act = async (method, input, { markSaved = false } = {}) => {
      setBusy(true); setError("");
      try {
        const value = await api.invokeRuntime(method, input);
        setState(value);
        setDraft(value.settings);
        if (markSaved) setSavedTick((tick) => tick + 1);
      } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
    };
    if (!draft) return h(Page, { className: "plugin-dream" }, h("div", { className: error ? "plugin-dream-error" : "plugin-dream-loading", role: error ? "alert" : "status" }, error || "…"));
    const number = (event) => Number.parseInt(event.target.value, 10) || 0;
    const candidateCount = Object.keys(state?.candidates || {}).length;
    const status = state?.last_status || "—";
    const statusLabel = ["running", "completed", "failed", "skipped"].includes(status) ? tr(`dream.status.${status}`) : status;
    const statusTone = state?.last_error ? "bad" : status === "running" ? "busy" : status === "completed" ? "ok" : "";
    return h(Page, { className: "plugin-dream" }, h(Stack, { gap: "large" },
      h("p", { className: "plugin-dream-intro" }, tr("dream.description")),
      h(Section, { title: tr("dream.settings") }, h("div", { className: "plugin-dream-rows" },
        h(Checkbox, { label: tr("dream.enabled"), description: tr("dream.enabledHelp"), checked: draft.enabled, disabled: busy, onChange: (event) => setDraft({ ...draft, enabled: event.target.checked }) }),
        h("label", { className: "plugin-dream-row" },
          h("span", { className: "plugin-dream-row-copy" },
            h("span", { className: "plugin-dream-row-title" }, tr("dream.interval")),
            h("span", { className: "plugin-dream-row-help" }, tr("dream.intervalHelp"))),
          h("input", { className: "plugin-ui-input plugin-dream-num", type: "number", min: 1, max: 365, value: draft.interval_days, disabled: busy, onChange: (event) => setDraft({ ...draft, interval_days: number(event) }) })),
        h("label", { className: "plugin-dream-row" },
          h("span", { className: "plugin-dream-row-copy" },
            h("span", { className: "plugin-dream-row-title" }, tr("dream.minimum")),
            h("span", { className: "plugin-dream-row-help" }, tr("dream.minimumHelp"))),
          h("input", { className: "plugin-ui-input plugin-dream-num", type: "number", min: 1, max: 100, value: draft.min_sessions, disabled: busy, onChange: (event) => setDraft({ ...draft, min_sessions: number(event) }) })),
        h("label", { className: "plugin-dream-row" },
          h("span", { className: "plugin-dream-row-copy" },
            h("span", { className: "plugin-dream-row-title" }, tr("dream.model")),
            h("span", { className: "plugin-dream-row-help" }, tr("dream.modelHelp"))),
          h("input", { className: "plugin-ui-input plugin-dream-model", type: "text", value: draft.model_alias || "", disabled: busy, onChange: (event) => setDraft({ ...draft, model_alias: event.target.value }) })))),
      h(Row, { className: "plugin-dream-actions" },
        h(Button, { variant: "primary", disabled: busy, onClick: () => void act("dream.update", draft, { markSaved: true }) }, tr("dream.save")),
        h(Button, { disabled: busy || !draft.enabled || candidateCount === 0, onClick: () => void act("dream.run", {}) }, tr("dream.run")),
        savedTick ? h("span", { key: savedTick, className: "plugin-dream-saved" }, tr("dream.saved")) : null,
        error ? h("span", { className: "plugin-dream-error", role: "alert" }, error) : null),
      h(Section, { title: tr("dream.activity") }, h("div", { className: "plugin-dream-stats" },
        h("div", { className: "plugin-dream-stat" },
          h("span", { className: "plugin-dream-stat-label" }, tr("dream.candidates")),
          h("span", { className: "plugin-dream-stat-value" }, String(candidateCount))),
        h("div", { className: "plugin-dream-stat" },
          h("span", { className: "plugin-dream-stat-label" }, tr("dream.status")),
          h("span", { className: "plugin-dream-stat-value" },
            h("span", { className: "plugin-dream-status-dot", "data-tone": statusTone, "aria-hidden": true }),
            `${statusLabel}${state?.last_error ? ` · ${state.last_error}` : ""}`))))
    ));
  }
  api.registerViewType({ id: "dream.settings", title: "Dream", icon: "moon", defaultRegion: "settings", persistence: "durable", render: (props) => h(DreamSettings, props) });
}
