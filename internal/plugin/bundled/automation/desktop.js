export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Page, Panel, Card, Stack, Row, Button, TextInput, TextArea, Select, Checkbox, EmptyState } = api.ui;

  api.registerLocale({ id: "automation-en", locale: "en-US", entries: {
    "automation.title": "Automations", "automation.new": "New automation", "automation.empty": "No automations yet",
    "automation.emptyHelp": "Scheduled prompts will show up here.",
    "automation.name": "Name", "automation.prompt": "Prompt", "automation.schedule": "Cron schedule", "automation.timezone": "Timezone", "automation.workspace": "Workspace",
    "automation.recurring": "Repeat", "automation.workspaceHelp": "Tasks and runs shown below belong to this workspace.", "automation.workspaceNone": "No available project workspaces",
    "automation.isolation": "Run in an isolated git worktree", "automation.isolationHelp": "Each run executes in its own worktree instead of the live project directory.",
    "automation.create": "Create", "automation.cancel": "Cancel", "automation.pause": "Pause", "automation.resume": "Resume", "automation.remove": "Delete",
    "automation.paused": "Paused",
    "automation.mode.new": "New thread", "automation.mode.wake": "Wake session", "automation.mode.worktree": "worktree",
    "automation.next": "Next", "automation.run.completed": "Completed", "automation.run.failed": "Failed",
    "automation.run.running": "Running", "automation.run.queued": "Queued", "automation.run.interrupted": "Interrupted", "automation.run.never": "No runs yet",
    "automation.placeholder.prompt": "e.g. Summarize yesterday's progress at 9am…", "automation.placeholder.schedule": "0 9 * * 1-5",
  }});
  api.registerLocale({ id: "automation-zh", locale: "zh-CN", entries: {
    "automation.title": "自动化", "automation.new": "新建自动化", "automation.empty": "还没有自动化任务",
    "automation.emptyHelp": "按时间执行的提示词会显示在这里。",
    "automation.name": "名称", "automation.prompt": "提示词", "automation.schedule": "Cron 时间", "automation.timezone": "时区", "automation.workspace": "工作区",
    "automation.recurring": "重复执行", "automation.workspaceHelp": "下方任务和运行记录都属于这个工作区。", "automation.workspaceNone": "没有可用的项目工作区",
    "automation.isolation": "在独立 git worktree 中运行", "automation.isolationHelp": "每次运行都在自己的 worktree 中执行，不直接改动当前项目目录。",
    "automation.create": "创建", "automation.cancel": "取消", "automation.pause": "暂停", "automation.resume": "继续", "automation.remove": "删除",
    "automation.paused": "已暂停",
    "automation.mode.new": "新会话", "automation.mode.wake": "唤醒会话", "automation.mode.worktree": "worktree",
    "automation.next": "下次", "automation.run.completed": "已完成", "automation.run.failed": "失败",
    "automation.run.running": "运行中", "automation.run.queued": "排队中", "automation.run.interrupted": "已中断", "automation.run.never": "暂无运行",
    "automation.placeholder.prompt": "例如：每天早上 9 点整理昨天的进展…", "automation.placeholder.schedule": "0 9 * * 1-5",
  }});
  api.registerStyle({ id: "automation-catalog", css: `
    .plugin-automation { height:100%; overflow:auto; container-type:inline-size; color:var(--wuu-color-text, var(--ink, #181818)); background:var(--wuu-color-canvas, var(--paper, #fff)); }
    .plugin-automation-head { align-items:flex-end; gap:16px; }
    .plugin-automation-title { margin:0; color:var(--wuu-color-text-strong, var(--ink-strong, var(--ink))); font-size:var(--font-heading,18px); font-weight:var(--weight-semibold,600); letter-spacing:-0.01em; line-height:1.3; }
    .plugin-automation-heading { flex:1; min-width:0; }
    .plugin-automation-workspace { margin-top:3px; overflow:hidden; color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:var(--font-sm,12px); text-overflow:ellipsis; white-space:nowrap; }
    .plugin-automation-workspace-picker { width:min(320px, 42cqi); flex:none; }
    .plugin-automation-new { flex:none; }

    /* The create form is the one enclosed surface on the page: a quiet
     * hairline panel so the in-progress draft reads as a temporary zone. */
    .plugin-automation-form { display:flex; flex-direction:column; gap:14px; }
    .plugin-automation-form-grid { display:grid; grid-template-columns:1fr 1fr; gap:14px; }
    @container (max-width: 560px) { .plugin-automation-head { align-items:stretch; flex-direction:column; } .plugin-automation-workspace-picker { width:100%; } .plugin-automation-form-grid { grid-template-columns:1fr; } }
    .plugin-automation-form-actions { justify-content:flex-end; gap:8px; }

    .plugin-automation-list { display:grid; gap:10px; }
    .plugin-automation-card { display:flex; flex-direction:column; gap:10px; transition:border-color var(--motion-fast,120ms) ease; }
    .plugin-automation-card:hover { border-color:var(--wuu-color-border-strong, var(--hairline-strong)); }
    .plugin-automation-card[data-paused="true"] { opacity:0.68; }
    .plugin-automation-card-head { display:flex; align-items:center; gap:8px; min-width:0; }
    .plugin-automation-card-title { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--wuu-color-text, var(--ink)); font-size:var(--font-ui,13px); font-weight:var(--weight-semibold,600); line-height:1.4; }
    .plugin-automation-badge { flex:none; padding:1px 8px; border-radius:var(--radius-pill,999px); background:var(--wuu-color-surface-muted, var(--surface-2)); color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:11px; font-weight:var(--weight-medium,500); line-height:1.7; }

    .plugin-automation-meta { display:flex; flex-wrap:wrap; align-items:center; gap:6px 8px; min-width:0; color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:var(--font-sm,12px); line-height:1.5; }
    .plugin-automation-chip { flex:none; padding:1px 6px; border-radius:6px; background:var(--wuu-color-surface-muted, var(--surface-2)); color:var(--wuu-color-text, var(--ink-soft)); }
    .plugin-automation-chip-code { font:11px/1.6 var(--wuu-font-mono, ui-monospace, monospace); font-variant-numeric:tabular-nums; }
    .plugin-automation-chip-mode { color:var(--wuu-color-text-muted, var(--ink-muted)); }
    .plugin-automation-status { row-gap:4px; }
    .plugin-automation-status-label { color:var(--wuu-color-text-muted, var(--ink-faint)); }
    .plugin-automation-run { display:inline-flex; align-items:center; gap:6px; color:var(--wuu-color-text-muted, var(--ink-muted)); }
    .plugin-automation-run-dot { width:6px; height:6px; border-radius:50%; background:currentColor; }
    .plugin-automation-run[data-run="completed"] { color:var(--success, #1f9d55); }
    .plugin-automation-run[data-run="failed"] { color:var(--danger, #b42318); }
    .plugin-automation-run[data-run="running"], .plugin-automation-run[data-run="queued"] { color:var(--accent-warm, #ef5b18); }
    .plugin-automation-run[data-run="interrupted"] { color:var(--wuu-color-text-muted, var(--ink-muted)); }
    .plugin-automation-prompt { margin:0; display:-webkit-box; -webkit-box-orient:vertical; -webkit-line-clamp:2; overflow:hidden; color:var(--wuu-color-text-muted, var(--ink-soft)); font-size:var(--font-ui,13px); line-height:1.55; white-space:pre-wrap; }
    .plugin-automation-error { color:var(--wuu-color-danger, var(--danger, #b42318)); font-size:var(--font-ui,13px); }
  ` });

  function shortTimezone(timezone) {
    const parts = String(timezone || "").split("/");
    return parts[parts.length - 1] || "UTC";
  }

  function workspaceName(root) {
    const parts = String(root || "").split(/[\\/]/).filter(Boolean);
    return parts[parts.length - 1] || root || "—";
  }

  function formatDateTime(iso, timezone) {
    if (!iso) return null;
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return null;
    const options = { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" };
    try {
      return new Intl.DateTimeFormat(undefined, { ...options, timeZone: timezone || undefined }).format(date);
    } catch {
      return new Intl.DateTimeFormat(undefined, options).format(date);
    }
  }

  function runStatus(run) {
    if (!run) return null;
    switch (run.status) {
      case "completed": return "completed";
      case "failed": return "failed";
      case "running":
      case "starting": return "running";
      case "queued": return "queued";
      case "interrupted":
      case "discarded": return "interrupted";
      default: return "queued";
    }
  }

  function Catalog(props) {
    const tr = props.translate;
    const [tasks, setTasks] = React.useState([]);
    const [runs, setRuns] = React.useState([]);
    const [workspaces, setWorkspaces] = React.useState([]);
    const [selectedWorkspaceID, setSelectedWorkspaceID] = React.useState("");
    const [creating, setCreating] = React.useState(false);
    const [busy, setBusy] = React.useState(false);
    const [error, setError] = React.useState("");
    const [draft, setDraft] = React.useState({ title: "", prompt: "", schedule: "0 9 * * 1-5", timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC", mode: "new_thread", recurring: true, workspace: "shared" });
    const refreshEpoch = React.useRef(0);
    const availableWorkspaces = workspaces.filter((candidate) => candidate.available !== false);
    const selectedWorkspace = availableWorkspaces.find((candidate) => candidate.id === selectedWorkspaceID);
    const loadWorkspaces = React.useCallback(async () => {
      setError("");
      try {
        const snapshot = await api.listWorkspaces();
        const items = Array.isArray(snapshot?.workspaces) ? snapshot.workspaces : [];
        const available = items.filter((candidate) => candidate?.available !== false && candidate?.id);
        setWorkspaces(items);
        setSelectedWorkspaceID((current) => {
          if (available.some((candidate) => candidate.id === current)) return current;
          if (available.some((candidate) => candidate.id === snapshot?.activeWorkspaceId)) return snapshot.activeWorkspaceId;
          return available[0]?.id || "";
        });
      } catch (reason) {
        setWorkspaces([]);
        setSelectedWorkspaceID("");
        setError(String(reason));
      }
    }, []);
    const refresh = React.useCallback(async (workspaceID) => {
      const epoch = ++refreshEpoch.current;
      if (!workspaceID) {
        setTasks([]);
        setRuns([]);
        return;
      }
      setError("");
      try {
        const options = { workspaceId: workspaceID };
        const [list, runList] = await Promise.all([
          api.invokeRuntime("automation.list", {}, options),
          api.invokeRuntime("automation.run.list", {}, options).catch(() => null),
        ]);
        if (epoch !== refreshEpoch.current) return;
        if (list?.workspace?.id !== workspaceID) throw new Error("Automation runtime returned a different workspace");
        setTasks(Array.isArray(list?.tasks) ? list.tasks : []);
        setRuns(Array.isArray(runList?.runs) ? runList.runs : []);
      } catch (reason) {
        if (epoch !== refreshEpoch.current) return;
        setTasks([]);
        setRuns([]);
        setError(String(reason));
      }
    }, []);
    React.useEffect(() => { void loadWorkspaces(); }, [loadWorkspaces]);
    React.useEffect(() => { void refresh(selectedWorkspaceID); }, [refresh, selectedWorkspaceID]);
    const act = async (method, input) => {
      if (!selectedWorkspace) {
        setError(tr("automation.workspaceNone"));
        return false;
      }
      setBusy(true);
      setError("");
      try {
        await api.invokeRuntime(method, {
          ...input,
          workspace_id: selectedWorkspace.id,
          workspace_root: selectedWorkspace.root,
        }, { workspaceId: selectedWorkspace.id });
        await refresh(selectedWorkspace.id);
        return true;
      } catch (reason) {
        setError(String(reason));
        return false;
      } finally {
        setBusy(false);
      }
    };
    const lastRunFor = (task) => runs
      .filter((run) => run.task_id === task.id)
      .sort((a, b) => new Date(b.triggered_at) - new Date(a.triggered_at))[0];
    return h("main", { className: "plugin-automation" }, h(Page, null, h(Stack, { gap: "large" },
      h(Row, { className: "plugin-automation-head" },
        h("div", { className: "plugin-automation-heading" },
          h("h1", { className: "plugin-automation-title" }, tr("automation.title")),
          h("div", { className: "plugin-automation-workspace", title: selectedWorkspace?.root || "" }, selectedWorkspace?.root || tr("automation.workspaceNone"))),
        h("div", { className: "plugin-automation-workspace-picker" },
          h(Select, {
            label: tr("automation.workspace"),
            description: tr("automation.workspaceHelp"),
            value: selectedWorkspaceID,
            disabled: busy || availableWorkspaces.length === 0,
            onChange: (event) => setSelectedWorkspaceID(event.target.value),
          }, availableWorkspaces.length === 0
            ? h("option", { value: "" }, tr("automation.workspaceNone"))
            : availableWorkspaces.map((candidate) => h("option", { key: candidate.id, value: candidate.id }, candidate.name || workspaceName(candidate.root))))),
        creating || tasks.length === 0 ? null : h(Button, { className: "plugin-automation-new", variant: "primary", disabled: !selectedWorkspace, onClick: () => setCreating(true) }, tr("automation.new"))),
      creating ? h(Panel, null, h("div", { className: "plugin-automation-form" },
        h(TextInput, { label: tr("automation.name"), value: draft.title, onChange: (event) => setDraft({ ...draft, title: event.target.value }) }),
        h(TextArea, { label: tr("automation.prompt"), rows: 4, placeholder: tr("automation.placeholder.prompt"), value: draft.prompt, onChange: (event) => setDraft({ ...draft, prompt: event.target.value }) }),
        h("div", { className: "plugin-automation-form-grid" },
          h(TextInput, { label: tr("automation.schedule"), placeholder: tr("automation.placeholder.schedule"), value: draft.schedule, onChange: (event) => setDraft({ ...draft, schedule: event.target.value }) }),
          h(TextInput, { label: tr("automation.timezone"), value: draft.timezone, onChange: (event) => setDraft({ ...draft, timezone: event.target.value }) })),
        h(Checkbox, { label: tr("automation.recurring"), checked: draft.recurring, onChange: (event) => setDraft({ ...draft, recurring: event.target.checked }) }),
        h(Checkbox, { label: tr("automation.isolation"), description: tr("automation.isolationHelp"), checked: draft.workspace === "worktree", onChange: (event) => setDraft({ ...draft, workspace: event.target.checked ? "worktree" : "shared" }) }),
        h(Row, { className: "plugin-automation-form-actions" },
          h(Button, { variant: "ghost", onClick: () => setCreating(false) }, tr("automation.cancel")),
          h(Button, { variant: "primary", disabled: busy || !selectedWorkspace || !draft.prompt.trim(), onClick: async () => { if (await act("automation.create", { ...draft, durable: true })) setCreating(false); } }, tr("automation.create"))))) : null,
      error ? h("div", { className: "plugin-automation-error", role: "alert" }, error) : null,
      tasks.length === 0
        ? h(EmptyState, { title: tr("automation.empty"), description: selectedWorkspace ? tr("automation.emptyHelp") : tr("automation.workspaceNone"), actions: h(Button, { variant: "primary", disabled: !selectedWorkspace, onClick: () => setCreating(true) }, tr("automation.new")) })
        : h("div", { className: "plugin-automation-list" }, tasks.map((task) => {
            const title = task.title || task.prompt;
            const body = task.title && task.title !== task.prompt ? task.prompt : "";
            const status = runStatus(lastRunFor(task));
            const next = task.paused ? null : formatDateTime(task.next_run_at, task.timezone);
            return h(Card, { className: "plugin-automation-card", key: task.id, "data-paused": task.paused ? "true" : "false" },
              h("div", { className: "plugin-automation-card-head" },
                h("span", { className: "plugin-automation-card-title", title }, title),
                task.paused ? h("span", { className: "plugin-automation-badge" }, tr("automation.paused")) : null,
                h(Button, { variant: "ghost", disabled: busy, onClick: () => act("automation.update", { ...task, paused: !task.paused }) }, tr(task.paused ? "automation.resume" : "automation.pause")),
                h(Button, { variant: "danger", disabled: busy, onClick: () => act("automation.remove", { id: task.id }) }, tr("automation.remove"))),
              h("div", { className: "plugin-automation-meta" },
                h("code", { className: "plugin-automation-chip plugin-automation-chip-code" }, task.cron),
                h("span", { className: "plugin-automation-chip plugin-automation-chip-mode" }, tr(task.mode === "thread_heartbeat" ? "automation.mode.wake" : "automation.mode.new")),
                task.workspace_mode === "worktree" ? h("span", { className: "plugin-automation-chip" }, tr("automation.mode.worktree")) : null,
                h("span", { className: "plugin-automation-tz" }, shortTimezone(task.timezone))),
              h("div", { className: "plugin-automation-meta plugin-automation-status" },
                next ? h("span", null, h("span", { className: "plugin-automation-status-label" }, tr("automation.next")), " ", next) : null,
                status
                  ? h("span", { className: "plugin-automation-run", "data-run": status }, h("span", { className: "plugin-automation-run-dot", "aria-hidden": true }), tr("automation.run." + status))
                  : h("span", { className: "plugin-automation-run" }, tr("automation.run.never"))),
              body ? h("p", { className: "plugin-automation-prompt" }, body) : null);
          }))
    )));
  }

  api.registerViewType({ id: "automation.catalog", title: "Automations", icon: "clock", defaultRegion: "primary", persistence: "durable", render: (props) => h(Catalog, props) });
}
