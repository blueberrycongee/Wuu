export const composerStyles = `
.wuu-composer-stack {
  --wuu-composer-collapsed-height: 100px;
  --wuu-composer-expanded-height: clamp(240px, 44vh, 420px);
  position: relative;
  display: flex;
  width: 100%;
  min-width: 0;
  flex-direction: column;
}

.wuu-composer-surface {
  position: relative;
  display: flex;
  min-height: var(--wuu-composer-collapsed-height);
  flex-direction: column;
  gap: 8px;
  box-sizing: border-box;
  padding: 10px 8px 7px;
  overflow: hidden;
  border: 1px solid var(--surface-4, rgba(31, 35, 40, 0.12));
  border-radius: var(--session-composer-radius, 18px);
  background: var(--paper, #fff);
  box-shadow: 0 8px 24px rgba(20, 24, 28, 0.06);
}

.wuu-composer-stack.is-expanded .wuu-composer-surface {
  min-height: var(--wuu-composer-expanded-height);
}

.wuu-composer-surface textarea {
  field-sizing: content;
  width: 100%;
  min-height: 46px;
  max-height: 336px;
  flex: 1 1 auto;
  box-sizing: border-box;
  resize: none;
  overflow-y: auto;
  padding: 4px 36px 4px 8px;
  border: 0;
  outline: 0;
  color: var(--ink, #202423);
  background: transparent;
  font: inherit;
  font-size: 15px;
  line-height: 22px;
}

.wuu-composer-surface textarea::placeholder {
  color: var(--ink-muted, #8b918e);
}

.wuu-composer-expand {
  position: absolute;
  top: 8px;
  right: 8px;
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 7px;
  color: var(--ink-muted, #6d7471);
  background: transparent;
}

.wuu-composer-expand:hover,
.wuu-composer-expand:focus-visible {
  background: rgba(31, 35, 40, 0.08);
}

.wuu-composer-toolbar {
  display: flex;
  min-height: 28px;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.wuu-composer-toolbar-left,
.wuu-composer-toolbar-right {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

.wuu-composer-toolbar-right {
  flex: none;
}

.wuu-composer-status {
  min-width: 0;
  overflow: hidden;
  color: var(--danger, #b1271b);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.wuu-composer-send {
  display: grid;
  width: 32px;
  height: 32px;
  place-items: center;
  flex: none;
  padding: 0;
  border: 0;
  border-radius: 999px;
  color: var(--wuu-color-on-accent, #fff);
  background: var(--wuu-accent, #202423);
  font-weight: 600;
}

.wuu-composer-send:disabled {
  opacity: 0.45;
}

@media (prefers-reduced-motion: reduce) {
  .wuu-composer-surface,
  .wuu-composer-expand {
    transition: none;
  }
}
`;
