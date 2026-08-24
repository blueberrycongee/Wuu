import { App } from "../../../desktop/src/renderer/App";
import { I18nProvider } from "../../../desktop/src/renderer/i18n";
import { applyMessageFlowFontSize } from "../../../desktop/src/renderer/MessageFlowFontSizeSection";
import { applyPlatformStamp } from "../../../desktop/src/renderer/platform";
import { startRendererVisibilitySync } from "../../../desktop/src/renderer/RendererVisibility";
import {
  applyMeasuredScrollbarWidth,
  startScrollbarWidthSync,
} from "../../../desktop/src/renderer/ScrollbarMetrics";
import {
  applyThemePreference,
  startThemePreferenceSync,
} from "../../../desktop/src/renderer/Theme";
import { ToastViewport } from "../../../desktop/src/renderer/Toast";
import { WuuUIRoot } from "../../../desktop/src/renderer/ui/layers/UILayerHost";
import "../../../desktop/src/renderer/styles.css";

applyThemePreference(window.wuu.initialThemePreference ?? "system");
startThemePreferenceSync();
applyPlatformStamp();
applyMeasuredScrollbarWidth();
startScrollbarWidthSync();
startRendererVisibilitySync();
applyMessageFlowFontSize(window.wuu.initialMessageFlowFontSize ?? 16);

export default function WebWorkspace(): React.JSX.Element {
  return (
    <I18nProvider>
      <WuuUIRoot>
        <App />
        <ToastViewport />
      </WuuUIRoot>
    </I18nProvider>
  );
}
