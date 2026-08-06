import { useEffect, useState, type ReactNode } from "react";
import { motionDurationMs } from "./motion";

const COLLAPSE_MOTION_FALLBACK_MS = 440;
const COLLAPSED_CONTENT_RELEASE_BUFFER_MS = 32;

export function CollapsibleDetails({
  children,
  className,
  expanded,
  id,
  innerClassName,
}: {
  children: ReactNode;
  className?: string;
  expanded: boolean;
  id?: string;
  innerClassName?: string;
}): JSX.Element {
  const [renderChildren, setRenderChildren] = useState(expanded);
  useEffect(() => {
    if (expanded) {
      setRenderChildren(true);
      return undefined;
    }
    if (!renderChildren) {
      return undefined;
    }
    // Keep the body mounted through the 440ms close motion, then release the
    // hidden Markdown/tool tree. Long conversations otherwise retain every
    // completed process row even though the folds are collapsed.
    const motionDuration = motionDurationMs(
      "--collapse-motion-duration",
      COLLAPSE_MOTION_FALLBACK_MS,
    );
    const retention =
      motionDuration > 0
        ? motionDuration + COLLAPSED_CONTENT_RELEASE_BUFFER_MS
        : 0;
    const timer = window.setTimeout(
      () => setRenderChildren(false),
      retention,
    );
    return () => window.clearTimeout(timer);
  }, [expanded, renderChildren]);
  const shouldRenderChildren = expanded || renderChildren;
  const detailsClassName = [
    "collapsible-details",
    expanded ? "expanded" : "collapsed",
    className,
  ]
    .filter(Boolean)
    .join(" ");
  const innerClassNames = ["collapsible-details-inner", innerClassName]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={detailsClassName} id={id} aria-hidden={!expanded}>
      <div className={innerClassNames}>{shouldRenderChildren ? children : null}</div>
    </div>
  );
}
