export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Page, Section, Stack, Row, Button } = api.ui;

  api.registerLocale({ id: "delivery-en", locale: "en-US", entries: {
    "delivery.details": "Details",
    "delivery.title": "Delivery details",
    "delivery.bubble": "Shown bubble",
    "delivery.prompt": "Delivered prompt",
    "delivery.messageId": "Message id",
    "delivery.empty": "This delivery message does not include a hidden prompt.",
    "delivery.close": "Close",
  } });
  api.registerLocale({ id: "delivery-zh", locale: "zh-CN", entries: {
    "delivery.details": "详情",
    "delivery.title": "投递详情",
    "delivery.bubble": "气泡文案",
    "delivery.prompt": "实际投递内容",
    "delivery.messageId": "消息 ID",
    "delivery.empty": "这条投递消息没有隐藏的提示词。",
    "delivery.close": "关闭",
  } });

  api.registerStyle({ id: "delivery", css: `
    .plugin-delivery-actions { display:flex; align-items:center; gap:6px; margin:6px 0 2px; }
    .plugin-delivery-detail { min-width:0; }
    .plugin-delivery-prompt {
      white-space:pre-wrap; word-break:break-word; overflow-wrap:anywhere;
      font:12px/1.6 var(--wuu-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
      font-variant-numeric:tabular-nums;
      background:var(--wuu-color-surface-muted, var(--paper-muted, #f6f6f4));
      border:1px solid var(--wuu-color-border-subtle, var(--line, #e3e2de));
      border-radius:var(--wuu-radius-control, 8px);
      padding:10px 12px;
    }
    .plugin-delivery-meta { margin:0; color:var(--wuu-color-text-muted, var(--ink-muted, #666)); font-size:11px; }
    .plugin-delivery-label { margin:10px 0 4px; color:var(--wuu-color-text-strong, var(--ink-strong, #181818)); font-size:12px; font-weight:600; }
  ` });

  const tr = (translate, key) => (typeof translate === "function" ? translate(key) : key);

  function DeliveryInspector(props) {
    const context = props.context || {};
    const inputText = typeof context.inputText === "string" ? context.inputText : "";
    const displayText = typeof context.displayText === "string" ? context.displayText : "";
    const messageId = typeof context.messageId === "string" ? context.messageId : "-";

    return h(Page, { className: "plugin-delivery-detail" },
      h(Section, { title: tr(props.translate, "delivery.title") },
        h(Stack, null,
          h("p", { className: "plugin-delivery-meta" }, `${tr(props.translate, "delivery.messageId")}: ${messageId}`),
          h("div", { className: "plugin-delivery-label" }, tr(props.translate, "delivery.bubble")),
          h("div", { className: "plugin-delivery-prompt" }, displayText || tr(props.translate, "delivery.empty")),
          h("div", { className: "plugin-delivery-label" }, tr(props.translate, "delivery.prompt")),
          h("div", { className: "plugin-delivery-prompt" }, inputText || tr(props.translate, "delivery.empty")),
        )),
      h(Row, { className: "plugin-delivery-actions" },
        h(Button, { variant: "ghost", onClick: () => void props.host.closeView() }, tr(props.translate, "delivery.close"))));
  }

  api.registerViewType({
    id: "delivery.inspector",
    title: "Delivery details",
    icon: "inbox",
    defaultRegion: "auxiliary",
    persistence: "session",
    render: DeliveryInspector,
  });
}
