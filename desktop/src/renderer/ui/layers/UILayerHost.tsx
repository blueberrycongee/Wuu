import {
  createContext,
  type ReactNode,
  useContext,
  useState,
} from "react";

const UILayerHostContext = createContext<HTMLElement | null>(null);

export interface WuuUIRootProps {
  children: ReactNode;
}

/**
 * Owns the protected DOM target for host overlays. It intentionally sits
 * outside replaceable plugin surfaces while retaining React context for
 * dialogs, menus, popovers, tooltips, and notices rendered through portals.
 */
export function WuuUIRoot({ children }: WuuUIRootProps): JSX.Element {
  const [layerHost, setLayerHost] = useState<HTMLDivElement | null>(null);

  return (
    <UILayerHostContext.Provider value={layerHost}>
      {children}
      <div
        ref={setLayerHost}
        data-wuu-layer-host="true"
        data-wuu-component="layer-host"
      />
    </UILayerHostContext.Provider>
  );
}

/**
 * Standalone component tests and legacy secondary roots fall back to body.
 * The production renderer always provides WuuUIRoot.
 */
export function useUILayerHost(): HTMLElement {
  return useContext(UILayerHostContext) ?? document.body;
}
