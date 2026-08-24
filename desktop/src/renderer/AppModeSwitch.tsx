import { useI18n } from "./i18n";

export type AppMode = "harness" | "collaboration";

export function AppModeSwitch({
  mode,
  collaborationEnabled,
  onChange,
}: {
  mode: AppMode;
  collaborationEnabled: boolean;
  onChange: (mode: AppMode) => void;
}): JSX.Element {
  const { t } = useI18n();

  return (
    <div className="sidebar-brand" aria-label={t("sidebar.productMode")}>
      <span className="sidebar-brand-wordmark">wuu</span>
      <div className="sidebar-mode-switch" role="group" aria-label={t("sidebar.productMode")}>
        {collaborationEnabled ? (
          <button
            className="sidebar-mode-option"
            type="button"
            aria-pressed={mode === "collaboration"}
            onClick={() => onChange("collaboration")}
          >
            {t("sidebar.collaboration")}
          </button>
        ) : null}
        <button
          className="sidebar-mode-option sidebar-brand-descriptor"
          type="button"
          aria-pressed={mode === "harness"}
          onClick={() => onChange("harness")}
        >
          {t("sidebar.harness")}
        </button>
      </div>
    </div>
  );
}
