const SURFACES = [
  ["app.sidebar", "manga-pass-through manga-sidebar", "INDEX"],
  ["app.main", "manga-pass-through manga-main", "STAGE"],
  ["app.auxiliary", "manga-pass-through manga-auxiliary", "EXTRA"],
  ["app.status", "manga-pass-through manga-status-surface", "STATUS"],
  ["view.launch", "manga-view manga-launch", "START"],
  ["view.conversation", "manga-view manga-conversation", "STORY"],
  ["view.workspace", "manga-view manga-workspace", "FILES"],
  ["view.settings", "manga-view manga-settings", "SETTINGS"],
  ["view.catalog", "manga-view manga-catalog", "CATALOG"],
  ["conversation.timeline", "manga-pass-through manga-timeline", "TIMELINE"],
  ["conversation.message", "manga-pass-through manga-message", "MESSAGE"],
  ["conversation.composer", "manga-pass-through manga-composer", "COMPOSER"],
];

const PRESENTERS = [
  ["conversation.item", "manga-preview", "ITEM"],
  ["conversation.process", "manga-preview", "PROCESS"],
  ["conversation.composer", "manga-preview", "COMPOSER"],
  ["header.conversation", "manga-preview", "CONVERSATION HEADER"],
  ["header.workspace", "manga-preview", "WORKSPACE HEADER"],
  ["navigation.primary", "manga-preview", "NAVIGATION"],
  ["app.status", "manga-preview", "STATUS"],
  ["content.preview", "manga-preview", "PREVIEW"],
  ["settings", "manga-preview", "SETTINGS"],
];

function frame(React, className, label, fallback, tag = "div") {
  return React.createElement(
    tag,
    { className, "data-manga-label": label },
    fallback,
  );
}

function canInvoke(host, action) {
  return host.actions.includes(action);
}

function invoke(host, action, input) {
  if (!canInvoke(host, action)) return;
  void host.invoke(action, input).catch((error) => console.error(`Manga Studio action ${action} failed`, error));
}

function itemText(item) {
  if (item.text) return item.text;
  return (item.content ?? []).map((part) => part.text).filter(Boolean).join("\n\n");
}

function mangaButton(React, label, onClick, options = {}) {
  return React.createElement(
    "button",
    { type: "button", className: options.className ?? "manga-action", disabled: options.disabled, onClick },
    label,
  );
}

function renderMangaItem(React, snapshot, host) {
  const text = itemText(snapshot);
  const attachments = snapshot.attachments ?? [];
  const actions = [];
  if (canInvoke(host, "conversation.item.copy")) {
    actions.push(mangaButton(React, "COPY", () => invoke(host, "conversation.item.copy")));
  }
  if (canInvoke(host, "conversation.item.edit")) {
    actions.push(mangaButton(React, "EDIT", () => invoke(host, "conversation.item.edit")));
  }
  if (canInvoke(host, "conversation.item.retry")) {
    actions.push(mangaButton(React, "RETRY!", () => invoke(host, "conversation.item.retry"), { className: "manga-action manga-action-hot" }));
  }
  return React.createElement(
    "article",
    { className: `manga-native-message manga-native-${snapshot.kind}`, "data-status": snapshot.status ?? "completed" },
    React.createElement("div", { className: "manga-native-kicker" }, snapshot.kind.replaceAll("-", " ")),
    text ? React.createElement("div", { className: "manga-native-copy" }, text) : null,
    attachments.length
      ? React.createElement(
          "div",
          { className: "manga-native-attachments" },
          ...attachments.map((attachment) => mangaButton(
            React,
            `▣ ${attachment.name}`,
            () => invoke(host, "conversation.item.open-attachment", { attachmentId: attachment.id }),
          )),
        )
      : null,
    actions.length ? React.createElement("footer", { className: "manga-native-actions" }, ...actions) : null,
  );
}

function renderMangaProcess(React, snapshot) {
  return React.createElement(
    "section",
    { className: `manga-native-process manga-native-process-${snapshot.status}` },
    React.createElement(
      "header",
      { className: "manga-native-process-title" },
      React.createElement("span", null, snapshot.kind.toUpperCase()),
      React.createElement("strong", null, snapshot.streaming ? "DRAWING…" : snapshot.status.toUpperCase()),
    ),
    ...snapshot.items.map((item, index) => React.createElement(
      "div",
      { className: `manga-native-process-row manga-native-process-${item.kind}`, key: item.id },
      React.createElement("span", { className: "manga-native-process-index" }, String(index + 1).padStart(2, "0")),
      React.createElement(
        "div",
        null,
        React.createElement("strong", null, item.toolName ?? item.kind.replaceAll("-", " ")),
        item.text ? React.createElement("p", null, item.text) : null,
        item.error ? React.createElement("p", { role: "alert" }, item.error) : null,
      ),
    )),
  );
}

function renderMangaComposer(React, snapshot, host) {
  const draft = snapshot.draftText ?? "";
  const attachments = snapshot.attachments ?? [];
  const running = snapshot.running === true;
  const readOnly = snapshot.readOnly === true;
  return React.createElement(
    "section",
    { className: "manga-native-composer" },
    React.createElement(
      "header",
      { className: "manga-native-composer-meta" },
      React.createElement("strong", null, "NEXT PANEL"),
      React.createElement("span", null, snapshot.model?.label ?? "MODEL"),
      React.createElement("span", null, snapshot.permission?.label ?? "STANDARD"),
      snapshot.contextUsage?.percent === undefined
        ? null
        : React.createElement("span", null, `${Math.round(snapshot.contextUsage.percent)}% CONTEXT`),
    ),
    attachments.length
      ? React.createElement(
          "div",
          { className: "manga-native-composer-attachments" },
          ...attachments.map((attachment) => mangaButton(
            React,
            `${attachment.name} ×`,
            () => invoke(host, "conversation.composer.remove-attachment", attachment.id),
          )),
        )
      : null,
    React.createElement("textarea", {
      className: "manga-native-composer-input",
      value: draft,
      disabled: readOnly,
      placeholder: snapshot.disabledReason ?? "Write the next scene…",
      onChange: (event) => invoke(host, "conversation.composer.set-draft", event.currentTarget.value),
      onKeyDown: (event) => {
        if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent?.isComposing && draft.trim()) {
          event.preventDefault();
          invoke(host, "conversation.composer.submit");
        }
      },
    }),
    React.createElement(
      "footer",
      { className: "manga-native-composer-actions" },
      canInvoke(host, "conversation.composer.add-attachment")
        ? mangaButton(React, "+ ATTACH", () => invoke(host, "conversation.composer.add-attachment"))
        : null,
      React.createElement(
        "div",
        { className: "manga-native-modes" },
        ...(snapshot.availableSubmissionModes ?? []).map((mode) => mangaButton(
          React,
          mode.toUpperCase(),
          () => invoke(host, "conversation.composer.set-submission-mode", mode),
          { className: mode === snapshot.activeSubmissionMode ? "manga-action manga-action-selected" : "manga-action" },
        )),
      ),
      running && canInvoke(host, "conversation.composer.stop")
        ? mangaButton(React, "STOP!", () => invoke(host, "conversation.composer.stop"), { className: "manga-action manga-action-hot" })
        : mangaButton(
            React,
            "SEND →",
            () => invoke(host, "conversation.composer.submit"),
            { className: "manga-action manga-action-hot", disabled: readOnly || !draft.trim() },
          ),
    ),
  );
}

function renderMangaHeader(React, snapshot, host) {
  return React.createElement(
    "header",
    { className: "manga-native-header" },
    canInvoke(host, "header.navigate-back")
      ? mangaButton(React, "←", () => invoke(host, "header.navigate-back"), { disabled: !snapshot.canNavigateBack })
      : null,
    React.createElement(
      "div",
      { className: "manga-native-tabs" },
      ...(snapshot.tabs ?? []).map((tab, index) => React.createElement(
        "div",
        { className: tab.id === snapshot.activeTabId ? "manga-native-tab active" : "manga-native-tab", key: tab.id },
        mangaButton(React, `${String(index + 1).padStart(2, "0")} ${tab.title}`, () => invoke(host, "header.select-tab", { tabId: tab.id })),
        canInvoke(host, "header.close-tab")
          ? mangaButton(React, "×", () => invoke(host, "header.close-tab", { tabId: tab.id }))
          : null,
      )),
    ),
    snapshot.tabs?.length ? null : React.createElement("strong", null, snapshot.title ?? snapshot.scope.toUpperCase()),
    canInvoke(host, "header.navigate-forward")
      ? mangaButton(React, "→", () => invoke(host, "header.navigate-forward"), { disabled: !snapshot.canNavigateForward })
      : null,
  );
}

function renderMangaNavigation(React, snapshot, host) {
  return React.createElement(
    "nav",
    { className: "manga-native-navigation" },
    ...snapshot.nodes.map((node) => React.createElement(
      "div",
      {
        className: `manga-native-nav-node manga-native-nav-${node.kind}${node.active ? " active" : ""}`,
        key: node.id,
        style: { "--manga-depth": String(node.depth ?? 0) },
      },
      mangaButton(
        React,
        `${node.running ? "●" : node.unread ? "◆" : "○"} ${node.label}`,
        () => invoke(host, "navigation.activate-node", { id: node.id }),
        { disabled: node.disabled },
      ),
      node.pinned && canInvoke(host, "navigation.unpin-node")
        ? mangaButton(React, "UNPIN", () => invoke(host, "navigation.unpin-node", { id: node.id }))
        : null,
    )),
  );
}

export function activate(api) {
  const React = api.react;

  api.registerStyle({
    id: "manga-studio-design",
    order: 100,
    css: `
      @keyframes manga-pop {
        0% { transform: translateY(7px) scale(.985); opacity: 0; }
        100% { transform: translateY(0) scale(1); opacity: 1; }
      }

      .manga-shell {
        --manga-ink: #17130f;
        --manga-paper: #fffdf2;
        --manga-paper-deep: #f1ead8;
        --manga-pink: #ff3f7f;
        --manga-cyan: #39c7d4;
        --manga-yellow: #ffd83d;
        display: contents;
        color: var(--manga-ink);
        font-family: var(--wuu-font-family-ui, "Avenir Next Condensed", Arial, sans-serif);
      }
      body:has(.manga-shell) {
        --manga-ink: #17130f;
        --manga-paper: #fffdf2;
        --manga-paper-deep: #f1ead8;
        --manga-pink: #ff3f7f;
        --manga-cyan: #39c7d4;
        --manga-yellow: #ffd83d;
        color: var(--manga-ink);
        background-color: var(--manga-paper);
        background-image:
          radial-gradient(circle at 1px 1px, rgb(23 19 15 / .12) 1px, transparent 1.2px),
          linear-gradient(112deg, transparent 0 78%, rgb(255 63 127 / .07) 78% 79%, transparent 79%);
        background-size: 13px 13px, 100% 100%;
        font-family: var(--wuu-font-family-ui, "Avenir Next Condensed", Arial, sans-serif);
      }
      .manga-shell *, .manga-shell *::before, .manga-shell *::after { box-sizing: border-box; }
      .manga-mode-badge {
        position: fixed;
        top: 7px;
        right: 12px;
        z-index: 9000;
        pointer-events: none;
        padding: 3px 9px 2px;
        color: var(--manga-paper);
        background: var(--manga-ink);
        border: 2px solid var(--manga-paper);
        outline: 2px solid var(--manga-ink);
        font: 900 10px/1 var(--wuu-font-family-ui, sans-serif);
        letter-spacing: .16em;
        transform: rotate(1.5deg);
      }
      .manga-shell button,
      .manga-shell input,
      .manga-shell textarea,
      .manga-shell select { font-family: inherit; }
      .manga-shell button { border-radius: 3px !important; font-weight: 800; }
      .manga-shell button:not(:disabled):hover { transform: translate(-1px, -1px); }
      .manga-shell input,
      .manga-shell textarea,
      .manga-shell select {
        border-color: var(--manga-ink) !important;
        border-radius: 3px !important;
      }
      .manga-shell :focus-visible { outline: 3px solid var(--manga-pink) !important; outline-offset: 2px; }
      .manga-shell ::selection { color: var(--manga-ink); background: var(--manga-yellow); }
      .manga-shell ::-webkit-scrollbar { width: 10px; height: 10px; }
      .manga-shell ::-webkit-scrollbar-track { background: var(--manga-paper-deep); border-left: 1px solid var(--manga-ink); }
      .manga-shell ::-webkit-scrollbar-thumb { background: var(--manga-ink); border: 2px solid var(--manga-paper-deep); }
      .manga-shell [role="dialog"],
      .manga-shell [role="menu"],
      .manga-shell [role="listbox"] {
        border: 2px solid var(--manga-ink) !important;
        border-radius: 3px !important;
        box-shadow: 6px 6px 0 var(--manga-ink) !important;
      }
      body:has(.manga-shell) [data-wuu-component="dialog"],
      body:has(.manga-shell) [data-wuu-component="menu"],
      body:has(.manga-shell) [data-wuu-layer="popover"],
      body:has(.manga-shell) [data-wuu-layer="listbox"] {
        border: 2px solid var(--manga-ink) !important;
        border-radius: 3px !important;
        box-shadow: 6px 6px 0 var(--manga-ink) !important;
      }

      [data-manga-label] { position: relative; min-width: 0; min-height: 0; }
      [data-manga-label]::before {
        content: attr(data-manga-label);
        position: absolute;
        top: -2px;
        left: 12px;
        z-index: 12;
        pointer-events: none;
        padding: 3px 8px 2px;
        color: var(--manga-paper);
        background: var(--manga-ink);
        font-size: 9px;
        font-weight: 900;
        line-height: 1;
        letter-spacing: .14em;
      }

      .manga-pass-through { display: contents; }
      .manga-pass-through::before,
      .manga-pass-through::after { content: none; }

      .manga-sidebar {
        color: var(--manga-ink);
      }

      .manga-view {
        display: flex;
        flex-direction: column;
        width: 100%;
        height: 100%;
        min-width: 0;
        min-height: 0;
        overflow: hidden;
        border: 3px solid var(--manga-ink);
        border-radius: 5px;
        background: rgb(255 253 242 / .94);
        box-shadow: 5px 5px 0 var(--manga-ink);
        animation: manga-pop 180ms ease-out both;
      }
      .manga-view::after {
        content: "";
        position: absolute;
        right: -36px;
        bottom: -46px;
        width: 130px;
        height: 130px;
        pointer-events: none;
        border: 18px double rgb(23 19 15 / .09);
        border-radius: 50%;
      }
      .manga-launch::before { background: var(--manga-yellow); color: var(--manga-ink); }
      .manga-settings::before { background: var(--manga-cyan); color: var(--manga-ink); }
      .manga-catalog::before { background: var(--manga-pink); }

      .manga-panel {
        margin: 0;
        border: 0;
        border-radius: 4px;
        background: var(--manga-paper);
        box-shadow: inset 0 0 0 2px var(--manga-ink), inset -4px -4px 0 rgb(23 19 15 / .08);
      }
      .manga-message-surface { box-shadow: inset 3px 0 0 var(--manga-pink); }

      .manga-composer {
        margin: 0;
        padding: 0;
        border: 0;
        border-radius: 6px;
        box-shadow: inset 0 0 0 3px var(--manga-ink), inset 0 -6px 0 var(--manga-pink);
      }
      .manga-composer::before { top: 0; background: var(--manga-yellow); color: var(--manga-ink); transform: rotate(-1deg); }

      .manga-item {
        margin: 0;
        padding: 0;
        box-shadow: inset 5px 0 0 var(--manga-ink);
      }
      .manga-item::before { top: 2px; left: 0; }
      .manga-process {
        margin: 0;
        padding: 0;
        box-shadow: inset 0 0 0 2px var(--manga-ink);
        background-image: repeating-linear-gradient(-45deg, transparent 0 7px, rgb(57 199 212 / .12) 7px 9px);
      }
      .manga-process::before { left: auto; right: 8px; background: var(--manga-cyan); color: var(--manga-ink); }
      .manga-tool {
        margin: 0;
        padding: 0;
        border: 0;
        background: linear-gradient(95deg, rgb(255 216 61 / .22), transparent 58%);
        box-shadow: inset 3px 0 0 var(--manga-yellow);
      }
      .manga-tool::before { left: auto; right: 8px; background: var(--manga-pink); }

      .manga-preview { box-shadow: inset 0 0 0 3px var(--manga-ink); background: var(--manga-paper); }

      .manga-native-message {
        position: relative;
        width: min(88%, 820px);
        margin: 12px 0;
        padding: 24px 22px 16px;
        color: var(--manga-ink);
        background: var(--manga-paper);
        border: 3px solid var(--manga-ink);
        border-radius: 2px;
        box-shadow: 7px 7px 0 var(--manga-ink);
        white-space: pre-wrap;
      }
      .manga-native-user-message {
        width: min(72%, 680px);
        margin-left: auto;
        background: var(--manga-yellow);
        box-shadow: 7px 7px 0 var(--manga-pink);
        transform: rotate(.25deg);
      }
      .manga-native-assistant-message { margin-right: auto; }
      .manga-native-notice { width: 100%; background: var(--manga-cyan); }
      .manga-native-kicker {
        position: absolute;
        top: -11px;
        left: 12px;
        padding: 3px 9px;
        color: var(--manga-paper);
        background: var(--manga-ink);
        font-size: 10px;
        font-weight: 900;
        letter-spacing: .14em;
        text-transform: uppercase;
      }
      .manga-native-user-message .manga-native-kicker { left: auto; right: 12px; background: var(--manga-pink); }
      .manga-native-copy { font-size: 15px; font-weight: 650; line-height: 1.65; }
      .manga-native-actions,
      .manga-native-attachments { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px; }

      .manga-action {
        min-height: 28px;
        padding: 4px 10px;
        color: var(--manga-ink);
        background: var(--manga-paper);
        border: 2px solid var(--manga-ink) !important;
        border-radius: 1px !important;
        box-shadow: 2px 2px 0 var(--manga-ink);
        font-size: 10px;
        font-weight: 900;
        letter-spacing: .08em;
      }
      .manga-action-hot { background: var(--manga-pink); color: white; }
      .manga-action-selected { background: var(--manga-yellow); }
      .manga-action:disabled { opacity: .42; box-shadow: none; }

      .manga-native-process {
        position: relative;
        margin: 14px 0;
        border: 3px solid var(--manga-ink);
        background: repeating-linear-gradient(-45deg, var(--manga-paper) 0 9px, #f7f1df 9px 11px);
        box-shadow: 6px 6px 0 var(--manga-cyan);
      }
      .manga-native-process-title {
        display: flex;
        justify-content: space-between;
        padding: 7px 10px;
        color: var(--manga-paper);
        background: var(--manga-ink);
        letter-spacing: .12em;
      }
      .manga-native-process-title strong { color: var(--manga-yellow); }
      .manga-native-process-row { display: grid; grid-template-columns: 38px 1fr; gap: 10px; padding: 10px; border-top: 2px solid var(--manga-ink); }
      .manga-native-process-index { display: grid; place-items: center; height: 30px; background: var(--manga-pink); color: white; font-weight: 900; }
      .manga-native-process-row p { margin: 5px 0 0; white-space: pre-wrap; }

      .manga-native-composer {
        width: 100%;
        padding: 12px;
        color: var(--manga-ink);
        background: var(--manga-paper);
        border: 3px solid var(--manga-ink);
        box-shadow: inset 0 -7px 0 var(--manga-pink);
      }
      .manga-native-composer-meta,
      .manga-native-composer-actions { display: flex; align-items: center; gap: 8px; }
      .manga-native-composer-meta { margin-bottom: 8px; font-size: 10px; font-weight: 900; letter-spacing: .1em; }
      .manga-native-composer-meta strong { padding: 4px 8px; background: var(--manga-yellow); }
      .manga-native-composer-input {
        display: block;
        width: 100%;
        min-height: 76px;
        max-height: 180px;
        resize: vertical;
        padding: 12px 14px;
        color: var(--manga-ink);
        background: white;
        border: 2px solid var(--manga-ink) !important;
        box-shadow: inset 4px 4px 0 rgb(23 19 15 / .08);
        font-size: 15px;
        line-height: 1.45;
      }
      .manga-native-composer-actions { justify-content: space-between; margin-top: 8px; padding-bottom: 5px; }
      .manga-native-modes,
      .manga-native-composer-attachments { display: flex; flex-wrap: wrap; gap: 6px; }

      .manga-native-header {
        display: flex;
        align-items: center;
        min-width: 0;
        height: 100%;
        padding: 5px 10px;
        color: var(--manga-paper);
        background: var(--manga-ink);
        border-bottom: 4px solid var(--manga-pink);
      }
      .manga-native-tabs { display: flex; min-width: 0; flex: 1; gap: 5px; overflow-x: auto; }
      .manga-native-tab { display: flex; background: #332d26; border: 1px solid var(--manga-paper); }
      .manga-native-tab.active { background: var(--manga-yellow); }
      .manga-native-tab.active .manga-action { color: var(--manga-ink); }
      .manga-native-header .manga-action { color: var(--manga-paper); background: transparent; border: 0 !important; box-shadow: none; white-space: nowrap; }

      .manga-native-navigation { display: grid; gap: 3px; padding: 8px 5px; }
      .manga-native-nav-node { display: flex; padding-left: calc(var(--manga-depth) * 12px); }
      .manga-native-nav-node .manga-action:first-child { flex: 1; text-align: left; border-color: transparent !important; box-shadow: none; background: transparent; }
      .manga-native-nav-node.active .manga-action:first-child { background: var(--manga-yellow); border-color: var(--manga-ink) !important; box-shadow: 3px 3px 0 var(--manga-ink); }
      .manga-native-nav-section { margin-top: 8px; border-top: 2px solid var(--manga-ink); }

      @media (prefers-reduced-motion: reduce) {
        .manga-view { animation: none; }
        .manga-shell button:not(:disabled):hover { transform: none; }
      }
    `,
  });

  api.registerSurface("app.shell", {
    id: "manga-shell",
    mode: "wrap",
    render(_context, fallback) {
      return React.createElement(
        "main",
        { className: "manga-shell" },
        fallback,
        React.createElement("span", { className: "manga-mode-badge", "aria-hidden": true }, "MANGA MODE • WUU"),
      );
    },
  });

  for (const [surface, className, label] of SURFACES) {
    api.registerSurface(surface, {
      id: `manga-${surface.replaceAll(".", "-")}`,
      mode: "wrap",
      render(_context, fallback) {
        return frame(React, className, label, fallback);
      },
    });
  }

  for (const [target, className, label] of PRESENTERS) {
    api.registerPresenter({
      id: `manga-presenter-${target.replaceAll(".", "-")}`,
      target,
      mode: "wrap",
      render({ fallback }) {
        return frame(React, className, label, fallback);
      },
    });
  }

  api.registerPresenter({
    id: "manga-native-conversation-item",
    target: "conversation.item",
    mode: "replace",
    priority: 100,
    render({ snapshot, host }) {
      return renderMangaItem(React, snapshot, host);
    },
  });
  api.registerPresenter({
    id: "manga-native-process",
    target: "conversation.process",
    mode: "replace",
    priority: 100,
    render({ snapshot }) {
      return renderMangaProcess(React, snapshot);
    },
  });
  api.registerPresenter({
    id: "manga-native-composer",
    target: "conversation.composer",
    mode: "replace",
    priority: 100,
    render({ snapshot, host }) {
      return renderMangaComposer(React, snapshot, host);
    },
  });
  for (const target of ["header.conversation", "header.workspace"]) {
    api.registerPresenter({
      id: `manga-native-${target.replaceAll(".", "-")}`,
      target,
      mode: "replace",
      priority: 100,
      render({ snapshot, host }) {
        return renderMangaHeader(React, snapshot, host);
      },
    });
  }
  api.registerPresenter({
    id: "manga-native-navigation",
    target: "navigation.primary",
    mode: "replace",
    priority: 100,
    render({ snapshot, host }) {
      return renderMangaNavigation(React, snapshot, host);
    },
  });

}
