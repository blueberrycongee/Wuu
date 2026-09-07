import type { ReactNode } from "react";

export function SettingsRow({
  title,
  description,
  children,
  block = false,
}: {
  title: string;
  description?: string;
  children: ReactNode;
  block?: boolean;
}): JSX.Element {
  return (
    <div className={`settings-row${block ? " settings-row-block" : ""}`} data-wuu-component="settings-row">
      <div className="settings-row-label">
        <span className="settings-row-label-title">{title}</span>
        {description ? <span className="settings-row-label-description">{description}</span> : null}
      </div>
      <div className={block ? "settings-row-control-block" : "settings-row-control"}>{children}</div>
    </div>
  );
}
