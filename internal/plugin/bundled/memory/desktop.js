export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Page, Section, Stack, Row, Button, TextArea, EmptyState } = api.ui;

  api.registerLocale({ id: "memory-en", locale: "en-US", entries: {
    "memory.subtitle": "Durable preferences, feedback, references, and reusable lessons.",
    "memory.overview": "Memory overview", "memory.refresh": "Refresh", "memory.refreshing": "Refreshing…",
    "memory.raw": "Notebook files", "memory.rawHelp": "The MEMORY.md index and every topic file, verbatim.",
    "memory.empty": "The memory notebook is empty.",
    "memory.chat": "Update memory with the Agent", "memory.chatHelp": "Ask the Agent to add, correct, or forget durable information.",
    "memory.message": "What should the Agent remember or change?", "memory.send": "Send",
    "memory.changed": "Changed files", "memory.failed": "Memory task failed",
    "memory.thinking": "Updating memory…",
    "memory.loadFailed": "Memory overview could not be generated. Try again.", "memory.errorDetails": "Error details"
  }});
  api.registerLocale({ id: "memory-zh", locale: "zh-CN", entries: {
    "memory.subtitle": "管理长期偏好、反馈、参考信息和可复用经验。",
    "memory.overview": "记忆概览", "memory.refresh": "刷新", "memory.refreshing": "刷新中…",
    "memory.raw": "笔记本原文", "memory.rawHelp": "MEMORY.md 索引与全部主题文件的原始内容。",
    "memory.empty": "记忆笔记本还是空的。",
    "memory.chat": "通过 Agent 更新记忆", "memory.chatHelp": "让 Agent 添加、修正或忘记需要长期保留的信息。",
    "memory.message": "希望 Agent 记住或修改什么？", "memory.send": "发送",
    "memory.changed": "变更文件", "memory.failed": "记忆任务失败",
    "memory.thinking": "正在更新记忆…",
    "memory.loadFailed": "暂时无法生成记忆概览，请重试。", "memory.errorDetails": "错误详情"
  }});
  api.registerStyle({ id: "memory-settings", css: `
    /* The panel speaks the settings-surface language: quiet 12px group
     * labels, content sitting on the canvas, hairlines as separators. */
    .plugin-memory { min-width:0; }
    .plugin-memory-header { justify-content:space-between; align-items:flex-start; gap:16px; }
    .plugin-memory-intro { margin:0; max-width:52ch; color:var(--wuu-color-text-muted,var(--ink-soft)); font-size:var(--font-ui,13px); line-height:1.5; }
    .plugin-memory-refresh { flex:none; }
    .plugin-memory-refresh[data-busy="true"] { pointer-events:none; }
    .plugin-memory-refresh[data-busy="true"] .plugin-memory-refresh-dot { animation:plugin-memory-spin .9s linear infinite; }
    .plugin-memory-refresh-dot { display:inline-block; margin-right:6px; }
    @keyframes plugin-memory-spin { to { transform:rotate(360deg); } }

    /* Overview renders as typeset prose, not a monospace dump: the tiny
     * line parser below promotes "##" leads and "-" items into structure. */
    .plugin-memory-overview { display:flex; flex-direction:column; gap:12px; animation:plugin-memory-fade-up var(--motion-base,160ms) ease both; }
    .plugin-memory-overview h3 { margin:8px 0 0; color:var(--wuu-color-text,var(--ink)); font-size:var(--font-ui,13px); font-weight:var(--weight-semibold,600); line-height:1.4; }
    .plugin-memory-overview h3:first-child { margin-top:0; }
    .plugin-memory-overview p { margin:0; max-width:68ch; color:var(--wuu-color-text,var(--ink)); font-size:var(--font-ui,13px); line-height:1.6; }
    .plugin-memory-overview ul { display:flex; flex-direction:column; gap:4px; margin:0; padding-left:18px; }
    .plugin-memory-overview li { color:var(--wuu-color-text,var(--ink)); font-size:var(--font-ui,13px); line-height:1.55; }
    .plugin-memory-overview li::marker { color:var(--wuu-color-text-muted,var(--ink-muted)); }

    /* Skeleton: quiet gray bars with a slow shimmer while the overview job runs. */
    .plugin-memory-skeleton { display:flex; flex-direction:column; gap:10px; padding:4px 0; }
    .plugin-memory-skeleton-bar { height:10px; border-radius:5px; background:linear-gradient(90deg,var(--surface-2) 25%,var(--surface-3) 50%,var(--surface-2) 75%); background-size:200% 100%; animation:plugin-memory-shimmer 1.4s linear infinite; }
    .plugin-memory-skeleton-bar:nth-child(2) { width:82%; }
    .plugin-memory-skeleton-bar:nth-child(3) { width:64%; }
    .plugin-memory-skeleton-bar:nth-child(4) { width:74%; }
    @keyframes plugin-memory-shimmer { from { background-position:180% 0; } to { background-position:-20% 0; } }

    /* Chat: entries fade up as they land; the user's own messages stay
     * text-only on the right, agent replies read as prose on the left. */
    .plugin-memory-chat-log { display:flex; flex-direction:column; gap:12px; }
    .plugin-memory-chat-entry { display:flex; animation:plugin-memory-fade-up var(--motion-base,160ms) ease both; }
    @keyframes plugin-memory-fade-up { from { opacity:0; transform:translateY(2px); } to { opacity:1; transform:none; } }
    .plugin-memory-chat-entry.user { justify-content:flex-end; }
    .plugin-memory-chat-bubble { max-width:min(72%,420px); padding:8px 12px; border-radius:var(--radius-md,12px); background:var(--wuu-color-surface-muted,var(--surface-2)); color:var(--wuu-color-text,var(--ink)); font-size:var(--font-ui,13px); line-height:1.55; white-space:pre-wrap; overflow-wrap:anywhere; }
    .plugin-memory-chat-reply { max-width:68ch; margin:0; color:var(--wuu-color-text,var(--ink)); font-size:var(--font-ui,13px); line-height:1.6; white-space:pre-wrap; overflow-wrap:anywhere; }
    .plugin-memory-chat-pending { color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:var(--font-sm,12px); }
    .plugin-memory-changes { color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:var(--font-sm,12px); }
    .plugin-memory-changes summary { cursor:pointer; }
    .plugin-memory-changes ul { margin:4px 0 0; padding-left:16px; }
    .plugin-memory-changes code { font-size:var(--font-sm,12px); }
    .plugin-memory-composer { align-items:flex-end; gap:8px; }
    .plugin-memory-composer .plugin-ui-field { flex:1; min-width:0; }
    .plugin-memory-composer textarea { min-height:38px; max-height:140px; }
    .plugin-memory-composer .plugin-ui-button { flex:none; }

    /* Raw notebook: one file per group, separated by full-width hairlines;
     * the verbatim content stays monospace but quiet. */
    .plugin-memory-files { display:flex; flex-direction:column; animation:plugin-memory-fade-up var(--motion-base,160ms) ease both; }
    .plugin-memory-file { display:flex; flex-direction:column; gap:6px; padding:14px 0; border-bottom:1px solid var(--hairline-soft,var(--hairline)); }
    .plugin-memory-file:last-child { border-bottom:0; padding-bottom:0; }
    .plugin-memory-file:first-child { padding-top:0; }
    .plugin-memory-file-head { display:flex; align-items:baseline; gap:8px; min-width:0; }
    .plugin-memory-file-name { color:var(--wuu-color-text,var(--ink)); font-size:var(--font-ui,13px); font-weight:var(--weight-medium,500); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .plugin-memory-file-type { flex:none; padding:1px 8px; border-radius:var(--radius-pill,999px); background:var(--wuu-color-surface-muted,var(--surface-2)); color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:11px; font-weight:var(--weight-medium,500); line-height:1.6; }
    .plugin-memory-file-desc { color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:var(--font-sm,12px); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .plugin-memory-file pre { max-height:220px; margin:0; padding:10px 12px; overflow:auto; border-radius:var(--radius-sm,8px); background:var(--wuu-color-surface,var(--surface-1)); color:var(--wuu-color-text-muted,var(--ink-soft)); white-space:pre-wrap; overflow-wrap:anywhere; font:12px/1.6 var(--wuu-font-mono,ui-monospace,monospace); }

    .plugin-memory-muted { color:var(--wuu-color-text-muted,var(--ink-muted)); font-size:var(--font-sm,12px); }
    .plugin-memory-error { display:grid; gap:6px; padding:10px 12px; border:1px solid color-mix(in srgb,var(--wuu-color-danger,#b42318) 28%,transparent); border-radius:var(--radius-sm,8px); color:var(--wuu-color-danger,#b42318); background:color-mix(in srgb,var(--wuu-color-danger,#b42318) 7%,transparent); font-size:var(--font-ui,13px); overflow-wrap:anywhere; }
    .plugin-memory-error summary { cursor:pointer; color:var(--wuu-color-text-muted,var(--ink-soft)); font-size:var(--font-sm,12px); }
    .plugin-memory-error pre { max-height:160px; margin:4px 0 0; overflow:auto; white-space:pre-wrap; color:var(--wuu-color-text-muted,var(--ink-soft)); font:12px/1.5 var(--wuu-font-mono,ui-monospace,monospace); }
  ` });

  const terminalStates = new Set(["completed", "failed", "interrupted", "discarded"]);
  const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  // Render the overview's markdown-ish text as lightweight structure:
  // "##" lines become group titles, "-" lines list items, the rest prose.
  function OverviewProse({ text }) {
    const blocks = [];
    let list = null;
    const flushList = () => { if (list) { blocks.push({ kind: "ul", items: list }); list = null; } };
    for (const rawLine of String(text).split("\n")) {
      const line = rawLine.trim();
      if (!line) { flushList(); continue; }
      const heading = line.match(/^#{1,3}\s+(.*)$/);
      if (heading) { flushList(); blocks.push({ kind: "h3", text: heading[1] }); continue; }
      const item = line.match(/^[-*]\s+(.*)$/);
      if (item) { (list ||= []).push(item[1]); continue; }
      flushList();
      blocks.push({ kind: "p", text: line });
    }
    flushList();
    return h("div", { className: "plugin-memory-overview" }, blocks.map((block, index) => {
      if (block.kind === "h3") return h("h3", { key: index }, block.text);
      if (block.kind === "ul") return h("ul", { key: index }, block.items.map((item, itemIndex) => h("li", { key: itemIndex }, item)));
      return h("p", { key: index }, block.text);
    }));
  }

  function OverviewSkeleton() {
    return h("div", { className: "plugin-memory-skeleton", "aria-hidden": true },
      h("div", { className: "plugin-memory-skeleton-bar" }),
      h("div", { className: "plugin-memory-skeleton-bar" }),
      h("div", { className: "plugin-memory-skeleton-bar" }),
      h("div", { className: "plugin-memory-skeleton-bar" }));
  }

  function MemorySettings(props) {
    const tr = props.translate;
    const [raw, setRaw] = React.useState({ index_raw: "", files: [] });
    const [overview, setOverview] = React.useState("");
    const [draft, setDraft] = React.useState("");
    const [messages, setMessages] = React.useState([]);
    const [busy, setBusy] = React.useState(false);
    const [chatBusy, setChatBusy] = React.useState(false);
    const [error, setError] = React.useState("");
    const started = React.useRef(false);

    const refreshRaw = React.useCallback(async () => {
      try {
        const value = await api.invokeRuntime("memory.read", {});
        setRaw({ index_raw: typeof value?.index_raw === "string" ? value.index_raw : "", files: Array.isArray(value?.files) ? value.files : [] });
      } catch (reason) { setError(String(reason)); }
    }, []);
    const waitForJob = React.useCallback(async (id) => {
      for (;;) {
        const value = await api.invokeRuntime("memory.job.get", { id });
        if (terminalStates.has(value?.state)) return value;
        await delay(750);
      }
    }, []);
    const refreshOverview = React.useCallback(async () => {
      setBusy(true); setError("");
      try {
        const startedJob = await api.invokeRuntime("memory.overview.start", {});
        const result = await waitForJob(startedJob.id);
        if (result.state !== "completed") throw new Error(result.error || tr("memory.failed"));
        setOverview(result.output || tr("memory.empty"));
        await refreshRaw();
      } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
    }, [refreshRaw, tr, waitForJob]);
    React.useEffect(() => {
      if (started.current) return;
      started.current = true;
      // The raw notebook must render even when the LLM overview fails, so it
      // loads independently instead of being gated behind overview success.
      void refreshRaw();
      void refreshOverview();
    }, [refreshRaw, refreshOverview]);

    const send = async () => {
      const message = draft.trim();
      if (!message || chatBusy) return;
      setDraft(""); setMessages((current) => [...current, { role: "user", text: message }]); setChatBusy(true); setError("");
      try {
        const startedJob = await api.invokeRuntime("memory.chat.start", { message });
        const result = await waitForJob(startedJob.id);
        if (result.state !== "completed") throw new Error(result.error || tr("memory.failed"));
        setMessages((current) => [...current, { role: "assistant", text: result.output || "", changed: result.changed_files || [] }]);
        await refreshRaw();
        await refreshOverview();
      } catch (reason) { setError(String(reason)); } finally { setChatBusy(false); }
    };

    const files = [];
    if (raw.index_raw) files.push({ name: "MEMORY.md", content: raw.index_raw });
    files.push(...raw.files);
    return h(Page, { className: "plugin-memory" }, h(Stack, { gap: "large" },
      h(Row, { className: "plugin-memory-header" },
        h("p", { className: "plugin-memory-intro" }, tr("memory.subtitle")),
        h(Button, { className: "plugin-memory-refresh", variant: "ghost", disabled: busy, "data-busy": busy ? "true" : "false", onClick: () => { void refreshRaw(); void refreshOverview(); } },
          h("span", { className: "plugin-memory-refresh-dot", "aria-hidden": true }, "↻"),
          tr(busy ? "memory.refreshing" : "memory.refresh"))),
      h(Section, { title: tr("memory.overview") },
        busy && !overview ? h(OverviewSkeleton) : h(OverviewProse, { text: overview || tr("memory.empty") })),
      h(Section, { title: tr("memory.chat"), description: tr("memory.chatHelp") }, h(Stack, null,
        messages.length || chatBusy ? h("div", { className: "plugin-memory-chat-log" },
          messages.map((entry, index) => h("div", { key: index, className: `plugin-memory-chat-entry ${entry.role}` },
            entry.role === "user"
              ? h("div", { className: "plugin-memory-chat-bubble" }, entry.text)
              : h("div", null,
                h("p", { className: "plugin-memory-chat-reply" }, entry.text),
                entry.changed?.length ? h("details", { className: "plugin-memory-changes" },
                  h("summary", null, `${tr("memory.changed")} · ${entry.changed.length}`),
                  h("ul", null, entry.changed.map((item, itemIndex) => h("li", { key: itemIndex }, h("code", null, item.path))))) : null))),
          chatBusy ? h("div", { className: "plugin-memory-chat-entry" }, h("span", { className: "plugin-memory-chat-pending" }, tr("memory.thinking"))) : null) : null,
        h(Row, { className: "plugin-memory-composer" },
          h(TextArea, { label: tr("memory.message"), value: draft, disabled: chatBusy, onChange: (event) => setDraft(event.target.value), onKeyDown: (event) => { if ((event.metaKey || event.ctrlKey) && event.key === "Enter") void send(); } }),
          h(Button, { variant: "primary", disabled: chatBusy || !draft.trim(), onClick: () => void send() }, tr("memory.send"))))),
      error ? h("div", { className: "plugin-memory-error", role: "alert" }, h("strong", null, tr("memory.loadFailed")), h("details", null, h("summary", null, tr("memory.errorDetails")), h("pre", null, error))) : null,
      h(Section, { title: tr("memory.raw"), description: tr("memory.rawHelp") },
        files.length ? h("div", { className: "plugin-memory-files" }, files.map((file) => h("article", { className: "plugin-memory-file", key: file.name },
          h("div", { className: "plugin-memory-file-head" },
            h("span", { className: "plugin-memory-file-name" }, file.name),
            file.type ? h("span", { className: "plugin-memory-file-type" }, file.type) : null,
            file.description ? h("span", { className: "plugin-memory-file-desc" }, file.description) : null),
          h("pre", null, file.content)))) : h(EmptyState, { title: tr("memory.empty") }))
    ));
  }

  api.registerViewType({ id: "memory.settings", title: "Memory", icon: "brain", defaultRegion: "settings", persistence: "durable", render: (props) => h(MemorySettings, props) });
}
