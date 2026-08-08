export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Page, Panel, Card, Section, Stack, Row, Button, TextInput, Checkbox } = api.ui;
  api.registerLocale({ id:"dream-en", locale:"en-US", entries:{
    "dream.description":"Consolidate completed sessions into durable workspace memory in the background.",
    "dream.enabled":"Enable background consolidation", "dream.enabledHelp":"Dream runs only while Wuu is open.",
    "dream.interval":"Interval (days)", "dream.minimum":"Completed sessions before a run", "dream.model":"Model alias (optional)",
    "dream.save":"Save changes", "dream.run":"Run now", "dream.activity":"Activity",
    "dream.candidates":"Pending sessions", "dream.status":"Last status"
  }});
  api.registerLocale({ id:"dream-zh", locale:"zh-CN", entries:{
    "dream.description":"在后台把已完成会话整合为工作区长期记忆。",
    "dream.enabled":"启用后台记忆整合", "dream.enabledHelp":"仅在 Wuu 运行期间执行。",
    "dream.interval":"运行间隔（天）", "dream.minimum":"触发前累计完成会话数", "dream.model":"模型别名（可选）",
    "dream.save":"保存更改", "dream.run":"立即运行", "dream.activity":"运行情况",
    "dream.candidates":"待整理会话", "dream.status":"上次状态"
  }});
  api.registerStyle({ id:"dream-settings", css:`
    .plugin-dream { padding-top:8px; }
    .plugin-dream-intro { margin:0; color:var(--wuu-color-text-muted,var(--ink-soft)); }
    .plugin-dream-actions { justify-content:flex-start; }
    .plugin-dream-status { font-size:13px; color:var(--wuu-color-text-muted,var(--ink-soft)); }
    .plugin-dream-error { color:var(--wuu-color-danger,var(--danger)); font-size:13px; }
  `});
  function DreamSettings(props) {
    const tr=props.translate;
    const [state,setState]=React.useState(null);const [draft,setDraft]=React.useState(null);const [busy,setBusy]=React.useState(false);const [error,setError]=React.useState("");
    const refresh=React.useCallback(async()=>{const value=await api.invokeRuntime("dream.get",{});setState(value);setDraft(value.settings)},[]);
    React.useEffect(()=>{void refresh().catch((reason)=>setError(String(reason)))},[refresh]);
    const act=async(method,input)=>{setBusy(true);setError("");try{const value=await api.invokeRuntime(method,input);setState(value);setDraft(value.settings)}catch(reason){setError(String(reason))}finally{setBusy(false)}};
    if(!draft)return h(Page,{className:"plugin-dream"},h("div",{className:"plugin-dream-status"},"…"));
    const number=(event)=>Number.parseInt(event.target.value,10)||0;
    return h(Page,{className:"plugin-dream"},h(Stack,{gap:"large"},
      h("p",{className:"plugin-dream-intro"},tr("dream.description")),
      h(Panel,null,h(Stack,null,
        h(Checkbox,{label:tr("dream.enabled"),description:tr("dream.enabledHelp"),checked:draft.enabled,onChange:(event)=>setDraft({...draft,enabled:event.target.checked})}),
        h(TextInput,{type:"number",min:1,max:365,label:tr("dream.interval"),value:draft.interval_days,onChange:(event)=>setDraft({...draft,interval_days:number(event)})}),
        h(TextInput,{type:"number",min:1,max:100,label:tr("dream.minimum"),value:draft.min_sessions,onChange:(event)=>setDraft({...draft,min_sessions:number(event)})}),
        h(TextInput,{label:tr("dream.model"),value:draft.model_alias||"",onChange:(event)=>setDraft({...draft,model_alias:event.target.value})}),
        h(Row,{className:"plugin-dream-actions"},h(Button,{variant:"primary",disabled:busy,onClick:()=>void act("dream.update",draft)},tr("dream.save")),h(Button,{disabled:busy||!draft.enabled||Object.keys(state?.candidates||{}).length===0,onClick:()=>void act("dream.run",{})},tr("dream.run")))
      )),
      error?h("div",{className:"plugin-dream-error",role:"alert"},error):null,
      h(Card,null,h(Section,{title:tr("dream.activity")},h(Stack,{gap:"small"},h("div",null,`${tr("dream.candidates")}: ${Object.keys(state?.candidates||{}).length}`),h("div",{className:"plugin-dream-status"},`${tr("dream.status")}: ${state?.last_status||"—"}${state?.last_error?` · ${state.last_error}`:""}`))))
    ));
  }
  api.registerViewType({id:"dream.settings",title:"Dream",icon:"moon",defaultPane:"main",persistence:"durable",render:(props)=>h(DreamSettings,props)});
}
