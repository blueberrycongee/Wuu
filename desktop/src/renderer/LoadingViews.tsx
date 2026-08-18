import { Blobatar } from "blobatar/react";
import "blobatar/motion.css";
import mascotFace from "./assets/mascot-face.png";
import { useI18n } from "./i18n";
import { AVATAR_HUES, avatarHueIndex } from "./DefaultAvatar";

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
            <img
              className="wuu-launch-carving"
              src={mascotFace}
              alt=""
              draggable={false}
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
  children
}: {
  title: string;
  // Optional element rendered directly under the title in the same
  // grid cell so it can sit a few pixels below the greeting without
  // inheriting the very large row-gap reserved for the hero composer.
  belowTitle?: JSX.Element;
  children: JSX.Element;
}): JSX.Element {
  return (
    <section className="empty-home" data-wuu-component="empty-session">
      <div className="empty-home-inner session-flow">
        <div className="empty-home-header">
          {/* The wuu blobatar greeting, always idling; see .empty-home-mascot
              in turns.css. Hue is pinned the same way default avatars are, so
              the mark matches the pairing hero on the phone companion. Shape
              is pinned to "round" (the first silhouette, shape < 0.28) with
              the aspect ratio locked to 1, and the plate is off (none), so
              the mascot is a pure round ball floating on the paper. */}
          <Blobatar
            className="empty-home-mascot"
            name="wuu"
            hue={AVATAR_HUES[avatarHueIndex("wuu")]}
            background={false}
            traits={{ shape: 0.2, "body.ratio": 0.5 }}
            animate="always"
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
