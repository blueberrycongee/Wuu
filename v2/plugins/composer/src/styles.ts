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
  box-sizing: border-box;
  overflow: hidden;
  border: 0;
  border-radius: var(--session-composer-radius, 18px);
  background: var(--paper, #fff);
  box-shadow:
    inset 0 0 0 1px var(--surface-4, rgba(31, 35, 40, 0.12)),
    0 8px 24px rgba(20, 24, 28, 0.06);
}

.wuu-composer-stack.is-expanded .wuu-composer-surface {
  height: var(--wuu-composer-expanded-height);
  min-height: var(--wuu-composer-expanded-height);
  max-height: var(--wuu-composer-expanded-height);
}

.wuu-composer-command-host {
  position: relative;
  z-index: 20;
  height: 0;
  min-width: 0;
}

.wuu-composer-surface textarea {
  display: block;
  width: 100%;
  height: 60px;
  min-height: 60px;
  max-height: 180px;
  flex: 0 0 auto;
  box-sizing: border-box;
  resize: none;
  overflow-y: auto;
  padding: 10px 44px 8px 16px;
  border: 0;
  outline: 0;
  color: var(--ink, #202423);
  background: transparent;
  font: inherit;
  font-size: 15px;
  line-height: 22px;
}

.wuu-composer-stack.is-expanded .wuu-composer-surface textarea {
  height: auto;
  min-height: clamp(180px, 34vh, 320px);
  max-height: none;
  flex: 1 1 0;
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
  height: 40px;
  min-height: 40px;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 8px 4px;
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

.wuu-composer-toolbar label {
  display: flex;
  min-width: 0;
}

.wuu-composer-toolbar select {
  max-width: 180px;
  height: 28px;
  min-width: 0;
  padding: 0 24px 0 8px;
  border: 0;
  border-radius: 8px;
  color: var(--ink-muted, #5f6663);
  background: transparent;
  font: inherit;
  font-size: 12px;
}

.wuu-composer-toolbar select:hover,
.wuu-composer-toolbar select:focus-visible {
  background: rgba(31, 35, 40, 0.08);
  outline: none;
}

.wuu-composer-toolbar select:disabled {
  opacity: 0.5;
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
