import {
  RemoteClient,
  pair,
  type Credentials,
  type HostState,
  type ServerRequest,
  type ServerRequestResult,
} from "@wuu/remote-core";
import type {
  DesktopPlatform,
  DesktopProject,
  InitializeResult,
  LanguagePreference,
  MessageFlowFontSize,
  ProjectListResult,
  RunningThreadSnapshot,
  ServerEvent,
  ThemePreference,
  VoiceInputSettings,
  WuuDesktopApi,
} from "@wuu/protocol";

type ServerEventListener = (event: ServerEvent) => void;
type RunningListener = (snapshot: RunningThreadSnapshot[]) => void;
type PreferenceListener<T> = (value: T) => void;

const THEME_KEY = "wuu.web.theme";
const LANGUAGE_KEY = "wuu.web.language";
const MESSAGE_SIZE_KEY = "wuu.web.message-size";
const DEFAULT_MESSAGE_SIZE = 16;

function basename(path: string): string {
  const trimmed = path.replace(/[\\/]+$/, "");
  return trimmed.split(/[\\/]/).pop() || path || "Workspace";
}

function browserPlatform(): DesktopPlatform {
  const ua = navigator.userAgent;
  if (ua.includes("Windows")) return "win32";
  if (ua.includes("Macintosh") || ua.includes("Mac OS")) return "darwin";
  return "linux";
}

function storedTheme(): ThemePreference {
  const value = localStorage.getItem(THEME_KEY);
  return value === "light" || value === "dark" || value === "system" ? value : "system";
}

function storedLanguage(): LanguagePreference {
  const value = localStorage.getItem(LANGUAGE_KEY);
  return value === "zh-CN" || value === "en-US" || value === "system" ? value : "system";
}

function storedMessageSize(): MessageFlowFontSize {
  const value = Number(localStorage.getItem(MESSAGE_SIZE_KEY));
  return (Number.isFinite(value) ? value : DEFAULT_MESSAGE_SIZE) as MessageFlowFontSize;
}

/** Browser host adapter for the shared desktop renderer. */
export class RemoteDesktopBridge {
  private readonly client: RemoteClient;
  private readonly serverListeners = new Set<ServerEventListener>();
  private readonly runningListeners = new Set<RunningListener>();
  private readonly themeListeners = new Set<PreferenceListener<ThemePreference>>();
  private readonly languageListeners = new Set<PreferenceListener<LanguagePreference>>();
  private readonly voiceListeners = new Set<PreferenceListener<VoiceInputSettings>>();
  private readonly pendingServerRequests = new Map<
    string,
    { resolve: (result: ServerRequestResult) => void }
  >();
  private hostState: HostState | null = null;
  private attachedOnce = false;
  private readonly projectID: string;

  readonly api: WuuDesktopApi;

  constructor(credentials: Credentials) {
    this.projectID = `remote:${credentials.host_pub}`;
    this.client = new RemoteClient(credentials, {
      // Do not use mobile_chat: the shared workbench needs the full event
      // stream, including tools, activities, usage, and lifecycle events.
      onNotification: (method, params) => {
        this.emitServerEvent({
          workdir: this.workdir(),
          kind: "notification",
          message: { method, params },
        });
      },
      onServerRequest: (request) => this.handleServerRequest(request),
      onState: (state) => {
        this.hostState = state;
        this.emitRunning();
      },
      onAttach: ({ resumed }) => {
        if (this.attachedOnce && !resumed) {
          // Replay preserves renderer state. A fresh app-server does not yet
          // have a reconciliation handshake, so reload instead of showing
          // plausible but stale state.
          window.location.reload();
          return;
        }
        this.attachedOnce = true;
      },
    });
    this.api = this.createApi();
  }

  static async pair(uri: string, deviceName: string): Promise<Credentials> {
    return pair(uri, deviceName);
  }

  async connect(): Promise<void> {
    this.client.start();
    await this.client.waitAttached(30_000);
    this.hostState = this.client.latestState();
  }

  async disconnect(): Promise<void> {
    await this.client.stop();
  }

  install(): void {
    window.wuu = this.api;
  }

  private call<T>(method: string, params?: unknown): Promise<T> {
    return this.client.call<T>(method, params, 30_000);
  }

  private workdir(): string {
    return this.hostState?.host.workdir ?? "";
  }

  private projectState(): ProjectListResult {
    const path = this.workdir();
    const now = new Date().toISOString();
    const project: DesktopProject = {
      id: this.projectID,
      name: basename(path),
      path,
      created_at: now,
      updated_at: now,
    };
    return {
      projects: [project],
      active_project_id: project.id,
      active_context: { kind: "project", project_id: project.id, cwd: path },
    };
  }

  private runningSnapshot(): RunningThreadSnapshot[] {
    const workdir = this.workdir();
    return (this.hostState?.running ?? []).map((turn) => ({
      workdir,
      thread_id: turn.thread_id,
    }));
  }

  private emitRunning(): void {
    const snapshot = this.runningSnapshot();
    for (const listener of this.runningListeners) listener(snapshot);
  }

  private emitServerEvent(event: ServerEvent): void {
    for (const listener of this.serverListeners) listener(event);
  }

  private handleServerRequest(request: ServerRequest): Promise<ServerRequestResult> {
    return new Promise((resolve) => {
      const id = String(request.id);
      this.pendingServerRequests.set(id, { resolve });
      this.emitServerEvent({
        workdir: this.workdir(),
        kind: "server-request",
        message: { id, method: request.method, params: request.params },
      });
    });
  }

  private createApi(): WuuDesktopApi {
    const initialVoiceInputSettings: VoiceInputSettings = {
      polish_enabled: false,
      language: "system",
    };
    const target: Partial<WuuDesktopApi> & Record<string, unknown> = {
      platform: browserPlatform(),
      initialThemePreference: storedTheme(),
      initialLanguagePreference: storedLanguage(),
      initialSystemLocale: navigator.language,
      initialVoiceInputSettings,
      initialMessageFlowFontSize: storedMessageSize(),
      popOutInit: () => ({ kind: null, threadID: null, context: null }),

      listProjects: async () => this.projectState(),
      selectProject: async () => this.projectState(),
      selectNoProject: async () => this.projectState(),

      initialize: async () => {
        const result = await this.call<InitializeResult>("initialize", {
          client: { name: "wuu-web", version: "0.1" },
        });
        // Renderer extension module URLs are still Electron-owned. Keep the
        // shared renderer from activating them until a public web loader exists.
        return {
          ...result,
          extension_inventory: [],
          features: { ...result.features, browser: false },
        };
      },
      listThreads: (cwd?: string) => this.call("thread/list", cwd ? { cwd } : undefined),
      listAllThreads: () => this.call("thread/listAll"),
      listArchivedThreads: () => this.call("thread/listArchived"),
      resumeThread: (sessionId?: string) =>
        this.call("thread/resume", { session_id: sessionId ?? "" }),
      startThread: (params = {}) => this.call("thread/start", params),
      searchThreads: (query: string, limit?: number) =>
        this.call("thread/search", { query, limit }),
      getThreadPreview: (threadId: string, limit?: number) =>
        this.call("thread/preview", { thread_id: threadId, limit }),
      pinThread: (threadId: string, pinned: boolean) =>
        this.call("thread/pin", { thread_id: threadId, pinned }),
      renameThread: (threadId: string, title: string) =>
        this.call("thread/rename", { thread_id: threadId, title }),
      archiveThread: (threadId: string, archived: boolean, force?: boolean) =>
        this.call("thread/archive", { thread_id: threadId, archived, force }),
      deleteThread: (threadId: string) => this.call("thread/delete", { thread_id: threadId }),
      compactThread: (threadId: string) => this.call("thread/compact/start", { thread_id: threadId }),

      startTurn: (threadId, prompt, images, files, permissionMode, activeDocument, contentParts) =>
        this.call("turn/start", {
          thread_id: threadId,
          prompt,
          images: images ?? [],
          files: files ?? [],
          ...(permissionMode === undefined ? {} : { permission_mode: permissionMode }),
          ...(activeDocument === undefined ? {} : { active_document: activeDocument }),
          ...(contentParts === undefined ? {} : { content_parts: contentParts }),
        }),
      queueTurn: (threadId, prompt, images, clientId, files, permissionMode, activeDocument, contentParts) =>
        this.call("turn/queue", {
          thread_id: threadId,
          prompt,
          images: images ?? [],
          files: files ?? [],
          client_id: clientId,
          ...(permissionMode === undefined ? {} : { permission_mode: permissionMode }),
          ...(activeDocument === undefined ? {} : { active_document: activeDocument }),
          ...(contentParts === undefined ? {} : { content_parts: contentParts }),
        }),
      updateQueuedTurn: (threadId, queueId, prompt, images, files, contentParts) =>
        this.call("turn/update-queued", {
          thread_id: threadId,
          queue_id: queueId,
          prompt,
          images: images ?? [],
          files: files ?? [],
          content_parts: contentParts,
        }),
      dequeueTurn: (threadId, queueId) =>
        this.call("turn/dequeue", { thread_id: threadId, queue_id: queueId }),
      steerTurn: (threadId, expectedTurnId, prompt, images, clientId, files, activeDocument, contentParts) =>
        this.call("turn/steer", {
          thread_id: threadId,
          expected_turn_id: expectedTurnId,
          prompt,
          images: images ?? [],
          files: files ?? [],
          client_id: clientId,
          active_document: activeDocument,
          content_parts: contentParts,
        }),
      unsteerTurn: (threadId, steerId) =>
        this.call("turn/unsteer", { thread_id: threadId, steer_id: steerId }),
      interruptTurn: (threadId) => this.call("turn/interrupt", { thread_id: threadId }),

      listUserQuestions: (threadId?: string) =>
        this.call("user-question/list", threadId ? { thread_id: threadId } : undefined),
      answerUserQuestion: (requestId, answer) =>
        this.call("user-question/respond", { request_id: requestId, answer }),
      cancelUserQuestion: (requestId) =>
        this.call("user-question/cancel", { request_id: requestId }),

      onServerEvent: (listener: ServerEventListener) => {
        this.serverListeners.add(listener);
        return () => this.serverListeners.delete(listener);
      },
      getRunningThreadsSnapshot: async () => this.runningSnapshot(),
      onRunningThreadsChanged: (listener: RunningListener) => {
        this.runningListeners.add(listener);
        return () => this.runningListeners.delete(listener);
      },
      respondToServerRequest: async (id: string, result: unknown) => {
        const pending = this.pendingServerRequests.get(id);
        this.pendingServerRequests.delete(id);
        pending?.resolve({ result });
      },
      rejectServerRequest: async (id: string, message: string) => {
        const pending = this.pendingServerRequests.get(id);
        this.pendingServerRequests.delete(id);
        pending?.resolve({ error: { code: "rejected", message } });
      },

      listEngines: async () => ({ engines: [] }),
      gitStatus: async () => ({ is_repo: false, dirty_count: 0 }),
      getThemePreference: async () => storedTheme(),
      setThemePreference: async (theme: ThemePreference) => {
        localStorage.setItem(THEME_KEY, theme);
        for (const listener of this.themeListeners) listener(theme);
        return { ok: true, theme };
      },
      onThemePreferenceChange: (listener: PreferenceListener<ThemePreference>) => {
        this.themeListeners.add(listener);
        return () => this.themeListeners.delete(listener);
      },
      getLanguagePreference: async () => storedLanguage(),
      setLanguagePreference: async (language: LanguagePreference) => {
        localStorage.setItem(LANGUAGE_KEY, language);
        for (const listener of this.languageListeners) listener(language);
        return { ok: true, language };
      },
      onLanguagePreferenceChange: (listener: PreferenceListener<LanguagePreference>) => {
        this.languageListeners.add(listener);
        return () => this.languageListeners.delete(listener);
      },
      getVoiceInputSettings: async () => ({
        settings: initialVoiceInputSettings,
        microphone_permission: "unavailable",
        speech_permission: "unavailable",
      }),
      updateVoiceInputSettings: async (settings: VoiceInputSettings) => {
        for (const listener of this.voiceListeners) listener(settings);
        return settings;
      },
      onVoiceInputSettingsChange: (listener: PreferenceListener<VoiceInputSettings>) => {
        this.voiceListeners.add(listener);
        return () => this.voiceListeners.delete(listener);
      },
      getMessageFlowFontSize: async () => storedMessageSize(),
      setMessageFlowFontSize: async (fontSize: MessageFlowFontSize) => {
        localStorage.setItem(MESSAGE_SIZE_KEY, String(fontSize));
        return { ok: true, fontSize };
      },
      openExternal: async (url: string) => {
        window.open(url, "_blank", "noopener,noreferrer");
      },
      showSystemNotification: async ({ title, body }: { title: string; body: string }) => {
        if ("Notification" in window && Notification.permission === "granted") {
          new Notification(title, { body });
          return { shown: true };
        }
        return { shown: false };
      },
    };

    return new Proxy(target, {
      get(object, property, receiver) {
        if (Reflect.has(object, property)) return Reflect.get(object, property, receiver);
        if (typeof property === "string" && property.startsWith("on")) {
          return () => () => {};
        }
        return () => Promise.reject(new Error(
          `${String(property)} is not available from the web client yet`,
        ));
      },
    }) as WuuDesktopApi;
  }
}
