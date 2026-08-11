import { ChevronRight, Code2, Play } from "lucide-react";
import { useMemo, useState } from "react";
import type { ThreadItem } from "../shared/protocol";
import {
  buildFrontendPreviewDocument,
  parseFrontendPreviewSpec,
  type FrontendPreviewSpec,
} from "./FrontendPreviewSpec";
import { useI18n } from "./i18n";

export function FrontendPreviewCard({ item }: { item: ThreadItem }): JSX.Element {
  const { t } = useI18n();
  const parsed = useMemo(() => parseFrontendPreviewSpec(item.arguments), [item.arguments]);
  const spec = parsed.spec;
  const [expanded, setExpanded] = useState(false);
  const [tab, setTab] = useState<"preview" | "source">("preview");
  const title = parsed.spec?.title ?? t("frontendPreview.invalidTitle");

  return (
    <section className={`frontend-preview-card${parsed.error ? " is-invalid" : ""}`}>
      <button
        type="button"
        className="frontend-preview-summary"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
      >
        <Play className="icon-sm frontend-preview-kind-icon" aria-hidden />
        <span className="frontend-preview-summary-copy">
          <span className="frontend-preview-title">{title}</span>
          <span className="frontend-preview-kind">{t("frontendPreview.kind")}</span>
        </span>
        <ChevronRight className="icon-sm frontend-preview-chevron" aria-hidden />
      </button>
      {expanded ? (
        <div className="frontend-preview-body">
          {parsed.error || !spec ? (
            <div className="frontend-preview-error" role="alert">
              {t("frontendPreview.invalidDetail", {
                detail: parsed.error ?? t("frontendPreview.invalidTitle"),
              })}
            </div>
          ) : (
            <>
              <div className="frontend-preview-tabs" role="tablist" aria-label={t("frontendPreview.tabsLabel")}>
                <button
                  type="button"
                  role="tab"
                  aria-selected={tab === "preview"}
                  className={tab === "preview" ? "is-active" : ""}
                  onClick={() => setTab("preview")}
                >
                  <Play className="icon-xs" aria-hidden />
                  {t("frontendPreview.previewTab")}
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={tab === "source"}
                  className={tab === "source" ? "is-active" : ""}
                  onClick={() => setTab("source")}
                >
                  <Code2 className="icon-xs" aria-hidden />
                  {t("frontendPreview.sourceTab")}
                </button>
              </div>
              {tab === "preview" ? (
                <FrontendPreviewFrame spec={spec} />
              ) : (
                <FrontendPreviewSource spec={spec} />
              )}
            </>
          )}
        </div>
      ) : null}
    </section>
  );
}

function FrontendPreviewFrame({ spec }: { spec: FrontendPreviewSpec }): JSX.Element {
  const source = useMemo(() => buildFrontendPreviewDocument(spec), [spec]);
  return (
    <iframe
      className="frontend-preview-frame"
      title={spec.title}
      sandbox="allow-scripts"
      referrerPolicy="no-referrer"
      allow="camera 'none'; microphone 'none'; geolocation 'none'; clipboard-read 'none'; clipboard-write 'none'"
      srcDoc={source}
      style={{ height: `${spec.viewport.height}px` }}
    />
  );
}

function FrontendPreviewSource({ spec }: { spec: FrontendPreviewSpec }): JSX.Element {
  const sections = [
    ["HTML", spec.html],
    ["CSS", spec.css],
    ["JavaScript", spec.javascript],
  ].filter((entry) => entry[1].trim().length > 0);
  return (
    <div className="frontend-preview-source">
      {sections.map(([label, source]) => (
        <section key={label}>
          <h4>{label}</h4>
          <pre><code>{source}</code></pre>
        </section>
      ))}
    </div>
  );
}
