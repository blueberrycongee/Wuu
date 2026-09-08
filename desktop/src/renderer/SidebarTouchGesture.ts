import { useEffect, type RefObject } from "react";
import { isTouchWebShell } from "./ComposerFocus";

// Leave controls and horizontal scrollers (code, editors, terminal) in charge
// of their own gestures, even when they reach the edge of the screen.
function ownsGesture(target: Element, shell: HTMLElement): boolean {
  if (target.closest('input, textarea, select, button, a, [contenteditable], [role="dialog"], [role="slider"], .monaco-editor, .xterm')) {
    return true;
  }
  for (let node: Element | null = target; node && node !== shell; node = node.parentElement) {
    if (node.scrollWidth > node.clientWidth && /auto|scroll/.test(getComputedStyle(node).overflowX)) {
      return true;
    }
  }
  return false;
}

export function useSidebarTouchGesture(
  shellRef: RefObject<HTMLDivElement | null>,
  enabled: boolean,
  open: () => void,
): void {
  useEffect(() => {
    const shell = shellRef.current;
    if (!enabled || !shell || !isTouchWebShell()) return;

    let gesture: { id: number; x: number; y: number; horizontal: boolean } | null = null;
    const cancel = (): void => { gesture = null; };
    const start = (event: TouchEvent): void => {
      cancel();
      if (event.defaultPrevented || event.touches.length !== 1 || !isTouchWebShell()) return;
      const touch = event.touches[0];
      const x = touch.clientX - shell.getBoundingClientRect().left;
      if (x < 0 || x > 28 || !(event.target instanceof Element) || ownsGesture(event.target, shell)) return;
      gesture = { id: touch.identifier, x: touch.clientX, y: touch.clientY, horizontal: false };
    };
    const move = (event: TouchEvent): void => {
      if (!gesture) return;
      const touch = event.touches[0];
      if (event.defaultPrevented || event.touches.length !== 1 || touch.identifier !== gesture.id || !event.cancelable) {
        cancel();
        return;
      }
      const dx = touch.clientX - gesture.x;
      const dy = Math.abs(touch.clientY - gesture.y);
      if (!gesture.horizontal) {
        if (Math.max(Math.abs(dx), dy) < 10) return;
        // Once a vertical/leftward gesture starts, never reclaim it later.
        if (dx <= dy * 1.5) { cancel(); return; }
        // Keep this direction until release, like a drawer drag: a short drag
        // can be abandoned without turning its tail into a page scroll.
        gesture.horizontal = true;
      }
      event.preventDefault();
    };
    const end = (event: TouchEvent): void => {
      const current = gesture;
      cancel();
      if (!current?.horizontal || event.touches.length || event.defaultPrevented) return;
      const touch = Array.from(event.changedTouches).find((item) => item.identifier === current.id);
      if (!touch) return;
      const dx = touch.clientX - current.x;
      if (dx >= 64 && dx > Math.abs(touch.clientY - current.y) * 1.5) {
        if (event.cancelable) event.preventDefault();
        open();
      }
    };

    shell.addEventListener("touchstart", start, { passive: true });
    // Cancel scrolling only after a rightward edge gesture is established.
    shell.addEventListener("touchmove", move, { passive: false });
    shell.addEventListener("touchend", end, { passive: false });
    shell.addEventListener("touchcancel", cancel);
    return () => {
      shell.removeEventListener("touchstart", start);
      shell.removeEventListener("touchmove", move);
      shell.removeEventListener("touchend", end);
      shell.removeEventListener("touchcancel", cancel);
    };
  }, [shellRef, enabled, open]);
}
