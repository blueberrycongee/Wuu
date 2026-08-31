import { Bell } from "lucide-react";
import {
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  useEffect,
  useRef,
  useState,
} from "react";
import { useI18n } from "./i18n";

export type AppMode = "harness" | "collaboration";
const CLEAR_UNREAD_HOLD_MS = 600;

export function AppModeSwitch({
  mode,
  collaborationEnabled,
  onChange,
  readOnly = false,
  unreadViewOpen = false,
  unreadCount = 0,
  onToggleUnreadView,
  onClearUnread,
}: {
  mode: AppMode;
  collaborationEnabled: boolean;
  onChange?: (mode: AppMode) => void;
  readOnly?: boolean;
  unreadViewOpen?: boolean;
  unreadCount?: number;
  onToggleUnreadView?: () => void;
  onClearUnread?: () => void;
}): JSX.Element {
  const { t } = useI18n();
  const holdTimerRef = useRef<number | undefined>(undefined);
  const longPressTriggeredRef = useRef(false);
  const keyboardPressRef = useRef<string | undefined>(undefined);
  const [holding, setHolding] = useState(false);

  function cancelHold(): void {
    if (holdTimerRef.current !== undefined) {
      window.clearTimeout(holdTimerRef.current);
      holdTimerRef.current = undefined;
    }
    setHolding(false);
  }

  function startHold(): void {
    cancelHold();
    longPressTriggeredRef.current = false;
    setHolding(true);
    holdTimerRef.current = window.setTimeout(() => {
      holdTimerRef.current = undefined;
      longPressTriggeredRef.current = true;
      setHolding(false);
      onClearUnread?.();
    }, CLEAR_UNREAD_HOLD_MS);
  }

  useEffect(() => () => {
    if (holdTimerRef.current !== undefined) {
      window.clearTimeout(holdTimerRef.current);
    }
  }, []);

  function handleNotificationPointerDown(event: ReactPointerEvent<HTMLButtonElement>): void {
    if (event.button !== 0) return;
    startHold();
  }

  function handleNotificationPointerLeave(): void {
    cancelHold();
    longPressTriggeredRef.current = false;
  }

  function handleNotificationKeyDown(event: ReactKeyboardEvent<HTMLButtonElement>): void {
    if ((event.key !== "Enter" && event.key !== " ") || event.repeat) return;
    event.preventDefault();
    keyboardPressRef.current = event.key;
    startHold();
  }

  function handleNotificationKeyUp(event: ReactKeyboardEvent<HTMLButtonElement>): void {
    if (keyboardPressRef.current !== event.key) return;
    event.preventDefault();
    keyboardPressRef.current = undefined;
    const wasLongPress = longPressTriggeredRef.current;
    cancelHold();
    longPressTriggeredRef.current = false;
    if (!wasLongPress) onToggleUnreadView?.();
  }

  return (
    <div className="sidebar-brand" aria-label={t("sidebar.productMode")}>
      <span className="sidebar-brand-wordmark">wuu</span>
      <div className="sidebar-mode-switch" role="group" aria-label={t("sidebar.productMode")}>
        {collaborationEnabled ? (
          readOnly ? (
            <span className="sidebar-mode-option sidebar-mode-option-static">
              {t("sidebar.collaboration")}
            </span>
          ) : (
            <button
              className="sidebar-mode-option"
              type="button"
              aria-pressed={mode === "collaboration"}
              onClick={() => onChange?.("collaboration")}
            >
              {t("sidebar.collaboration")}
            </button>
          )
        ) : null}
        {readOnly ? (
          <span className="sidebar-mode-option sidebar-mode-option-static sidebar-brand-descriptor">
            {t("sidebar.harness")}
          </span>
        ) : (
          <button
            className="sidebar-mode-option sidebar-brand-descriptor"
            type="button"
            aria-pressed={mode === "harness"}
            onClick={() => onChange?.("harness")}
          >
            {t("sidebar.harness")}
          </button>
        )}
      </div>
      {!readOnly && mode === "harness" ? (
        <button
          className="sidebar-notifications-button"
          type="button"
          aria-label={t("sidebar.notificationsHint", { count: unreadCount })}
          aria-pressed={unreadViewOpen}
          title={t("sidebar.notificationsHint", { count: unreadCount })}
          data-has-unread={unreadCount > 0 || undefined}
          data-holding={holding || undefined}
          onPointerDown={handleNotificationPointerDown}
          onPointerUp={cancelHold}
          onPointerCancel={() => {
            cancelHold();
            longPressTriggeredRef.current = false;
          }}
          onPointerLeave={handleNotificationPointerLeave}
          onKeyDown={handleNotificationKeyDown}
          onKeyUp={handleNotificationKeyUp}
          onBlur={() => {
            cancelHold();
            keyboardPressRef.current = undefined;
            longPressTriggeredRef.current = false;
          }}
          onClick={(event) => {
            if (longPressTriggeredRef.current) {
              event.preventDefault();
              longPressTriggeredRef.current = false;
              return;
            }
            onToggleUnreadView?.();
          }}
        >
          <Bell aria-hidden="true" />
          <span className="sidebar-notifications-dot" aria-hidden="true" />
        </button>
      ) : null}
    </div>
  );
}
