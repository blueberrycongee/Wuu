export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { LiveDuration } = api.ui;

  api.registerLocale({ id: "goal-en", locale: "en-US", entries: {
    "goal.pause": "Pause goal", "goal.resume": "Resume goal", "goal.edit": "Edit goal",
    "goal.clear": "Clear goal", "goal.confirmClear": "Confirm clear", "goal.more": "More goal actions",
    "goal.running": "Running", "goal.ready": "Ready", "goal.paused": "Paused",
    "goal.blocked": "Blocked", "goal.complete": "Complete", "goal.elapsed": "Elapsed time",
    "goal.save": "Save", "goal.cancel": "Cancel"
  }});
  api.registerLocale({ id: "goal-zh", locale: "zh-CN", entries: {
    "goal.pause": "暂停目标", "goal.resume": "继续目标", "goal.edit": "编辑目标",
    "goal.clear": "清除目标", "goal.confirmClear": "确认清除", "goal.more": "更多目标操作",
    "goal.running": "运行中", "goal.ready": "等待继续", "goal.paused": "已暂停",
    "goal.blocked": "已阻塞", "goal.complete": "已完成", "goal.elapsed": "累计运行时间",
    "goal.save": "保存", "goal.cancel": "取消"
  }});
  api.registerStyle({ id: "goal-strip", css: `
    .plugin-goal-strip { position:relative; display:flex; flex-direction:column; margin:0 2px 8px; min-width:0; }
    .plugin-goal-row { display:flex; align-items:center; min-height:38px; gap:9px; min-width:0; padding:0 8px 0 10px;
      border:1px solid var(--wuu-color-border-subtle); border-radius:10px; background:color-mix(in srgb, var(--wuu-color-surface-muted) 72%, transparent); }
    .plugin-goal-state { display:inline-grid; place-items:center; width:18px; height:18px; flex:none; color:var(--wuu-color-text-muted); }
    .plugin-goal-state[data-state="running"] { color:var(--wuu-color-accent); }
    .plugin-goal-state[data-state="blocked"] { color:var(--wuu-color-danger, #c44); }
    .plugin-goal-state[data-state="complete"] { color:var(--wuu-color-success, #368a55); }
    .plugin-goal-spinner { width:12px; height:12px; border:1.5px solid color-mix(in srgb, currentColor 28%, transparent); border-top-color:currentColor; border-radius:50%; animation:plugin-goal-spin .9s linear infinite; }
    @keyframes plugin-goal-spin { to { transform:rotate(360deg); } }
    .plugin-goal-icon { width:15px; height:15px; display:block; }
    .plugin-goal-text { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--wuu-color-text); font-size:12px; font-weight:500; }
    .plugin-goal-time { flex:none; color:var(--wuu-color-text-muted); font:500 11px/1.2 var(--wuu-font-family-ui); font-variant-numeric:tabular-nums; }
    .plugin-goal-controls { display:flex; align-items:center; gap:2px; flex:none; }
    .plugin-goal-icon-button { display:inline-grid; place-items:center; width:28px; height:28px; padding:0; border:0; border-radius:7px; background:transparent; color:var(--wuu-color-text-muted); cursor:pointer; }
    .plugin-goal-icon-button:hover, .plugin-goal-icon-button[aria-expanded="true"] { color:var(--wuu-color-text); background:var(--wuu-color-surface-elevated); }
    .plugin-goal-icon-button:disabled { opacity:.42; cursor:default; }
    .plugin-goal-menu { position:absolute; z-index:20; right:4px; bottom:42px; display:flex; flex-direction:column; min-width:132px; padding:5px;
      border:1px solid var(--wuu-color-border-subtle); border-radius:9px; background:var(--wuu-color-surface); box-shadow:0 10px 28px rgba(20,24,28,.14); }
    .plugin-goal-menu-button { display:flex; align-items:center; gap:8px; width:100%; padding:7px 9px; border:0; border-radius:6px; background:transparent; color:var(--wuu-color-text); font:inherit; font-size:12px; text-align:left; cursor:pointer; }
    .plugin-goal-menu-button:hover { background:var(--wuu-color-surface-muted); }
    .plugin-goal-menu-button.danger { color:var(--wuu-color-danger, #c44); }
    .plugin-goal-menu-button:disabled { opacity:.42; cursor:default; }
    .plugin-goal-editor { display:flex; gap:6px; align-items:flex-end; padding:8px; border:1px solid var(--wuu-color-border-subtle); border-radius:10px; background:var(--wuu-color-surface-muted); }
    .plugin-goal-editor textarea { flex:1; resize:vertical; min-height:42px; max-height:120px; padding:6px 8px; border:1px solid var(--wuu-color-border-subtle); border-radius:7px; background:var(--wuu-color-surface); color:var(--wuu-color-text); font:12px/1.4 var(--wuu-font-family-ui); }
    .plugin-goal-editor-actions { display:flex; gap:3px; }
    .plugin-goal-text-button { border:0; padding:5px 7px; border-radius:6px; background:transparent; color:var(--wuu-color-text-muted); font:inherit; font-size:11px; cursor:pointer; }
    .plugin-goal-text-button:hover { background:var(--wuu-color-surface-elevated); color:var(--wuu-color-text); }
    .plugin-goal-text-button:disabled { opacity:.42; cursor:default; }
    .plugin-goal-error { padding:5px 8px 0; font-size:11px; color:var(--wuu-color-danger, #c44); }
    @media (prefers-reduced-motion: reduce) { .plugin-goal-spinner { animation:none; } }
  ` });

  function Icon({ name }) {
    const common = { className: "plugin-goal-icon", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: 1.8, strokeLinecap: "round", strokeLinejoin: "round", "aria-hidden": true };
    if (name === "play") return h("svg", common, h("path", { d: "m8 5 11 7-11 7Z", fill: "currentColor", stroke: "none" }));
    if (name === "pause") return h("svg", common, h("path", { d: "M9 5v14M15 5v14" }));
    if (name === "check") return h("svg", common, h("path", { d: "m5 12 4 4L19 6" }));
    if (name === "alert") return h("svg", common, h("path", { d: "M12 3 2.8 19h18.4L12 3Z" }), h("path", { d: "M12 9v4M12 16.5h.01" }));
    if (name === "edit") return h("svg", common, h("path", { d: "M4 20h4L19 9l-4-4L4 16v4ZM13.5 6.5l4 4" }));
    if (name === "trash") return h("svg", common, h("path", { d: "M4 7h16M9 7V4h6v3M7 7l1 13h8l1-13M10 11v5M14 11v5" }));
    if (name === "more") return h("svg", { ...common, fill: "currentColor", stroke: "none" }, h("circle", { cx: 5, cy: 12, r: 1.5 }), h("circle", { cx: 12, cy: 12, r: 1.5 }), h("circle", { cx: 19, cy: 12, r: 1.5 }));
    return h("svg", common, h("circle", { cx: 12, cy: 12, r: 7 }), h("circle", { cx: 12, cy: 12, r: 2 }));
  }

  function GoalStrip(props) {
    const threadId = typeof props.threadId === "string" ? props.threadId : "";
    const translate = typeof props.translate === "function" ? props.translate : (key) => key;
    const [goal, setGoal] = React.useState(null);
    const [editing, setEditing] = React.useState(false);
    const [draft, setDraft] = React.useState("");
    const [busy, setBusy] = React.useState(false);
    const [menuOpen, setMenuOpen] = React.useState(false);
    const [confirmClear, setConfirmClear] = React.useState(false);
    const [error, setError] = React.useState("");
    const menuRef = React.useRef(null);
    const optimisticStartRef = React.useRef(0);

    const tr = React.useCallback((key) => {
      const value = translate(key);
      return typeof value === "string" && value !== key ? value : key.split(".").pop();
    }, [translate]);
    const refresh = React.useCallback(() => {
      if (!threadId) { setGoal(null); return Promise.resolve(); }
      return api.invokeRuntime("summary.get", { thread_id: threadId })
        .then((value) => setGoal(value && typeof value === "object" ? value.goal ?? null : null))
        .catch((reason) => setError(String(reason)));
    }, [threadId]);

    React.useEffect(() => { void refresh(); }, [refresh]);
    React.useEffect(() => {
      const subscription = api.onHostEvent((event) => {
        const method = event && typeof event === "object" && event.message && typeof event.message === "object" ? event.message.method : "";
        if (typeof method === "string" && (method.startsWith("turn/") || method.startsWith("thread/"))) void refresh();
      });
      return () => subscription.dispose();
    }, [refresh]);
    React.useEffect(() => {
      setEditing(false); setMenuOpen(false); setConfirmClear(false); setError(""); optimisticStartRef.current = 0;
    }, [threadId, goal?.updated_at]);
    React.useEffect(() => {
      if (!menuOpen) return undefined;
      const close = (event) => { if (menuRef.current && !menuRef.current.contains(event.target)) setMenuOpen(false); };
      document.addEventListener("pointerdown", close);
      return () => document.removeEventListener("pointerdown", close);
    }, [menuOpen]);

    if (!props.mainConversation || !threadId || !goal) return null;
    const act = async (method, input = {}) => {
      setBusy(true); setError("");
      try {
        const value = await api.invokeRuntime(method, { thread_id: threadId, ...input });
        setGoal(value?.goal ?? null);
        return true;
      } catch (reason) {
        setError(String(reason));
        return false;
      } finally { setBusy(false); }
    };
    if (editing) return h("div", { className: "plugin-goal-strip" },
      h("div", { className: "plugin-goal-editor" },
        h("textarea", { value: draft, onChange: (event) => setDraft(event.target.value), disabled: busy, autoFocus: true }),
        h("div", { className: "plugin-goal-editor-actions" },
          h("button", { className: "plugin-goal-text-button", disabled: busy || !draft.trim(), onClick: async () => { if (await act("goal.update_text", { objective: draft.trim() })) setEditing(false); } }, tr("goal.save")),
          h("button", { className: "plugin-goal-text-button", disabled: busy, onClick: () => setEditing(false) }, tr("goal.cancel"))
        )
      ), error ? h("div", { className: "plugin-goal-error" }, error) : null
    );

    const persistedStart = Number(goal.running_since_ms);
    const clockActive = Boolean(props.running) && (Number.isFinite(persistedStart) || goal.status === "active");
    if (clockActive && !Number.isFinite(persistedStart) && optimisticStartRef.current === 0) optimisticStartRef.current = Date.now();
    if (!clockActive) optimisticStartRef.current = 0;
    const runningSinceMs = Number.isFinite(persistedStart) ? persistedStart : optimisticStartRef.current || undefined;
    const elapsedMs = Number.isFinite(Number(goal.time_used_ms)) ? Number(goal.time_used_ms) : Math.max(0, Number(goal.time_used_seconds) || 0) * 1000;
    const state = goal.status === "complete" ? "complete" : goal.status === "blocked" ? "blocked" : goal.status === "paused" ? "paused" : clockActive ? "running" : "ready";
    const stateLabel = tr(`goal.${state}`);
    const resumable = goal.status === "paused" || goal.status === "blocked";
    const stateVisual = state === "running" ? h("span", { className: "plugin-goal-spinner" }) : h(Icon, { name: state === "complete" ? "check" : state === "blocked" ? "alert" : state === "paused" ? "pause" : "target" });

    return h("div", { className: "plugin-goal-strip" },
      h("div", { className: "plugin-goal-row" },
        h("span", { className: "plugin-goal-state", "data-state": state, title: stateLabel, "aria-label": stateLabel }, stateVisual),
        h("span", { className: "plugin-goal-text", title: goal.objective }, goal.objective),
        h(LiveDuration, { className: "plugin-goal-time", elapsedMs, runningSinceMs, active: clockActive, title: tr("goal.elapsed") }),
        h("div", { className: "plugin-goal-controls", ref: menuRef },
          goal.status === "complete" ? null : h("button", {
            className: "plugin-goal-icon-button", type: "button", disabled: busy,
            title: tr(resumable ? "goal.resume" : "goal.pause"), "aria-label": tr(resumable ? "goal.resume" : "goal.pause"),
            onClick: () => void act(resumable ? "goal.resume" : "goal.pause")
          }, h(Icon, { name: resumable ? "play" : "pause" })),
          h("button", {
            className: "plugin-goal-icon-button", type: "button", disabled: busy, "aria-expanded": menuOpen,
            title: tr("goal.more"), "aria-label": tr("goal.more"), onClick: () => { setMenuOpen(!menuOpen); setConfirmClear(false); }
          }, h(Icon, { name: "more" })),
          menuOpen ? h("div", { className: "plugin-goal-menu", role: "menu" },
            goal.status === "complete" ? null : h("button", {
              className: "plugin-goal-menu-button", type: "button", role: "menuitem", disabled: busy,
              onClick: () => { setDraft(goal.objective); setEditing(true); setMenuOpen(false); }
            }, h(Icon, { name: "edit" }), tr("goal.edit")),
            h("button", {
              className: "plugin-goal-menu-button danger", type: "button", role: "menuitem", disabled: busy,
              onClick: () => { if (confirmClear) void act("goal.clear"); else setConfirmClear(true); }
            }, h(Icon, { name: "trash" }), tr(confirmClear ? "goal.confirmClear" : "goal.clear"))
          ) : null
        )
      ), error ? h("div", { className: "plugin-goal-error" }, error) : null
    );
  }

  api.registerSlot("composer.above", { id: "goal-strip", order: 20, render(context) { return h(GoalStrip, context); } });
}
