export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Page, Panel, Card, Section, Stack, Row, Button, TextInput, TextArea, Checkbox, EmptyState } = api.ui;

  api.registerLocale({ id: "automation-en", locale: "en-US", entries: {
    "automation.title": "Automations", "automation.new": "New automation", "automation.empty": "No automations yet",
    "automation.name": "Name", "automation.prompt": "Prompt", "automation.schedule": "Cron schedule", "automation.timezone": "Timezone",
    "automation.description": "Wake the main agent on a schedule with a saved prompt.",
    "automation.recurring": "Repeat", "automation.recurringHelp": "Keep scheduling future runs after this automation fires.",
    "automation.create": "Create automation", "automation.cancel": "Cancel", "automation.pause": "Pause", "automation.resume": "Resume", "automation.remove": "Remove"
  }});
  api.registerLocale({ id: "automation-zh", locale: "zh-CN", entries: {
    "automation.title": "自动化", "automation.new": "新建自动化", "automation.empty": "还没有自动化任务",
    "automation.name": "名称", "automation.prompt": "提示词", "automation.schedule": "Cron 时间", "automation.timezone": "时区",
    "automation.description": "按设定时间用保存的提示词唤醒主 Agent。",
    "automation.recurring": "重复执行", "automation.recurringHelp": "任务触发后继续安排下一次运行。",
    "automation.create": "创建自动化", "automation.cancel": "取消", "automation.pause": "暂停", "automation.resume": "继续", "automation.remove": "删除"
  }});
  api.registerStyle({ id: "automation-catalog", css: `
    .plugin-automation { height:100%; overflow:auto; color:var(--wuu-color-text, var(--ink, #181818)); background:var(--wuu-color-canvas, var(--paper, #fff)); }
    .plugin-automation-intro { flex:1; min-width:220px; color:var(--wuu-color-text-muted, var(--ink-muted, #666)); line-height:1.5; }
    .plugin-automation-list { display:grid; gap:10px; }
    .plugin-automation-row { display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
    .plugin-automation-card strong { flex:1; min-width:180px; }
    .plugin-automation-meta { color:var(--wuu-color-text-muted, var(--ink-muted, #666)); font-size:12px; }
    .plugin-automation-prompt { white-space:pre-wrap; line-height:1.5; }
    .plugin-automation-error { color:var(--wuu-color-danger, #b42318); font-size:12px; }
  ` });

  function Catalog(props) {
    const tr = props.translate;
    const [tasks, setTasks] = React.useState([]);
    const [creating, setCreating] = React.useState(false);
    const [busy, setBusy] = React.useState(false);
    const [error, setError] = React.useState("");
    const [draft, setDraft] = React.useState({ title:"", prompt:"", schedule:"0 9 * * 1-5", timezone:Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC", mode:"new_thread", recurring:true });
    const refresh = React.useCallback(() => api.invokeRuntime("automation.list", {}).then((value) => setTasks(Array.isArray(value?.tasks) ? value.tasks : [])).catch((reason) => setError(String(reason))), []);
    React.useEffect(() => { void refresh(); }, [refresh]);
    const act = async (method, input) => { setBusy(true); setError(""); try { await api.invokeRuntime(method,input); await refresh(); } catch (reason) { setError(String(reason)); } finally { setBusy(false); } };
    return h("main", { className:"plugin-automation" }, h(Page, null,
      h(Row, null,
        h("p", { className:"plugin-automation-intro" }, tr("automation.description")),
        h(Button, { variant:"primary", onClick:()=>setCreating(!creating) }, tr(creating ? "automation.cancel" : "automation.new"))
      ),
      creating ? h(Panel, null, h(Stack, null,
        h(TextInput, { label:tr("automation.name"), value:draft.title, onChange:(event)=>setDraft({...draft,title:event.target.value}) }),
        h(TextArea, { label:tr("automation.prompt"), value:draft.prompt, onChange:(event)=>setDraft({...draft,prompt:event.target.value}) }),
        h(Row, null,
          h(TextInput, { label:tr("automation.schedule"), value:draft.schedule, onChange:(event)=>setDraft({...draft,schedule:event.target.value}) }),
          h(TextInput, { label:tr("automation.timezone"), value:draft.timezone, onChange:(event)=>setDraft({...draft,timezone:event.target.value}) })
        ),
        h(Checkbox, { label:tr("automation.recurring"), description:tr("automation.recurringHelp"), checked:draft.recurring, onChange:(event)=>setDraft({...draft,recurring:event.target.checked}) }),
        h(Row, null, h(Button, { variant:"primary", disabled:busy||!draft.prompt.trim(), onClick:async()=>{await act("automation.create",{...draft,durable:true});setCreating(false);} }, tr("automation.create")))
      )) : null,
      error ? h("div", { className:"plugin-automation-error", role:"alert" }, error) : null,
      tasks.length===0 ? h(EmptyState, { title:tr("automation.empty"), description:tr("automation.description") }) : h(Section, { className:"plugin-automation-list" }, tasks.map((task)=>h(Card, { className:"plugin-automation-card", key:task.id },
        h(Stack, null,
          h("div", { className:"plugin-automation-row" },
            h("strong", null, task.title||task.prompt),
            h(Button, { disabled:busy, onClick:()=>act("automation.update",{...task,paused:!task.paused}) }, tr(task.paused?"automation.resume":"automation.pause")),
            h(Button, { disabled:busy, onClick:()=>act("automation.remove",{id:task.id}) }, tr("automation.remove"))
          ),
          h("div", { className:"plugin-automation-meta" }, `${task.cron} · ${task.timezone} · ${task.mode}`),
          h("div", { className:"plugin-automation-prompt" }, task.prompt)
        )
      )))
    ));
  }

  api.registerViewType({ id:"automation.catalog", title:"Automations", icon:"clock", defaultPane:"main", persistence:"durable", render:(props)=>h(Catalog,props) });
}
