import { useLayoutEffect } from "react";
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

applyPlatformStamp();
applyMeasuredScrollbarWidth();
startScrollbarWidthSync();

export default function WebWorkspace(): React.JSX.Element {
  useLayoutEffect(() => {
    applyThemePreference(window.wuu.initialThemePreference ?? "system");
    applyMessageFlowFontSize(window.wuu.initialMessageFlowFontSize ?? 16);
    const stopTheme = startThemePreferenceSync();
    const stopVisibility = startRendererVisibilitySync();
    const viewport = window.visualViewport;
    const resize = (): void => {
      // The layout viewport can extend behind a phone's on-screen keyboard.
      if (!viewport || viewport.scale === 1) {
        document.documentElement.style.setProperty("--web-viewport-height", `${viewport?.height ?? window.innerHeight}px`);
      }
    };
    resize();
    viewport?.addEventListener("resize", resize);
    window.addEventListener("resize", resize);
    return () => {
      stopTheme();
      stopVisibility();
      viewport?.removeEventListener("resize", resize);
      window.removeEventListener("resize", resize);
      document.documentElement.style.removeProperty("--web-viewport-height");
    };
  }, []);
  return (
    <I18nProvider>
      <WuuUIRoot>
        <App />
        <ToastViewport />
      </WuuUIRoot>
    </I18nProvider>
  );
}
