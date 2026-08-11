import type { ProjectionFrame } from "@wuu-v2/contracts";
import { Service, type Context, type Plugin } from "@wuu-v2/kernel";

export class ProjectionFeedService extends Service {
  constructor(ctx: Context) {
    super(ctx, "projectionFeed");
  }

  async snapshot(sessionId: string): Promise<ProjectionFrame> {
    const events = await this.ctx.sessions.load(sessionId);
    const projections = this.ctx.projections.buildEvents(events).map((projection) => ({
      key: projection.key,
      seq: projection.seq,
      ...(projection.value === undefined ? {} : { value: projection.value }),
    }));
    return {
      sessionId,
      lastDurableSeq: events.at(-1)?.seq ?? 0,
      projections,
    };
  }

  follow(sessionId: string, listener: (frame: ProjectionFrame) => void): () => void {
    let active = true;
    let dirty = true;
    let running = false;
    const pump = async () => {
      if (running) return;
      running = true;
      try {
        while (active && dirty) {
          dirty = false;
          const frame = await this.snapshot(sessionId);
          if (active) listener(frame);
        }
      } catch (error) {
        if (active) this.ctx.logger.error(error);
      } finally {
        running = false;
        if (active && dirty) void pump();
      }
    };
    const stop = this.ctx.sessions.subscribe(sessionId, () => {
      dirty = true;
      void pump();
    });
    const stopProjectionChanges = this.ctx.projections.subscribe(() => {
      dirty = true;
      void pump();
    });
    void pump();
    return () => {
      active = false;
      stop();
      stopProjectionChanges();
    };
  }
}

declare module "cordis" {
  interface Context {
    projectionFeed: ProjectionFeedService;
  }
}

export const projectionFeedPlugin: Plugin = function projectionFeed(ctx) {
  new ProjectionFeedService(ctx);
};

projectionFeedPlugin.inject = ["projections", "sessions"];
projectionFeedPlugin.provide = "projectionFeed";
