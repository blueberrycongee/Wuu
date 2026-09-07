import { createRoot } from "react-dom/client";
import { FirstRunOnboarding } from "../../src/renderer/FirstRunOnboarding";
import { WuuMascotRuntimeProvider } from "../../src/renderer/WuuMascot";
import { I18nProvider } from "../../src/renderer/i18n";
import { applyPlatformStamp } from "../../src/renderer/platform";
import "../../src/renderer/styles.css";

async function rejectPersistence(): Promise<never> {
  throw new Error("Onboarding preview must not persist changes");
}

applyPlatformStamp();
createRoot(document.getElementById("root")!).render(
  <I18nProvider>
    <WuuMascotRuntimeProvider>
      <FirstRunOnboarding
        preview
        onDismissPreview={() => window.close()}
        onUpdateExtensionPackage={rejectPersistence}
        onSaveProvider={rejectPersistence}
        onUpdateEngines={rejectPersistence}
        onComplete={rejectPersistence}
      />
    </WuuMascotRuntimeProvider>
  </I18nProvider>,
);
