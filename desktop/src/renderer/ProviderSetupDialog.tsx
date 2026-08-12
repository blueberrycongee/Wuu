import { useState } from "react";
import { KeyRound } from "lucide-react";
import { SidebarNameDialog } from "./SidebarNameDialog";
import { useI18n } from "./i18n";
import type { ProviderSummary } from "../shared/protocol";

/**
 * First-launch provider setup dialog.
 *
 * Shown once on startup when no provider is usable: a compact form lets the
 * user create a provider (name + type + model + API key) right in the dialog.
 * Skipping persists a dismissal flag so the dialog never auto-shows again;
 * the settings providers page and the send-blocking toast remain the
 * fallback paths.
 */
export const PROVIDER_SETUP_DISMISSED_KEY = "wuu.provider-setup-dismissed";

export function hasReadyProvider(providers: ProviderSummary[] | undefined): boolean {
  if (!providers || providers.length === 0) {
    return false;
  }
  return providers.some(
    (provider) => provider.api_key_configured === true || provider.connection_locked === true,
  );
}

export type ProviderSetupConnection = {
  type: string;
  create_provider: true;
  api_key: string;
};

export type ProviderSetupDialogProps = {
  open: boolean;
  providers: ProviderSummary[] | undefined;
  onSave: (
    provider: string,
    model: string,
    connection: ProviderSetupConnection,
  ) => Promise<void>;
  onClose: () => void;
};

export function ProviderSetupDialog({
  open,
  providers,
  onSave,
  onClose,
}: ProviderSetupDialogProps): JSX.Element | null {
  const { t } = useI18n();
  const [providerName, setProviderName] = useState("");
  const [providerType, setProviderType] = useState("openai-compatible");
  const [model, setModel] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  if (!open) {
    return null;
  }

  const trimmedName = providerName.trim();
  const trimmedModel = model.trim();
  const trimmedKey = apiKey.trim();
  const nameTaken = providers?.some((provider) => provider.name === trimmedName) ?? false;
  const canSubmit =
    trimmedName !== "" && trimmedModel !== "" && trimmedKey !== "" && !nameTaken && !saving;

  async function handleSubmit(): Promise<void> {
    if (!canSubmit) {
      return;
    }
    setSaving(true);
    setError("");
    try {
      await onSave(trimmedName, trimmedModel, {
        type: providerType,
        create_provider: true,
        api_key: trimmedKey,
      });
      onClose();
    } catch (saveError) {
      setError(
        saveError instanceof Error ? saveError.message : t("provider.saveFailed"),
      );
      setSaving(false);
    }
  }

  function handleSkip(): void {
    try {
      window.localStorage.setItem(PROVIDER_SETUP_DISMISSED_KEY, "1");
    } catch {
      // Storage may be unavailable (private mode); the dialog still closes.
    }
    onClose();
  }

  const formContent = (
    <div className="provider-setup-fields">
      <label className="provider-setup-field">
        <span className="provider-setup-label">{t("provider.identifier")}</span>
        <input
          className="sidebar-name-dialog-input"
          value={providerName}
          onChange={(event) => setProviderName(event.currentTarget.value)}
          placeholder="openai"
          aria-label={t("provider.identifier")}
          autoFocus
        />
        {nameTaken ? (
          <span className="provider-setup-error" role="alert">
            {t("provider.nameExists")}
          </span>
        ) : null}
      </label>
      <label className="provider-setup-field">
        <span className="provider-setup-label">{t("provider.type")}</span>
        <select
          className="sidebar-name-dialog-input"
          value={providerType}
          onChange={(event) => setProviderType(event.currentTarget.value)}
          aria-label={t("provider.type")}
        >
          <option value="openai-compatible">{t("provider.openaiCompatible")}</option>
          <option value="anthropic">{t("provider.anthropicCompatible")}</option>
        </select>
      </label>
      <label className="provider-setup-field">
        <span className="provider-setup-label">{t("provider.modelName")}</span>
        <input
          className="sidebar-name-dialog-input"
          value={model}
          onChange={(event) => setModel(event.currentTarget.value)}
          placeholder="gpt-4o"
          aria-label={t("provider.modelName")}
        />
      </label>
      <label className="provider-setup-field">
        <span className="provider-setup-label">{t("provider.apiKey")}</span>
        <input
          className="sidebar-name-dialog-input"
          type="password"
          value={apiKey}
          onChange={(event) => setApiKey(event.currentTarget.value)}
          aria-label={t("provider.apiKey")}
        />
      </label>
      <p className="provider-setup-description">{t("providerSetup.description")}</p>
      {error ? (
        <p className="provider-setup-error" role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );

  return (
    <SidebarNameDialog
      open={open}
      title={providerName}
      onTitleChange={setProviderName}
      onSubmit={() => void handleSubmit()}
      onClose={handleSkip}
      dialogTitle={t("providerSetup.title")}
      dialogTitleId="provider-setup-dialog-title"
      fieldLabel=""
      fieldAriaLabel=""
      placeholder=""
      icon={KeyRound}
      submitLabel={t("providerSetup.save")}
      cancelLabel={t("providerSetup.skip")}
      content={formContent}
      submitDisabled={!canSubmit}
      closeOnEscape
    />
  );
}
