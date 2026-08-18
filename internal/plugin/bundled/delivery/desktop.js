export async function activate(api) {
  const React = api.react;
  const h = React.createElement;
  const { Page, Stack } = api.ui;

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
    .plugin-delivery-detail { min-width:0; }
    .plugin-delivery-content { gap:18px; }
    .plugin-delivery-meta { display:grid; min-width:0; gap:4px; }
    .plugin-delivery-meta-label,
    .plugin-delivery-label {
      color:var(--wuu-color-text-muted, var(--ink-muted, #666));
      font-size:11px; font-weight:600;
    }
    .plugin-delivery-meta-value {
      min-width:0; overflow-wrap:anywhere;
      color:var(--wuu-color-text, var(--ink, #222));
      font:11px/1.45 var(--wuu-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
    }
    .plugin-delivery-field { display:grid; min-width:0; gap:6px; }
    .plugin-delivery-prompt {
      white-space:pre-wrap; word-break:break-word; overflow-wrap:anywhere;
      margin:0;
      font:12px/1.55 var(--wuu-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
      font-variant-numeric:tabular-nums;
      background:var(--wuu-color-surface-muted, var(--paper-muted, #f6f6f4));
      border:1px solid var(--wuu-color-border-subtle, var(--line, #e3e2de));
      border-radius:var(--wuu-radius-control, 8px);
      padding:10px 12px;
    }
  ` });

  const tr = (translate, key) => (typeof translate === "function" ? translate(key) : key);

  function DeliveryInspector(props) {
    const context = props.context || {};
    const inputText = typeof context.inputText === "string" ? context.inputText : "";
    const displayText = typeof context.displayText === "string" ? context.displayText : "";
    const messageId = typeof context.messageId === "string" ? context.messageId : "-";

    return h(Page, { className: "plugin-delivery-detail", density: "compact" },
      h(Stack, { className: "plugin-delivery-content" },
        h("div", { className: "plugin-delivery-meta" },
          h("span", { className: "plugin-delivery-meta-label" }, tr(props.translate, "delivery.messageId")),
          h("span", { className: "plugin-delivery-meta-value" }, messageId)),
        h("section", { className: "plugin-delivery-field" },
          h("div", { className: "plugin-delivery-label" }, tr(props.translate, "delivery.bubble")),
          h("p", { className: "plugin-delivery-prompt" }, displayText || tr(props.translate, "delivery.empty"))),
        h("section", { className: "plugin-delivery-field" },
          h("div", { className: "plugin-delivery-label" }, tr(props.translate, "delivery.prompt")),
          h("p", { className: "plugin-delivery-prompt" }, inputText || tr(props.translate, "delivery.empty")))));
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
