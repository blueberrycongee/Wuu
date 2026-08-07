export async function activate(api) {
  const React = api.react;
  const h = React.createElement;

  api.registerLocale({ id: "memory-en", locale: "en-US", entries: {
    "memory.title": "Memory", "memory.subtitle": "Durable preferences, feedback, references, and lessons",
    "memory.overview": "Overview", "memory.refresh": "Refresh overview", "memory.raw": "Notebook files",
    "memory.empty": "The memory notebook is empty.", "memory.loading": "Reviewing memory…",
    "memory.chat": "Ask the Agent to add, correct, or forget a memory", "memory.send": "Send",
    "memory.changed": "Changed files", "memory.failed": "Memory task failed"
  }});
  api.registerLocale({ id: "memory-zh", locale: "zh-CN", entries: {
    "memory.title": "记忆", "memory.subtitle": "长期偏好、反馈、参考信息和可复用经验",
    "memory.overview": "概览", "memory.refresh": "刷新概览", "memory.raw": "笔记本原文",
    "memory.empty": "记忆笔记本还是空的。", "memory.loading": "正在整理记忆…",
    "memory.chat": "让 Agent 添加、修正或忘记一条记忆", "memory.send": "发送",
    "memory.changed": "变更文件", "memory.failed": "记忆任务失败"
  }});
  api.registerStyle({ id: "memory-settings", css: `
    .plugin-memory { height:100%; overflow:auto; padding:28px; color:var(--wuu-color-text); background:var(--wuu-color-canvas); }
    .plugin-memory-inner { width:min(860px,100%); margin:0 auto; display:grid; gap:18px; }
    .plugin-memory-header { display:flex; align-items:flex-start; justify-content:space-between; gap:16px; }
    .plugin-memory-header h1 { margin:0; font-size:28px; }
    .plugin-memory-muted { color:var(--wuu-color-text-muted); font-size:13px; }
    .plugin-memory-panel { display:grid; gap:12px; padding:16px; border:1px solid var(--wuu-color-border-subtle); border-radius:var(--wuu-radius-panel); background:var(--wuu-color-surface); }
    .plugin-memory-panel h2 { margin:0; font-size:16px; }
    .plugin-memory-output,.plugin-memory-file pre { margin:0; white-space:pre-wrap; overflow-wrap:anywhere; font:13px/1.6 var(--wuu-font-mono,ui-monospace,monospace); }
    .plugin-memory-chat { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:8px; }
    .plugin-memory textarea { min-height:72px; resize:vertical; padding:9px 11px; border:1px solid var(--wuu-color-border-subtle); border-radius:var(--wuu-radius-control); color:var(--wuu-color-text); background:var(--wuu-color-surface-muted); font:inherit; }
    .plugin-memory button { align-self:start; padding:7px 11px; border:1px solid var(--wuu-color-border-subtle); border-radius:var(--wuu-radius-control); color:var(--wuu-color-text); background:var(--wuu-color-surface-muted); cursor:pointer; }
    .plugin-memory button:disabled { opacity:.5; cursor:default; }
    .plugin-memory-files { display:grid; gap:8px; }
    .plugin-memory-file { display:grid; gap:6px; padding-top:10px; border-top:1px solid var(--wuu-color-border-subtle); }
    .plugin-memory-file:first-child { padding-top:0; border-top:0; }
    .plugin-memory-error { color:var(--wuu-color-danger); font-size:13px; }
  ` });

  const terminalStates = new Set(["completed", "failed", "interrupted", "discarded"]);
  const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  function MemorySettings(props) {
    const translate = typeof props.translate === "function" ? props.translate : (key) => key;
    const tr = (key) => { const value = translate(key); return typeof value === "string" && value !== key ? value : key.split(".").pop(); };
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
    }, [refreshRaw, waitForJob]);
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

    return h("main",{className:"plugin-memory"},h("div",{className:"plugin-memory-inner"},
      h("header",{className:"plugin-memory-header"},h("div",null,h("h1",null,tr("memory.title")),h("div",{className:"plugin-memory-muted"},tr("memory.subtitle"))),h("button",{disabled:busy,onClick:()=>void refreshOverview()},tr("memory.refresh"))),
      h("section",{className:"plugin-memory-panel"},h("h2",null,tr("memory.overview")),busy&&!overview?h("div",{className:"plugin-memory-muted"},tr("memory.loading")):h("div",{className:"plugin-memory-output"},overview||tr("memory.empty"))),
      h("section",{className:"plugin-memory-panel"},h("h2",null,tr("memory.chat")),messages.map((entry,index)=>h("div",{key:index,className:entry.role==="assistant"?"plugin-memory-output":"plugin-memory-muted"},entry.text,entry.changed?.length?h("div",{className:"plugin-memory-muted"},`${tr("memory.changed")}: ${entry.changed.map((item)=>item.path).join(", ")}`):null)),h("div",{className:"plugin-memory-chat"},h("textarea",{value:draft,disabled:busy,onChange:(event)=>setDraft(event.target.value)}),h("button",{disabled:busy||!draft.trim(),onClick:()=>void send()},tr("memory.send")))),
      error?h("div",{className:"plugin-memory-error",role:"alert"},error):null,
      h("section",{className:"plugin-memory-panel"},h("h2",null,tr("memory.raw")),!raw.index_raw&&!raw.files.length?h("div",{className:"plugin-memory-muted"},tr("memory.empty")):h("div",{className:"plugin-memory-files"},raw.index_raw?h("article",{className:"plugin-memory-file"},h("strong",null,"MEMORY.md"),h("pre",null,raw.index_raw)):null,raw.files.map((file)=>h("article",{className:"plugin-memory-file",key:file.name},h("strong",null,file.name),h("div",{className:"plugin-memory-muted"},[file.type,file.description].filter(Boolean).join(" · ")),h("pre",null,file.content)))))
    ));
  }

  api.registerViewType({ id:"memory.settings", title:"Memory", icon:"brain", defaultPane:"main", persistence:"durable", render:(props)=>h(MemorySettings,props) });
}
