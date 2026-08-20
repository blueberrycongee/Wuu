import { useI18n } from "./i18n";
import { WuuMascot, type WuuMascotActivity } from "./WuuMascot";

export function RuntimeLoading({
  status,
  pinned = false,
  onExitPreview
}: {
  status: string;
  pinned?: boolean;
  onExitPreview?: () => void;
}): JSX.Element {
  const { t } = useI18n();
  const isStarting = pinned || status === "connecting" || status === "opening";
  return (
    <div className="project-empty-pane" data-wuu-component="launch-view">
      {isStarting ? (
        <div className="wuu-launch" role="status" aria-label={pinned ? t("loading.launchPreview") : t("loading.starting")}>
          <div className="wuu-launch-glass" aria-hidden="true">
            <WuuMascot
              className="wuu-launch-mascot"
              accessory="none"
            />
          </div>
          {pinned && onExitPreview ? (
            <button className="wuu-launch-exit" type="button" onClick={onExitPreview}>
              {t("loading.exitPreview")}
            </button>
          ) : null}
        </div>
      ) : (
        <div className="project-empty-content">
          <h2>{status}</h2>
        </div>
      )}
    </div>
  );
}

export function ViewSwitchLoading(): JSX.Element {
  const { t } = useI18n();
  return (
    <div className="view-switch-loading" role="status" aria-label={t("loading.switching")}>
      <div className="wuu-launch-mark view-switch-mark" aria-hidden="true">
        <span>w</span>
        <span>u</span>
        <span>u</span>
      </div>
      <div className="wuu-launch-rail view-switch-rail" aria-hidden="true" />
    </div>
  );
}

export function EmptyConversationHome({
  title,
  belowTitle,
  // Defaults to idle; the caller passes "compose" once a draft exists so the
  // mascot lifts its head from the composer to face the user.
  activity = "idle",
  children
}: {
  title: string;
  // Optional element rendered directly under the title in the same
  // grid cell so it can sit a few pixels below the greeting without
  // inheriting the very large row-gap reserved for the hero composer.
  belowTitle?: JSX.Element;
  activity?: WuuMascotActivity;
  children: JSX.Element;
}): JSX.Element {
  return (
    <section className="empty-home" data-wuu-component="empty-session">
      <div className="empty-home-inner session-flow">
        <div className="empty-home-header">
          {/* The wuu blobatar greeting, idling with its gaze lowered toward
              the composer below and lifting its head (activity "compose") as
              soon as a draft exists; see .empty-home-mascot in turns.css. Hue
              is pinned the same way default avatars are, so the mark matches
              the pairing hero on the phone companion. Shape is pinned to
              "round" (the first silhouette, shape < 0.28) with the aspect
              ratio locked to 1, and the plate is off (none), so the mascot is
              a pure round ball floating on the paper. */}
          <WuuMascot
            className="empty-home-mascot"
            activity={activity}
            aria-hidden="true"
          />
          <h2>{title}</h2>
          {belowTitle ?? null}
        </div>
        {children}
      </div>
    </section>
  );
}
