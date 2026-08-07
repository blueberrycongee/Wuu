export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  api.registerLocale({ id:"dream-en", locale:"en-US", entries:{
    "dream.title":"Dream", "dream.enabled":"Background consolidation", "dream.interval":"Interval (days)",
    "dream.minimum":"Completed sessions before a run", "dream.model":"Model alias (optional)",
    "dream.save":"Save", "dream.run":"Run now", "dream.candidates":"Pending sessions", "dream.status":"Last status"
  }});
  api.registerLocale({ id:"dream-zh", locale:"zh-CN", entries:{
    "dream.title":"Dream", "dream.enabled":"后台记忆整合", "dream.interval":"运行间隔（天）",
    "dream.minimum":"触发前累计完成会话数", "dream.model":"模型别名（可选）",
    "dream.save":"保存", "dream.run":"立即运行", "dream.candidates":"待整理会话", "dream.status":"上次状态"
  }});
  api.registerStyle({ id:"dream-settings", css:`
    .plugin-dream{height:100%;overflow:auto;padding:28px;color:var(--wuu-color-text);background:var(--wuu-color-canvas)}
    .plugin-dream-inner{width:min(760px,100%);margin:0 auto;display:grid;gap:16px}.plugin-dream-card{display:grid;gap:12px;padding:16px;border:1px solid var(--wuu-color-border-subtle);border-radius:var(--wuu-radius-panel);background:var(--wuu-color-surface)}
    .plugin-dream-row{display:grid;grid-template-columns:minmax(180px,1fr) minmax(180px,280px);align-items:center;gap:12px}.plugin-dream input{box-sizing:border-box;width:100%;padding:8px 10px;border:1px solid var(--wuu-color-border-subtle);border-radius:var(--wuu-radius-control);color:var(--wuu-color-text);background:var(--wuu-color-surface-muted)}
    .plugin-dream-actions{display:flex;gap:8px}.plugin-dream button{padding:7px 11px;border:1px solid var(--wuu-color-border-subtle);border-radius:var(--wuu-radius-control);color:var(--wuu-color-text);background:var(--wuu-color-surface-muted);cursor:pointer}.plugin-dream-muted{color:var(--wuu-color-text-muted);font-size:13px}.plugin-dream-error{color:var(--wuu-color-danger);font-size:13px}
  `});
  function DreamSettings(props) {
    const translate=typeof props.translate==="function"?props.translate:(key)=>key;
    const tr=(key)=>{const value=translate(key);return typeof value==="string"&&value!==key?value:key.split(".").pop()};
    const [state,setState]=React.useState(null);const [draft,setDraft]=React.useState(null);const [busy,setBusy]=React.useState(false);const [error,setError]=React.useState("");
    const refresh=React.useCallback(async()=>{const value=await api.invokeRuntime("dream.get",{});setState(value);setDraft(value.settings)},[]);
    React.useEffect(()=>{void refresh().catch((reason)=>setError(String(reason)))},[refresh]);
    const act=async(method,input)=>{setBusy(true);setError("");try{const value=await api.invokeRuntime(method,input);setState(value);setDraft(value.settings)}catch(reason){setError(String(reason))}finally{setBusy(false)}};
    if(!draft)return h("main",{className:"plugin-dream"},h("div",{className:"plugin-dream-muted"},"…"));
    const number=(event)=>Number.parseInt(event.target.value,10)||0;
    return h("main",{className:"plugin-dream"},h("div",{className:"plugin-dream-inner"},h("h1",null,tr("dream.title")),
      h("section",{className:"plugin-dream-card"},
        h("label",{className:"plugin-dream-row"},h("span",null,tr("dream.enabled")),h("input",{type:"checkbox",checked:draft.enabled,onChange:(event)=>setDraft({...draft,enabled:event.target.checked})})),
        h("label",{className:"plugin-dream-row"},h("span",null,tr("dream.interval")),h("input",{type:"number",min:1,max:365,value:draft.interval_days,onChange:(event)=>setDraft({...draft,interval_days:number(event)})})),
        h("label",{className:"plugin-dream-row"},h("span",null,tr("dream.minimum")),h("input",{type:"number",min:1,max:100,value:draft.min_sessions,onChange:(event)=>setDraft({...draft,min_sessions:number(event)})})),
        h("label",{className:"plugin-dream-row"},h("span",null,tr("dream.model")),h("input",{value:draft.model_alias||"",onChange:(event)=>setDraft({...draft,model_alias:event.target.value})})),
        h("div",{className:"plugin-dream-actions"},h("button",{disabled:busy,onClick:()=>void act("dream.update",draft)},tr("dream.save")),h("button",{disabled:busy||!draft.enabled||Object.keys(state?.candidates||{}).length===0,onClick:()=>void act("dream.run",{})},tr("dream.run")))
      ),error?h("div",{className:"plugin-dream-error",role:"alert"},error):null,
      h("section",{className:"plugin-dream-card"},h("div",null,`${tr("dream.candidates")}: ${Object.keys(state?.candidates||{}).length}`),h("div",{className:"plugin-dream-muted"},`${tr("dream.status")}: ${state?.last_status||"—"}${state?.last_error?` · ${state.last_error}`:""}`))
    ));
  }
  api.registerViewType({id:"dream.settings",title:"Dream",icon:"moon",defaultPane:"main",persistence:"durable",render:(props)=>h(DreamSettings,props)});
}
