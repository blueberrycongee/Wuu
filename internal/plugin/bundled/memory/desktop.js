export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Page, Panel, Section, Stack, Row, Button, TextArea, EmptyState } = api.ui;

  api.registerLocale({ id: "memory-en", locale: "en-US", entries: {
    "memory.subtitle": "Durable preferences, feedback, references, and reusable lessons.",
    "memory.overview": "Memory overview", "memory.refresh": "Refresh overview", "memory.raw": "Notebook files",
    "memory.empty": "The memory notebook is empty.", "memory.loading": "Reviewing memory…",
    "memory.chat": "Update memory with the Agent", "memory.chatHelp": "Ask the Agent to add, correct, or forget durable information.",
    "memory.message": "What should the Agent remember or change?", "memory.send": "Send",
    "memory.changed": "Changed files", "memory.failed": "Memory task failed"
  }});
  api.registerLocale({ id: "memory-zh", locale: "zh-CN", entries: {
    "memory.subtitle": "管理长期偏好、反馈、参考信息和可复用经验。",
    "memory.overview": "记忆概览", "memory.refresh": "刷新概览", "memory.raw": "笔记本原文",
    "memory.empty": "记忆笔记本还是空的。", "memory.loading": "正在整理记忆…",
    "memory.chat": "通过 Agent 更新记忆", "memory.chatHelp": "让 Agent 添加、修正或忘记需要长期保留的信息。",
    "memory.message": "希望 Agent 记住或修改什么？", "memory.send": "发送",
    "memory.changed": "变更文件", "memory.failed": "记忆任务失败"
  }});
  api.registerStyle({ id: "memory-settings", css: `
    .plugin-memory { padding-top: 8px; }
    .plugin-memory-intro { margin:0; color:var(--wuu-color-text-muted,var(--ink-soft)); }
    .plugin-memory-toolbar { justify-content:space-between; }
    .plugin-memory-output,.plugin-memory-file pre { margin:0; white-space:pre-wrap; overflow-wrap:anywhere; font:13px/1.65 var(--wuu-font-mono,ui-monospace,monospace); }
    .plugin-memory-messages { display:grid; gap:8px; }
    .plugin-memory-message { padding:10px 12px; border-radius:var(--wuu-radius-control,var(--radius-sm)); background:var(--wuu-color-surface-muted,var(--surface-2)); }
    .plugin-memory-message[data-role="user"] { margin-left:clamp(12px,8%,72px); }
    .plugin-memory-file { display:grid; gap:6px; padding-top:12px; border-top:1px solid var(--wuu-color-border-subtle,var(--hairline)); }
    .plugin-memory-file:first-child { padding-top:0; border-top:0; }
    .plugin-memory-muted { color:var(--wuu-color-text-muted,var(--ink-soft)); font-size:13px; }
    .plugin-memory-error { color:var(--wuu-color-danger,var(--danger)); font-size:13px; }
  ` });

  const terminalStates = new Set(["completed", "failed", "interrupted", "discarded"]);
  const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  function MemorySettings(props) {
    const tr = props.translate;
    const [raw, setRaw] = React.useState({ index_raw:"", files:[] });
    const [overview, setOverview] = React.useState("");
    const [draft, setDraft] = React.useState("");
    const [messages, setMessages] = React.useState([]);
    const [busy, setBusy] = React.useState(false);
    const [error, setError] = React.useState("");
    const started = React.useRef(false);

    const refreshRaw = React.useCallback(async () => {
      const value = await api.invokeRuntime("memory.read", {});
      setRaw({ index_raw:typeof value?.index_raw === "string" ? value.index_raw : "", files:Array.isArray(value?.files) ? value.files : [] });
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
      void refreshOverview();
    }, [refreshOverview]);

    const send = async () => {
      const message = draft.trim();
      if (!message || busy) return;
      setDraft(""); setMessages((current)=>[...current,{role:"user",text:message}]); setBusy(true); setError("");
      try {
        const startedJob = await api.invokeRuntime("memory.chat.start", { message });
        const result = await waitForJob(startedJob.id);
        if (result.state !== "completed") throw new Error(result.error || tr("memory.failed"));
        setMessages((current)=>[...current,{role:"assistant",text:result.output || "",changed:result.changed_files || []}]);
        await refreshRaw();
        await refreshOverview();
      } catch (reason) { setError(String(reason)); setBusy(false); }
    };

    const files = [];
    if (raw.index_raw) files.push({ name:"MEMORY.md", content:raw.index_raw });
    files.push(...raw.files);
    return h(Page,{className:"plugin-memory"},h(Stack,{gap:"large"},
      h(Row,{className:"plugin-memory-toolbar"},h("p",{className:"plugin-memory-intro"},tr("memory.subtitle")),h(Button,{disabled:busy,onClick:()=>void refreshOverview()},tr("memory.refresh"))),
      h(Panel,null,h(Section,{title:tr("memory.overview")},busy&&!overview?h("div",{className:"plugin-memory-muted"},tr("memory.loading")):h("div",{className:"plugin-memory-output"},overview||tr("memory.empty")))),
      h(Panel,null,h(Section,{title:tr("memory.chat"),description:tr("memory.chatHelp")},h(Stack,null,
        messages.length?h("div",{className:"plugin-memory-messages"},messages.map((entry,index)=>h("div",{key:index,className:"plugin-memory-message","data-role":entry.role},entry.text,entry.changed?.length?h("div",{className:"plugin-memory-muted"},`${tr("memory.changed")}: ${entry.changed.map((item)=>item.path).join(", ")}`):null))):null,
        h(TextArea,{label:tr("memory.message"),value:draft,disabled:busy,onChange:(event)=>setDraft(event.target.value)}),
        h(Row,null,h(Button,{variant:"primary",disabled:busy||!draft.trim(),onClick:()=>void send()},tr("memory.send")))
      ))),
      error?h("div",{className:"plugin-memory-error",role:"alert"},error):null,
      h(Panel,null,h(Section,{title:tr("memory.raw")},files.length?h(Stack,null,files.map((file)=>h("article",{className:"plugin-memory-file",key:file.name},h("strong",null,file.name),file.type||file.description?h("div",{className:"plugin-memory-muted"},[file.type,file.description].filter(Boolean).join(" · ")):null,h("pre",null,file.content)))):h(EmptyState,{title:tr("memory.empty")})))
    ));
  }

  api.registerViewType({ id:"memory.settings", title:"Memory", icon:"brain", defaultPane:"main", persistence:"durable", render:(props)=>h(MemorySettings,props) });
}
