export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Page, Panel, Stack, Row, Button, TextInput, TextArea, Select, Checkbox, EmptyState } = api.ui;

  api.registerLocale({ id: "automation-en", locale: "en-US", entries: {
    "automation.title": "Automations", "automation.subtitle": "Scheduled prompts and recurring work.",
    "automation.new": "New automation", "automation.empty": "No automations yet",
    "automation.emptyHelp": "Scheduled prompts will show up here.",
    "automation.search": "Search automations", "automation.filter.all": "All", "automation.filter.active": "Active",
    "automation.filter.empty": "No tasks match this filter.",
    "automation.name": "Name", "automation.prompt": "Prompt", "automation.schedule": "Cron schedule", "automation.timezone": "Timezone", "automation.workspace": "Workspace",
    "automation.recurring": "Repeat", "automation.workspaceHelp": "Tasks and runs shown below belong to this workspace.", "automation.workspaceNone": "No available project workspaces",
    "automation.isolation": "Run in an isolated git worktree", "automation.isolationHelp": "Each run executes in its own worktree instead of the live project directory.",
    "automation.create": "Create", "automation.cancel": "Cancel", "automation.pause": "Pause", "automation.resume": "Resume", "automation.remove": "Delete",
    "automation.paused": "Paused",
    "automation.mode.new": "New thread", "automation.mode.wake": "Wake session", "automation.mode.worktree": "worktree",
    "automation.next": "Next", "automation.run.completed": "Completed", "automation.run.failed": "Failed",
    "automation.run.running": "Running", "automation.run.queued": "Queued", "automation.run.interrupted": "Interrupted", "automation.run.never": "No runs yet",
    "automation.schedule.daily": "Daily", "automation.schedule.weekdays": "Weekdays", "automation.schedule.weekly": "Weekly",
    "automation.placeholder.prompt": "e.g. Summarize yesterday's progress at 9am…", "automation.placeholder.schedule": "0 9 * * 1-5",
  }});
  api.registerLocale({ id: "automation-zh", locale: "zh-CN", entries: {
    "automation.title": "自动化", "automation.subtitle": "按计划运行的提示词与周期性任务。",
    "automation.new": "新建自动化", "automation.empty": "还没有自动化任务",
    "automation.emptyHelp": "按时间执行的提示词会显示在这里。",
    "automation.search": "搜索自动化", "automation.filter.all": "全部", "automation.filter.active": "已开启",
    "automation.filter.empty": "没有符合筛选的任务。",
    "automation.name": "名称", "automation.prompt": "提示词", "automation.schedule": "Cron 时间", "automation.timezone": "时区", "automation.workspace": "工作区",
    "automation.recurring": "重复执行", "automation.workspaceHelp": "下方任务和运行记录都属于这个工作区。", "automation.workspaceNone": "没有可用的项目工作区",
    "automation.isolation": "在独立 git worktree 中运行", "automation.isolationHelp": "每次运行都在自己的 worktree 中执行，不直接改动当前项目目录。",
    "automation.create": "创建", "automation.cancel": "取消", "automation.pause": "暂停", "automation.resume": "继续", "automation.remove": "删除",
    "automation.paused": "已暂停",
    "automation.mode.new": "新会话", "automation.mode.wake": "唤醒会话", "automation.mode.worktree": "worktree",
    "automation.next": "下次", "automation.run.completed": "已完成", "automation.run.failed": "失败",
    "automation.run.running": "运行中", "automation.run.queued": "排队中", "automation.run.interrupted": "已中断", "automation.run.never": "暂无运行",
    "automation.schedule.daily": "每天", "automation.schedule.weekdays": "工作日", "automation.schedule.weekly": "每周",
    "automation.placeholder.prompt": "例如：每天早上 9 点整理昨天的进展…", "automation.placeholder.schedule": "0 9 * * 1-5",
  }});
  api.registerStyle({ id: "automation-catalog", css: `
    .plugin-automation { height:100%; overflow:auto; container-type:inline-size; color:var(--wuu-color-text, var(--ink, #181818)); background:var(--wuu-color-canvas, var(--paper, #fff)); }
    .plugin-automation .plugin-ui-page { padding-top:calc(var(--wuu-space-unit,4px) * 10 * var(--wuu-space-density,1)); }

    /* Catalog header: a display title + one-line subtitle on the left and the
     * page's single primary action pinned to the right edge. */
    .plugin-automation-head { align-items:flex-start; justify-content:space-between; gap:16px; }
    .plugin-automation-heading { flex:1; min-width:0; display:grid; gap:8px; }
    .plugin-automation-title { margin:0; color:var(--wuu-color-text-strong, var(--ink-strong, var(--ink))); font-size:28px; font-weight:var(--weight-semibold,600); letter-spacing:-0.02em; line-height:1.15; }
    .plugin-automation-subtitle { margin:0; color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:var(--font-body,14px); line-height:1.45; }
    .plugin-automation-new { flex:none; margin-top:4px; }

    /* Toolbar: full-width search on the left, the workspace switcher as one
     * inline settings row on the right — a quiet 12px label beside the
     * shared 32px select, not a stacked field block. */
    .plugin-automation-toolbar { align-items:center; gap:12px; flex-wrap:nowrap; }
    .plugin-automation-search { flex:1; min-width:0; display:flex; align-items:center; gap:8px; height:36px; border:1px solid var(--wuu-color-border-subtle, var(--hairline)); border-radius:10px; padding:0 12px; background:var(--wuu-control-field-background, var(--surface-1)); color:var(--wuu-color-text-muted, var(--ink-muted)); transition:border-color var(--motion-fast,120ms) ease, box-shadow var(--motion-fast,120ms) ease; }
    .plugin-automation-search:focus-within { border-color:var(--wuu-color-border-strong, var(--gray-350)); box-shadow:0 0 0 3px var(--ink-overlay-8, rgba(127,127,127,0.18)); }
    .plugin-automation-search svg { width:15px; height:15px; flex:none; }
    .plugin-automation-search input { flex:1; min-width:0; border:0; outline:0; padding:0; background:transparent; color:var(--wuu-color-text, var(--ink)); font:inherit; font-size:var(--font-ui,13px); }
    .plugin-automation-search input::placeholder { color:var(--wuu-color-text-muted, var(--ink-faint)); }
    .plugin-automation-workspace-picker { flex:none; display:flex; align-items:center; gap:8px; min-width:0; }
    .plugin-automation-workspace-picker .plugin-ui-field { display:flex; align-items:center; gap:8px; }
    .plugin-automation-workspace-picker .plugin-ui-field-label { flex:none; color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:var(--font-sm,12px); font-weight:400; white-space:nowrap; }
    .plugin-automation-workspace-picker .plugin-ui-select { width:min(200px, 32cqi); flex:none; }

    /* Status tabs: quiet text pills, the selected one lifted on a soft fill. */
    .plugin-automation-filters { display:flex; gap:2px; }
    .plugin-automation-filter { border:0; border-radius:var(--radius-pill,999px); padding:4px 12px; background:transparent; color:var(--wuu-color-text-muted, var(--ink-muted)); cursor:pointer; font:inherit; font-size:var(--font-sm,12px); font-weight:var(--weight-medium,500); line-height:1.6; transition:background-color var(--motion-fast,120ms) ease, color var(--motion-fast,120ms) ease; }
    .plugin-automation-filter:not(:disabled):hover { color:var(--wuu-color-text, var(--ink)); }
    .plugin-automation-filter[aria-pressed="true"] { background:var(--wuu-color-surface-muted, var(--surface-2)); color:var(--wuu-color-text, var(--ink)); }

    /* The create form is the one enclosed surface on the page: a quiet
     * hairline panel named by a 12px group label, like a settings group. */
    .plugin-automation-form { display:flex; flex-direction:column; gap:calc(var(--wuu-space-unit,4px) * 3 * var(--wuu-space-density,1)); }
    .plugin-automation-form-title { margin:0; color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:var(--font-sm,12px); font-weight:400; line-height:1.3; }
    .plugin-automation-form-grid { display:grid; grid-template-columns:1fr 1fr; gap:calc(var(--wuu-space-unit,4px) * 3 * var(--wuu-space-density,1)); }
    .plugin-automation-form-actions { justify-content:flex-end; gap:8px; }

    /* Task rows are borderless and open: a status dot, a two-line text block
     * and a right-hand state column. Grouping comes from whitespace rhythm
     * and the hover wash, not from card boxes. */
    .plugin-automation-list { display:flex; flex-direction:column; gap:2px; margin:0 -8px; }
    .plugin-automation-item { display:flex; align-items:center; gap:12px; min-width:0; padding:10px 8px; border-radius:10px; transition:background-color var(--motion-fast,120ms) ease; }
    .plugin-automation-item:hover { background:var(--wuu-color-surface-muted, var(--surface-1)); }
    .plugin-automation-item[data-paused="true"] .plugin-automation-item-main { opacity:0.55; }
    .plugin-automation-item-dot { width:8px; height:8px; flex:none; border-radius:50%; background:var(--wuu-color-text-muted, var(--ink-faint)); }
    .plugin-automation-item-dot[data-run="completed"] { background:var(--success, #1f9d55); }
    .plugin-automation-item-dot[data-run="failed"] { background:var(--danger, #b42318); }
    .plugin-automation-item-dot[data-run="running"], .plugin-automation-item-dot[data-run="queued"] { background:var(--accent-warm, #ef5b18); }
    .plugin-automation-item-main { flex:1; min-width:0; display:grid; gap:2px; }
    .plugin-automation-item-title-row { display:flex; align-items:center; gap:8px; min-width:0; }
    .plugin-automation-item-title { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--wuu-color-text, var(--ink)); font-size:var(--font-ui,13px); font-weight:var(--weight-semibold,600); line-height:1.4; }
    .plugin-automation-badge { flex:none; padding:1px 8px; border-radius:var(--radius-pill,999px); background:var(--wuu-color-surface-muted, var(--surface-2)); color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:11px; font-weight:var(--weight-medium,500); line-height:1.7; }
    .plugin-automation-item-meta { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:var(--font-sm,12px); line-height:1.5; }
    .plugin-automation-item-side { flex:none; display:flex; align-items:center; gap:10px; }
    .plugin-automation-item-next { color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:var(--font-sm,12px); white-space:nowrap; }
    .plugin-automation-item-next .plugin-automation-status-label { color:var(--wuu-color-text-muted, var(--ink-faint)); }
    .plugin-automation-item-actions { display:flex; align-items:center; gap:2px; opacity:0; transition:opacity var(--motion-fast,120ms) ease; }
    .plugin-automation-item:hover .plugin-automation-item-actions, .plugin-automation-item-actions:focus-within { opacity:1; }
    .plugin-automation-filtered-empty { margin:0; padding:calc(var(--wuu-space-unit,4px) * 8 * var(--wuu-space-density,1)) 0; color:var(--wuu-color-text-muted, var(--ink-muted)); font-size:var(--font-ui,13px); text-align:center; }

    /* When nothing is scheduled the empty state owns the page: it fills the
     * visible region instead of floating directly under the header. */
    .plugin-automation-empty { min-height:min(52vh, 480px); }
    .plugin-automation-error { color:var(--wuu-color-danger, var(--danger, #b42318)); font-size:var(--font-ui,13px); }
    .plugin-automation-sr { position:absolute; width:1px; height:1px; margin:-1px; overflow:hidden; clip:rect(0 0 0 0); white-space:nowrap; }

    @container (max-width: 640px) {
      .plugin-automation-head { flex-direction:column; }
      .plugin-automation-new { margin-top:0; }
      .plugin-automation-toolbar { flex-wrap:wrap; }
      .plugin-automation-search { flex:1 1 100%; }
      .plugin-automation-workspace-picker, .plugin-automation-workspace-picker .plugin-ui-field { flex:1; }
      .plugin-automation-workspace-picker .plugin-ui-select { flex:1; width:auto; min-width:0; }
      .plugin-automation-form-grid { grid-template-columns:1fr; }
    }
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

  function formatTime(hour, minute, timezone) {
    const date = new Date(Date.UTC(2026, 0, 4, hour, minute));
    const options = { hour: "numeric", minute: "2-digit" };
    try {
      return new Intl.DateTimeFormat(undefined, { ...options, timeZone: timezone || undefined }).format(date);
    } catch {
      return new Intl.DateTimeFormat(undefined, options).format(date);
    }
  }

  const WEEKDAY_REFERENCE = ["2026-01-04", "2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08", "2026-01-09", "2026-01-10"];

  function weekdayName(day) {
    return new Intl.DateTimeFormat(undefined, { weekday: "long" }).format(new Date(`${WEEKDAY_REFERENCE[day]}T12:00:00`));
  }

  // Recognize the schedules people actually write (daily, weekdays, one
  // weekday per week) and present them in words; anything more expressive
  // stays as the raw cron expression.
  function describeSchedule(cron, timezone, tr) {
    const fields = String(cron || "").trim().split(/\s+/);
    if (fields.length !== 5) return null;
    const [minute, hour, dom, month, dow] = fields;
    if (dom !== "*" || month !== "*") return null;
    if (!/^\d{1,2}$/.test(minute) || !/^\d{1,2}$/.test(hour)) return null;
    const time = formatTime(Number(hour), Number(minute), timezone);
    if (dow === "*") return `${tr("automation.schedule.daily")} ${time}`;
    if (dow === "1-5") return `${tr("automation.schedule.weekdays")} ${time}`;
    if (/^[0-7]$/.test(dow)) return `${tr("automation.schedule.weekly")} ${weekdayName(Number(dow) % 7)} ${time}`;
    return null;
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

  function SearchIcon() {
    return h("svg", { viewBox: "0 0 16 16", fill: "none", "aria-hidden": true },
      h("circle", { cx: "7", cy: "7", r: "4.5", stroke: "currentColor", "stroke-width": "1.5" }),
      h("path", { d: "m13.5 13.5-3-3", stroke: "currentColor", "stroke-width": "1.5", "stroke-linecap": "round" }));
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
    const [query, setQuery] = React.useState("");
    const [filter, setFilter] = React.useState("all");
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
    const normalizedQuery = query.trim().toLowerCase();
    const visibleTasks = tasks.filter((task) => {
      if (filter === "active" && task.paused) return false;
      if (filter === "paused" && !task.paused) return false;
      if (normalizedQuery) {
        const haystack = `${task.title || ""}\n${task.prompt || ""}`.toLowerCase();
        if (!haystack.includes(normalizedQuery)) return false;
      }
      return true;
    });
    return h("main", { className: "plugin-automation" }, h(Page, null, h(Stack, { gap: "large" },
      h(Row, { className: "plugin-automation-head" },
        h("div", { className: "plugin-automation-heading" },
          h("h1", { className: "plugin-automation-title" }, tr("automation.title")),
          h("p", { className: "plugin-automation-subtitle" }, tr("automation.subtitle"))),
        h(Button, { className: "plugin-automation-new", variant: "primary", disabled: busy || creating || !selectedWorkspace, onClick: () => setCreating(true) }, tr("automation.new"))),
      h(Row, { className: "plugin-automation-toolbar" },
        tasks.length > 0
          ? h("label", { className: "plugin-automation-search" },
              h("span", { className: "plugin-automation-sr" }, tr("automation.search")),
              h(SearchIcon),
              h("input", { type: "search", value: query, placeholder: tr("automation.search"), onChange: (event) => setQuery(event.target.value) }))
          : null,
        h("div", { className: "plugin-automation-workspace-picker" },
          h(Select, {
            label: tr("automation.workspace"),
            title: tr("automation.workspaceHelp"),
            value: selectedWorkspaceID,
            disabled: busy || availableWorkspaces.length === 0,
            onChange: (event) => setSelectedWorkspaceID(event.target.value),
          }, availableWorkspaces.length === 0
            ? h("option", { value: "" }, tr("automation.workspaceNone"))
            : availableWorkspaces.map((candidate) => h("option", { key: candidate.id, value: candidate.id }, candidate.name || workspaceName(candidate.root)))))),
      tasks.length > 0
        ? h("div", { className: "plugin-automation-filters", role: "group" },
            [["all", tr("automation.filter.all")], ["active", tr("automation.filter.active")], ["paused", tr("automation.paused")]].map(([value, label]) =>
              h("button", { key: value, type: "button", className: "plugin-automation-filter", "aria-pressed": filter === value ? "true" : "false", onClick: () => setFilter(value) }, label)))
        : null,
      creating ? h(Panel, null, h("div", { className: "plugin-automation-form" },
        h("h2", { className: "plugin-automation-form-title" }, tr("automation.new")),
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
        ? (creating ? null : h(EmptyState, { className: "plugin-automation-empty", title: tr("automation.empty"), description: selectedWorkspace ? tr("automation.emptyHelp") : tr("automation.workspaceNone") }))
        : visibleTasks.length === 0
          ? h("p", { className: "plugin-automation-filtered-empty" }, tr("automation.filter.empty"))
          : h("div", { className: "plugin-automation-list" }, visibleTasks.map((task) => {
              const title = task.title || task.prompt;
              const status = runStatus(lastRunFor(task));
              const next = task.paused ? null : formatDateTime(task.next_run_at, task.timezone);
              const meta = [
                describeSchedule(task.cron, task.timezone, tr) || task.cron,
                tr(task.mode === "thread_heartbeat" ? "automation.mode.wake" : "automation.mode.new"),
                task.workspace_mode === "worktree" ? tr("automation.mode.worktree") : null,
                shortTimezone(task.timezone),
              ].filter(Boolean).join(" · ");
              return h("article", { className: "plugin-automation-item", key: task.id, "data-paused": task.paused ? "true" : "false" },
                h("span", { className: "plugin-automation-item-dot", "data-run": status || undefined, title: status ? tr("automation.run." + status) : tr("automation.run.never") }),
                h("div", { className: "plugin-automation-item-main" },
                  h("div", { className: "plugin-automation-item-title-row" },
                    h("span", { className: "plugin-automation-item-title", title }, title),
                    task.paused ? h("span", { className: "plugin-automation-badge" }, tr("automation.paused")) : null),
                  h("div", { className: "plugin-automation-item-meta", title: task.cron }, meta)),
                h("div", { className: "plugin-automation-item-side" },
                  next ? h("span", { className: "plugin-automation-item-next" }, h("span", { className: "plugin-automation-status-label" }, tr("automation.next")), " ", next) : null,
                  h("div", { className: "plugin-automation-item-actions" },
                    h(Button, { variant: "ghost", disabled: busy, onClick: () => act("automation.update", { ...task, paused: !task.paused }) }, tr(task.paused ? "automation.resume" : "automation.pause")),
                    h(Button, { variant: "danger", disabled: busy, onClick: () => act("automation.remove", { id: task.id }) }, tr("automation.remove")))));
            }))
    )));
  }

  api.registerViewType({ id: "automation.catalog", title: "Automations", icon: "clock", defaultRegion: "primary", persistence: "durable", render: (props) => h(Catalog, props) });
}
