export async function activate(api) {
  const React = api.react;
  const h = React.createElement;

  api.registerLocale({ id: "goal-en", locale: "en-US", entries: {
    "goal.pause": "Pause goal", "goal.resume": "Resume goal", "goal.edit": "Edit goal",
    "goal.clear": "Clear goal", "goal.confirmClear": "Click again to confirm",
    "goal.ready": "Ready to continue", "goal.paused": "Paused", "goal.blocked": "Blocked", "goal.complete": "Complete",
    "goal.save": "Save", "goal.cancel": "Cancel", "goal.emptyText": "Goal text cannot be empty"
  }});
  api.registerLocale({ id: "goal-zh", locale: "zh-CN", entries: {
    "goal.pause": "暂停目标", "goal.resume": "继续目标", "goal.edit": "编辑目标",
    "goal.clear": "清除目标", "goal.confirmClear": "再次点击确认清除",
    "goal.ready": "可以继续", "goal.paused": "已暂停", "goal.blocked": "已阻塞", "goal.complete": "已完成",
    "goal.save": "保存", "goal.cancel": "取消", "goal.emptyText": "目标内容不能为空"
  }});
  api.registerStyle({ id: "goal-strip", css: `
    .plugin-goal-strip { display:flex; flex-direction:column; gap:8px; margin:0 2px 8px; padding:9px 11px;
      border:1px solid var(--wuu-color-border-subtle); border-radius:var(--wuu-radius-control); background:var(--wuu-color-surface-muted); }
    .plugin-goal-row { display:flex; align-items:center; gap:8px; min-width:0; }
    .plugin-goal-mark { color:var(--wuu-color-accent); font-size:13px; }
    .plugin-goal-text { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:12px; color:var(--wuu-color-text); }
    .plugin-goal-status { flex:none; font-size:11px; color:var(--wuu-color-text-muted); }
    .plugin-goal-actions { display:flex; gap:4px; flex:none; }
    .plugin-goal-button { border:0; padding:3px 6px; border-radius:5px; background:transparent; color:var(--wuu-color-text-muted); font:inherit; font-size:11px; cursor:pointer; }
    .plugin-goal-button:hover { background:var(--wuu-color-surface-elevated); color:var(--wuu-color-text); }
    .plugin-goal-button:disabled { opacity:.45; cursor:default; }
    .plugin-goal-editor { display:flex; gap:6px; align-items:flex-end; }
    .plugin-goal-editor textarea { flex:1; resize:vertical; min-height:44px; padding:6px 8px; border:1px solid var(--wuu-color-border-subtle); border-radius:6px; background:var(--wuu-color-surface); color:var(--wuu-color-text); font:inherit; font-size:12px; }
    .plugin-goal-error { font-size:11px; color:var(--wuu-color-danger, #c44); }
  ` });

  function GoalStrip(props) {
    const threadId = typeof props.threadId === "string" ? props.threadId : "";
    const translate = typeof props.translate === "function" ? props.translate : (key) => key;
    const [goal, setGoal] = React.useState(null);
    const [editing, setEditing] = React.useState(false);
    const [draft, setDraft] = React.useState("");
    const [busy, setBusy] = React.useState(false);
    const [confirmClear, setConfirmClear] = React.useState(false);
    const [error, setError] = React.useState("");

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
    React.useEffect(() => { setEditing(false); setConfirmClear(false); setError(""); }, [threadId, goal?.updated_at]);

    if (!props.mainConversation || !threadId || !goal) return null;
    const tr = (key) => {
      const value = translate(key);
      return typeof value === "string" && value !== key ? value : key.split(".").pop();
    };
    const act = async (method, input = {}) => {
      setBusy(true); setError("");
      try { const value = await api.invokeRuntime(method, { thread_id: threadId, ...input }); setGoal(value?.goal ?? null); }
      catch (reason) { setError(String(reason)); }
      finally { setBusy(false); }
    };
    if (editing) return h("div", { className: "plugin-goal-strip" },
      h("div", { className: "plugin-goal-editor" },
        h("textarea", { value: draft, onChange: (event) => setDraft(event.target.value), disabled: busy, autoFocus: true }),
        h("button", { className: "plugin-goal-button", disabled: busy || !draft.trim(), onClick: async () => { await act("goal.update_text", { objective: draft.trim() }); setEditing(false); } }, tr("goal.save")),
        h("button", { className: "plugin-goal-button", disabled: busy, onClick: () => setEditing(false) }, tr("goal.cancel"))
      ), error ? h("div", { className: "plugin-goal-error" }, error) : null
    );
    const statusKey = goal.status === "paused" ? "goal.paused" : goal.status === "blocked" ? "goal.blocked" : goal.status === "complete" ? "goal.complete" : "goal.ready";
    const terminal = goal.status === "complete";
    return h("div", { className: "plugin-goal-strip" },
      h("div", { className: "plugin-goal-row" },
        h("span", { className: "plugin-goal-mark", "aria-hidden": true }, "◎"),
        h("span", { className: "plugin-goal-text", title: goal.objective }, goal.objective),
        h("span", { className: "plugin-goal-status" }, tr(statusKey)),
        h("div", { className: "plugin-goal-actions" },
          terminal ? null : h("button", { className: "plugin-goal-button", disabled: busy, onClick: () => { setDraft(goal.objective); setEditing(true); } }, tr("goal.edit")),
          terminal ? null : h("button", { className: "plugin-goal-button", disabled: busy, onClick: () => act(goal.status === "paused" || goal.status === "blocked" ? "goal.resume" : "goal.pause") }, tr(goal.status === "paused" || goal.status === "blocked" ? "goal.resume" : "goal.pause")),
          h("button", { className: "plugin-goal-button", disabled: busy, onClick: () => { if (confirmClear) void act("goal.clear"); else setConfirmClear(true); } }, tr(confirmClear ? "goal.confirmClear" : "goal.clear"))
        )
      ), error ? h("div", { className: "plugin-goal-error" }, error) : null
    );
  }

  api.registerSlot("composer.above", { id: "goal-strip", order: 20, render(context) { return h(GoalStrip, context); } });
}
