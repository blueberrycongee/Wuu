import {
  createContext,
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";

const LayerContext = createContext<HTMLElement | null>(null);

const styles = `
.wuu-dialog-layer { position: fixed; z-index: 210; inset: 0; pointer-events: none; }
.wuu-dialog-backdrop {
  position: absolute; inset: 0; display: grid; place-items: center; min-width: 0; min-height: 0;
  padding: 24px; pointer-events: auto; background: rgba(20, 23, 22, 0.28); backdrop-filter: blur(2px);
}
.wuu-dialog {
  display: grid; width: min(var(--wuu-dialog-max-width, 640px), 100%); max-height: 100%; gap: 14px;
  overflow: auto; padding: 20px; border: 1px solid var(--hairline-strong); border-radius: 16px;
  color: var(--ink); background: var(--paper-solid); box-shadow: 0 24px 70px rgba(20, 23, 22, 0.24);
}
.wuu-dialog-header, .wuu-dialog-footer { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.wuu-dialog-heading { display: flex; min-width: 0; align-items: center; gap: 12px; }
.wuu-dialog-icon { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 8px; background: var(--surface-2); }
.wuu-dialog-title { margin: 0; font-size: 17px; font-weight: 650; line-height: 1.25; }
.wuu-dialog-subtitle { margin: -4px 0 0; color: var(--ink-muted); font-size: 13px; line-height: 1.45; }
.wuu-dialog-close { display: grid; width: 32px; height: 32px; place-items: center; padding: 0; border: 0; border-radius: 8px; color: var(--ink-muted); background: transparent; font-size: 20px; }
.wuu-dialog-close:hover:not(:disabled), .wuu-dialog-close:focus-visible { color: var(--ink); background: var(--surface-2); }
.wuu-dialog-footer { justify-content: flex-end; }
@media (max-width: 720px) { .wuu-dialog-backdrop { padding: 12px; } .wuu-dialog { padding: 16px; } }
`;

export function DialogLayerHost({ children }: { children: ReactNode }) {
  const [host, setHost] = useState<HTMLDivElement | null>(null);
  useEffect(() => {
    if (typeof document === "undefined") return;
    const style = document.createElement("style");
    style.dataset.wuuUiKit = "dialog";
    style.textContent = styles;
    document.head.append(style);
    return () => style.remove();
  }, []);
  return (
    <LayerContext.Provider value={host}>
      {children}
      <div ref={setHost} className="wuu-dialog-layer" data-wuu-layer="dialog" />
    </LayerContext.Provider>
  );
}

function focusable(panel: HTMLElement): HTMLElement[] {
  return [...panel.querySelectorAll<HTMLElement>(
    "a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex='-1'])",
  )].filter((element) => !element.hasAttribute("hidden") && element.getAttribute("aria-hidden") !== "true");
}

export interface DialogProps {
  title: ReactNode;
  ariaLabel?: string;
  icon?: ReactNode;
  subtitle?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  onClose?: () => void;
  closeDisabled?: boolean;
  asForm?: boolean;
  onSubmit?: (event: FormEvent<HTMLFormElement>) => void;
  className?: string;
}

export function Dialog({
  title,
  ariaLabel,
  icon,
  subtitle,
  children,
  footer,
  onClose,
  closeDisabled = false,
  asForm = false,
  onSubmit,
  className,
}: DialogProps) {
  const host = useContext(LayerContext);
  const titleId = useId();
  const panel = useRef<HTMLElement | null>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const dismissible = Boolean(onClose) && !closeDisabled;

  useEffect(() => {
    if (!host) return;
    previousFocus.current = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    const target = panel.current && (focusable(panel.current)[0] ?? panel.current);
    target?.focus({ preventScroll: true });
    return () => {
      if (previousFocus.current?.isConnected) previousFocus.current.focus({ preventScroll: true });
    };
  }, [host]);

  if (!host) return null;
  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    event.stopPropagation();
    if (event.key === "Escape" && dismissible) {
      event.preventDefault();
      onClose?.();
      return;
    }
    if (event.key !== "Tab" || !panel.current) return;
    const targets = focusable(panel.current);
    if (!targets.length) {
      event.preventDefault();
      panel.current.focus();
      return;
    }
    const current = document.activeElement;
    const index = targets.indexOf(current as HTMLElement);
    const next = event.shiftKey
      ? targets[(index <= 0 ? targets.length : index) - 1]
      : targets[(index + 1) % targets.length];
    event.preventDefault();
    next?.focus();
  };
  const body = (
    <>
      <header className="wuu-dialog-header">
        <div className="wuu-dialog-heading">
          {icon === undefined ? null : <span className="wuu-dialog-icon" aria-hidden="true">{icon}</span>}
          <h2 id={titleId} className="wuu-dialog-title">{title}</h2>
        </div>
        {onClose ? (
          <button className="wuu-dialog-close" type="button" aria-label="Close" disabled={closeDisabled} onClick={() => onClose()}>
            ×
          </button>
        ) : null}
      </header>
      {subtitle === undefined ? null : <p className="wuu-dialog-subtitle">{subtitle}</p>}
      {children}
      {footer === undefined ? null : <footer className="wuu-dialog-footer">{footer}</footer>}
    </>
  );
  const common = {
    ref: (node: HTMLElement | null) => { panel.current = node; },
    className: `wuu-dialog${className ? ` ${className}` : ""}`,
    role: "dialog" as const,
    "aria-modal": true,
    ...(ariaLabel ? { "aria-label": ariaLabel } : { "aria-labelledby": titleId }),
    tabIndex: -1,
    onKeyDown,
    onClick: (event: MouseEvent<HTMLElement>) => event.stopPropagation(),
  };
  return createPortal(
    <div
      className="wuu-dialog-backdrop"
      role="presentation"
      onPointerDown={(event) => {
        if (event.target === event.currentTarget && dismissible) onClose?.();
      }}
    >
      {asForm ? (
        <form {...common} onSubmit={(event) => { event.preventDefault(); onSubmit?.(event); }}>{body}</form>
      ) : <div {...common}>{body}</div>}
    </div>,
    host,
  );
}
