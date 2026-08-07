export async function activate(api) {
  const React = api.react;
  const h = React.createElement;

  api.registerLocale({ id: "automation-en", locale: "en-US", entries: {
    "automation.title": "Automations", "automation.new": "New automation", "automation.empty": "No automations yet",
    "automation.name": "Name", "automation.prompt": "Prompt", "automation.schedule": "Cron schedule", "automation.timezone": "Timezone",
    "automation.recurring": "Repeat", "automation.create": "Create", "automation.pause": "Pause", "automation.resume": "Resume", "automation.remove": "Remove"
  }});
  api.registerLocale({ id: "automation-zh", locale: "zh-CN", entries: {
    "automation.title": "自动化", "automation.new": "新建自动化", "automation.empty": "还没有自动化任务",
    "automation.name": "名称", "automation.prompt": "提示词", "automation.schedule": "Cron 时间", "automation.timezone": "时区",
    "automation.recurring": "重复执行", "automation.create": "创建", "automation.pause": "暂停", "automation.resume": "继续", "automation.remove": "删除"
  }});
  api.registerStyle({ id: "automation-catalog", css: `
    .plugin-automation { height:100%; overflow:auto; padding:24px; color:var(--wuu-color-text); background:var(--wuu-color-canvas); }
    .plugin-automation-inner { width:min(900px,100%); margin:0 auto; display:grid; gap:18px; }
    .plugin-automation-header { display:flex; justify-content:space-between; align-items:center; gap:12px; }
    .plugin-automation-list { display:grid; gap:10px; }
    .plugin-automation-card,.plugin-automation-form { display:grid; gap:10px; padding:14px; border:1px solid var(--wuu-color-border-subtle); border-radius:var(--wuu-radius-panel); background:var(--wuu-color-surface); }
    .plugin-automation-row { display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
    .plugin-automation-card strong { flex:1; min-width:180px; }
    .plugin-automation-meta { color:var(--wuu-color-text-muted); font-size:12px; }
    .plugin-automation-form input,.plugin-automation-form textarea,.plugin-automation-form select { width:100%; box-sizing:border-box; padding:8px 10px; border:1px solid var(--wuu-color-border-subtle); border-radius:var(--wuu-radius-control); color:var(--wuu-color-text); background:var(--wuu-color-surface-muted); font:inherit; }
    .plugin-automation-form textarea { min-height:100px; resize:vertical; }
    .plugin-automation button { padding:6px 10px; border:1px solid var(--wuu-color-border-subtle); border-radius:var(--wuu-radius-control); color:var(--wuu-color-text); background:var(--wuu-color-surface-muted); cursor:pointer; }
    .plugin-automation button:disabled { opacity:.5; cursor:default; }
    .plugin-automation-error { color:var(--wuu-color-danger); font-size:12px; }
  ` });

  function Catalog(props) {
    const translate = typeof props.translate === "function" ? props.translate : (key) => key.split(".").pop();
    const tr = (key) => { const value = translate(key); return typeof value === "string" && value !== key ? value : key.split(".").pop(); };
    const [tasks, setTasks] = React.useState([]);
    const [creating, setCreating] = React.useState(false);
    const [busy, setBusy] = React.useState(false);
    const [error, setError] = React.useState("");
    const [draft, setDraft] = React.useState({ title:"", prompt:"", schedule:"0 9 * * 1-5", timezone:Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC", mode:"new_thread", recurring:true });
    const refresh = React.useCallback(() => api.invokeRuntime("automation.list", {}).then((value) => setTasks(Array.isArray(value?.tasks) ? value.tasks : [])).catch((reason) => setError(String(reason))), []);
    React.useEffect(() => { void refresh(); }, [refresh]);
    const act = async (method, input) => { setBusy(true); setError(""); try { await api.invokeRuntime(method,input); await refresh(); } catch (reason) { setError(String(reason)); } finally { setBusy(false); } };
    return h("main", { className:"plugin-automation" }, h("div", { className:"plugin-automation-inner" },
      h("header", { className:"plugin-automation-header" }, h("h1",null,tr("automation.title")), h("button",{onClick:()=>setCreating(!creating)},tr("automation.new"))),
      creating ? h("section",{className:"plugin-automation-form"},
        h("input",{value:draft.title,placeholder:tr("automation.name"),onChange:(event)=>setDraft({...draft,title:event.target.value})}),
        h("textarea",{value:draft.prompt,placeholder:tr("automation.prompt"),onChange:(event)=>setDraft({...draft,prompt:event.target.value})}),
        h("div",{className:"plugin-automation-row"},h("input",{value:draft.schedule,placeholder:tr("automation.schedule"),onChange:(event)=>setDraft({...draft,schedule:event.target.value})}),h("input",{value:draft.timezone,placeholder:tr("automation.timezone"),onChange:(event)=>setDraft({...draft,timezone:event.target.value})})),
        h("label",null,h("input",{type:"checkbox",checked:draft.recurring,onChange:(event)=>setDraft({...draft,recurring:event.target.checked})})," ",tr("automation.recurring")),
        h("button",{disabled:busy||!draft.prompt.trim(),onClick:async()=>{await act("automation.create",{...draft,durable:true});setCreating(false);}},tr("automation.create"))
      ):null,
      error?h("div",{className:"plugin-automation-error",role:"alert"},error):null,
      tasks.length===0?h("div",{className:"plugin-automation-meta"},tr("automation.empty")):h("section",{className:"plugin-automation-list"},tasks.map((task)=>h("article",{className:"plugin-automation-card",key:task.id},
        h("div",{className:"plugin-automation-row"},h("strong",null,task.title||task.prompt),h("button",{disabled:busy,onClick:()=>act("automation.update",{...task,paused:!task.paused})},tr(task.paused?"automation.resume":"automation.pause")),h("button",{disabled:busy,onClick:()=>act("automation.remove",{id:task.id})},tr("automation.remove"))),
        h("div",{className:"plugin-automation-meta"},`${task.cron} · ${task.timezone} · ${task.mode}`),h("div",null,task.prompt)
      )))
    ));
  }

  api.registerViewType({ id:"automation.catalog", title:"Automations", icon:"clock", defaultPane:"main", persistence:"durable", render:(props)=>h(Catalog,props) });
}
