export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Stack, Row, Button, TextArea, TextInput } = api.ui;
  const labels = { active: "进行中", paused: "已暂停", blocked: "受阻", budget_limited: "预算已用尽", complete: "已完成" };
  const empty = Object.freeze({ goal: null, error: "", loaded: false, items: Object.freeze([]) });
  const cache = new Map();
  const listeners = new Map();
  const revisions = new Map();
  let disposed = false;
  function publish(id, value) {
    cache.set(id, value);
    for (const listener of listeners.get(id) || []) listener();
  }
  async function refresh(id) {
    if (!id || disposed) return;
    const revision = (revisions.get(id) || 0) + 1;
    revisions.set(id, revision);
    try {
      const value = await api.invokeRuntime("get_goal", { thread_id: id });
      if (disposed || revisions.get(id) !== revision) return;
      const goal = value?.goal || null;
      const items = goal ? Object.freeze([Object.freeze({
        id: goal.id, label: goal.objective,
        state: goal.status === "active" ? "running" : goal.error ? "error" : "idle",
        secondaryText: `${labels[goal.status] || goal.status} · ${goal.tokens_used} tokens${goal.token_budget == null ? "" : ` / ${goal.token_budget}`}`,
        tooltip: goal.error || goal.objective,
      })]) : empty.items;
      publish(id, { goal, error: "", loaded: true, items });
    } catch (error) {
      if (!disposed && revisions.get(id) === revision) {
        publish(id, { ...(cache.get(id) || empty), loaded: true, error: String(error.message || error) });
      }
    }
  }
  function subscribe(id, listener) {
    if (!id) return () => {};
    if (!listeners.has(id)) listeners.set(id, new Set());
    listeners.get(id).add(listener);
    void refresh(id);
    return () => {
      const current = listeners.get(id);
      current?.delete(listener);
      if (current?.size === 0) listeners.delete(id);
    };
  }
  // Observer writes can settle after the host's terminal notification.
  const timer = setInterval(() => { for (const id of listeners.keys()) void refresh(id); }, 1500);
  api.registerCleanup(() => { disposed = true; clearInterval(timer); listeners.clear(); cache.clear(); });
  api.onHostEvent((event) => {
    const method = event?.message?.method;
    if (["thread/started", "thread/resumed", "turn/started", "turn/completed", "turn/error"].includes(method)) {
      for (const id of listeners.keys()) void refresh(id);
    }
  });
  api.registerComposerStatusSource({
    id: "goal-status", order: 20,
    getSnapshot: ({ threadId }) => cache.get(threadId)?.items || empty.items,
    subscribe: ({ threadId }, listener) => subscribe(threadId, listener),
  });

  function Controls({ threadId }) {
    const state = React.useSyncExternalStore(
      React.useCallback((listener) => subscribe(threadId, listener), [threadId]),
      React.useCallback(() => cache.get(threadId) || empty, [threadId]),
    );
    const [objective, setObjective] = React.useState("");
    const [budget, setBudget] = React.useState("");
    const [busy, setBusy] = React.useState(false);
    const [error, setError] = React.useState("");
    const goal = state.goal;
    const canCreate = !goal || goal.status === "complete";
    const active = goal?.status === "active";
    async function act(method) {
      setBusy(true); setError("");
      try {
        const input = { thread_id: threadId };
        if (method === "create_goal") input.objective = objective;
        if ((method === "create_goal" || method === "resume") && budget.trim()) {
          const value = Number(budget);
          if (!Number.isSafeInteger(value) || value <= 0) throw new Error("Token 预算必须是正整数");
          input.token_budget = value;
        }
        await api.invokeRuntime(method, input);
        if (method === "create_goal" || method === "clear") { setObjective(""); setBudget(""); }
        await refresh(threadId);
      } catch (cause) { setError(String(cause.message || cause)); }
      finally { setBusy(false); }
    }
    if (!threadId) return h("p", null, "打开一个会话以设置目标。");
    return h(Stack, null,
      goal ? h(Stack, null,
        h("p", { className: "plugin-goal-objective" }, goal.objective),
        h("p", null, `${labels[goal.status] || goal.status} · ${goal.tokens_used} tokens${goal.token_budget == null ? "" : ` / ${goal.token_budget}`} · ${Math.floor(goal.time_used_seconds)} 秒`),
        goal.error ? h("p", { role: "alert" }, goal.error) : null,
      ) : h("p", null, state.loaded ? "尚未设置目标" : "正在读取目标…"),
      canCreate ? h(TextArea, { label: "目标", value: objective, disabled: busy, onChange: (event) => setObjective(event.target.value) }) : null,
      !active ? h(TextInput, { label: "Token 预算（可选，总额度）", value: budget, disabled: busy, placeholder: goal?.token_budget == null ? "不设上限" : String(goal.token_budget), onChange: (event) => setBudget(event.target.value) }) : null,
      h("p", { className: "plugin-goal-help" }, "预算按回合结束后的用量结算，可能超出一个回合。重启后目标暂停，点击继续恢复。"),
      h(Row, null,
        canCreate ? h(Button, { disabled: busy || !state.loaded || !objective.trim(), onClick: () => void act("create_goal") }, "开始目标") : null,
        goal && !canCreate ? h(Button, { disabled: busy, onClick: () => void act(active ? "pause" : "resume") }, active ? "暂停" : "继续") : null,
        goal ? h(Button, { disabled: busy, onClick: () => void act("clear") }, "清除") : null,
      ),
      error || state.error ? h("p", { role: "alert" }, error || state.error) : null,
    );
  }
  api.registerInspectorSection({
    id: "goal-controls", title: "Goal", priority: 20,
    render: ({ snapshot }) => h(Controls, { key: snapshot.session.id, threadId: snapshot.session.id }),
  });
  api.registerStyle({ id: "goal-controls", css: `
    .plugin-goal-objective { white-space:pre-wrap; overflow-wrap:anywhere; }
    .plugin-goal-help { color:var(--wuu-color-text-muted); font-size:var(--wuu-font-size-sm); }
  ` });
}
