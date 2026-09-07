import {
  RemoteClient,
  pair,
  type Credentials,
  type HostState,
  type ServerRequest,
  type ServerRequestResult,
} from "@wuu/remote-core";
import type {
  ChannelRoomPreferences,
  DesktopPlatform,
  DesktopProject,
  InitializeResult,
  LanguagePreference,
  MessageFlowFontSize,
  PluginConflictPreferences,
  ProjectListResult,
  RunningThreadSnapshot,
  ServerEvent,
  ThemePreference,
  VoiceInputSettings,
  WuuDesktopApi,
} from "@wuu/protocol";

export type WebConnectionSnapshot = {
  phase: "connecting" | "connected" | "reconnecting" | "restoring" | "error" | "disconnected";
  revision: number;
  error?: string;
};

type ServerEventListener = (event: ServerEvent) => void;
type RunningListener = (snapshot: RunningThreadSnapshot[]) => void;
type PreferenceListener<T> = (value: T) => void;

const THEME_KEY = "wuu.web.theme";
const LANGUAGE_KEY = "wuu.web.language";
const MESSAGE_SIZE_KEY = "wuu.web.message-size";
const CHANNEL_ROOM_PREFERENCES_KEY = "wuu.channels.roomPreferences";
const PLUGIN_CONFLICT_PREFERENCES_KEY = "wuu.web.plugin-conflict-preferences";
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

function storedChannelRoomPreferences(): ChannelRoomPreferences {
  try {
    const parsed = JSON.parse(localStorage.getItem(CHANNEL_ROOM_PREFERENCES_KEY) ?? "null") as
      | Partial<ChannelRoomPreferences>
      | null;
    return {
      pinnedRoomIDs: Array.isArray(parsed?.pinnedRoomIDs) ? parsed.pinnedRoomIDs : [],
      archivedRoomIDs: Array.isArray(parsed?.archivedRoomIDs) ? parsed.archivedRoomIDs : [],
    };
  } catch {
    return { pinnedRoomIDs: [], archivedRoomIDs: [] };
  }
}

function storedPluginConflictPreferences(): PluginConflictPreferences {
  try {
    const parsed = JSON.parse(localStorage.getItem(PLUGIN_CONFLICT_PREFERENCES_KEY) ?? "null");
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed as PluginConflictPreferences
      : {};
  } catch {
    return {};
  }
}


export class UnavailableHostOperationError extends Error {
  readonly code = "host_operation_unavailable";
  constructor(readonly operation: keyof WuuDesktopApi) {
    super(`${operation} is unavailable in this browser host`);
    this.name = "UnavailableHostOperationError";
  }
}

const unavailableWebMethods = [
  "createBlankProject",
  "chooseProjectFolder",
  "removeProject",
  "cleanupProjectState",
  "relocateProject",
  "selectNoProject",
  "gitStatus",
  "listGitChanges",
  "readGitFileDiff",
  "checkoutGitBranch",
  "createCheckoutGitBranch",
  "commitGitChanges",
  "generateCommitMessage",
  "createPullRequest",
  "listWorkspaceFiles",
  "listWorkspaceDirectory",
  "readWorkspaceFile",
  "writeWorkspaceFile",
  "resolveWorkspaceFileReference",
  "startTerminalSession",
  "writeTerminalSession",
  "resizeTerminalSession",
  "stopTerminalSession",
  "getBuildInfo",
  "startSpeechRecognition",
  "stopSpeechRecognition",
  "installPluginPackage",
  "loadPluginDesktopModule",
  "loadPluginIcon",
  "readSkillContent",
  "listInstructionFiles",
  "getRemoteControlSnapshot",
  "setRemoteRelay",
  "setRemoteHostEnabled",
  "startRemotePairing",
  "removeRemoteDevice",
  "updateVoiceInputSettings",
  "openVoicePrivacySettings",
  "listCodexPets",
  "updateCodexPetSettings",
  "updateCodexPetRuntime",
  "updateCodexPetHints",
  "revealSession",
  "revealWorkspaceItem",
  "showWorkspaceItemMenu",
  "popOutSession",
  "popOutClosed",
] as const satisfies readonly (keyof WuuDesktopApi)[];

// An explicit, type-checked list keeps new protocol members from silently
// becoming pretend functions. Every unsupported action rejects consistently.
const unavailableWebActions = Object.fromEntries(
  unavailableWebMethods.map((method) => [method, async () => {
    throw new UnavailableHostOperationError(method);
  }]),
) as { [K in typeof unavailableWebMethods[number]]: (...args: unknown[]) => Promise<never> };

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
  private defaultWorkdir = "";
  private connection: WebConnectionSnapshot = { phase: "connecting", revision: 0 };
  private readonly connectionListeners = new Set<() => void>();
  private readonly restoreListeners = new Set<() => Promise<void>>();
  private attachSync: Promise<void> = Promise.resolve();
  private stopped = false;

  getConnectionSnapshot = (): WebConnectionSnapshot => this.connection;
  subscribeConnection = (listener: () => void): (() => void) => {
    this.connectionListeners.add(listener);
    return () => this.connectionListeners.delete(listener);
  };

  private setConnection(phase: WebConnectionSnapshot["phase"], error?: string): number {
    this.connection = { phase, revision: this.connection.revision + 1, ...(error ? { error } : {}) };
    for (const listener of this.connectionListeners) listener();
    return this.connection.revision;
  }

  private clearPendingRequests(): void {
    for (const pending of this.pendingServerRequests.values()) {
      pending.resolve({ error: { code: "connection_replaced", message: "The remote connection was replaced" } });
    }
    this.pendingServerRequests.clear();
  }

  private async synchronize(first: boolean): Promise<void> {
    const revision = this.setConnection(first ? "connecting" : "restoring");
    try {
      const workspace = await this.client.call<{ current: string }>("workspace/list", undefined, 30_000);
      if (this.connection.revision !== revision || this.stopped) return;
      if (!workspace.current) throw new Error("Remote host did not provide a workspace");
      this.defaultWorkdir = workspace.current;
      if (!first) await Promise.all([...this.restoreListeners].map((restore) => restore()));
      if (this.connection.revision !== revision || this.stopped) return;
      this.setConnection("connected");
    } catch (error) {
      if (this.connection.revision !== revision || this.stopped) return;
      this.setConnection("error", error instanceof Error ? error.message : String(error));
      throw error;
    }
  }

  retryRestore = async (): Promise<void> => {
    if (!this.client.isAttached() || this.connection.phase === "restoring") return;
    await this.synchronize(false);
  };
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
        const first = !this.attachedOnce;
        this.attachedOnce = true;
        if (!resumed) this.clearPendingRequests();
        this.attachSync = this.synchronize(first);
        // connect() awaits the first sync; later failures are visible through
        // the connection snapshot and an explicit retry, never a page reload.
        void this.attachSync.catch(() => {});
      },
      onDetach: () => {
        if (!this.stopped) this.setConnection("reconnecting");
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
    await this.attachSync;
    this.hostState = this.client.latestState();
  }

  async disconnect(): Promise<void> {
    this.stopped = true;
    this.setConnection("disconnected");
    this.clearPendingRequests();
    await this.client.stop();
  }

  install(): void {
    window.wuu = this.api;
  }

  private async call<T>(method: string, params?: unknown): Promise<T> {
    // Do not queue UI actions while offline: a delayed send after reconnect
    // would execute an instruction the user already saw fail.
    if (this.stopped || !this.client.isAttached()) throw new Error("Remote host is disconnected");
    if (this.connection.phase === "restoring" && ![
      "initialize", "thread/list", "thread/listArchived", "thread/resume", "user-question/list",
    ].includes(method)) throw new Error("Remote workspace is restoring; retry after it reconnects");
    const revision = this.connection.revision;
    const result = await this.client.call<T>(method, params, 30_000);
    if (this.stopped || this.connection.revision !== revision) {
      throw new Error("Remote connection changed while the request was in flight");
    }
    return result;
  }

  private workdir(): string {
    return this.defaultWorkdir || this.hostState?.host.workdir || "";
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
    const target: WuuDesktopApi = {
      ...unavailableWebActions,
      hostKind: "web",
      onRuntimeRestore: (listener) => {
        this.restoreListeners.add(listener);
        return () => this.restoreListeners.delete(listener);
      },
      unsupportedMethods: unavailableWebMethods,
      platform: browserPlatform(),
      initialThemePreference: storedTheme(),
      initialLanguagePreference: storedLanguage(),
      initialSystemLocale: navigator.language,
      initialVoiceInputSettings,
      initialChannelRoomPreferences: storedChannelRoomPreferences(),
      initialMessageFlowFontSize: storedMessageSize(),
      popOutInit: () => ({ kind: null, threadID: null, context: null }),

      listProjects: async () => this.projectState(),
      selectProject: async (id) => {
        if (id !== this.projectID) throw new Error("Unknown remote workspace");
        return this.projectState();
      },

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

      listChannelRooms: () => this.call("channel/room/list"),
      listNamedAgents: () => this.call("channel/agent/list"),

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
      requeueTurn: (threadId, steerId) =>
        this.call("turn/requeue", { thread_id: threadId, steer_id: steerId }),
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

      listEngines: () => this.call("engine/list"),
      refreshModelCatalog: () => this.call("config/model-catalog/refresh"),
      listMCPServers: () => this.call("mcp/list"),
      startXAILogin: () => this.call("auth/xai/login/start"),
      listSkills: () => this.call("skill/list"),
      getNamedAgentInsights: () => this.call("channel/agent/insights"),
      bootstrapChannels: () => this.call("channel/bootstrap"),
      getChannelHumanMentionStatus: () => this.call("channel/human-mention/status"),
      ackChannelHumanMentions: () => this.call("channel/human-mention/ack"),
      getSessionOrganization: () => this.call("sessionOrganization/list"),
      getSettingsUsage: () => this.call("settings/usage"),
      refreshExtensionCatalog: () => this.call("extension/catalog/refresh"),
      updateAdvancedSettings: (params) => this.call("config/advanced/update", params),
      updateGeneralSettings: (params) => this.call("config/general/update", params),
      updateEngines: (params) => this.call("engine/update", params),
      updateExtensionPackage: (params) => this.call("extension/package/update", params),
      getPluginSetting: (params) => this.call("plugin/setting/get", params),
      setPluginSetting: (params) => this.call("plugin/setting/set", params),
      getPluginDiagnostics: (params) => this.call("plugin/diagnostics/list", params),
      getPluginStorage: (params) => this.call("plugin/storage/get", params),
      setPluginStorage: (params) => this.call("plugin/storage/set", params),
      requestPluginRuntime: (params) => this.call("plugin/client/request", params),
      createNamedAgent: (params) => this.call("channel/agent/create", params),
      updateNamedAgent: (params) => this.call("channel/agent/update", params),
      deleteNamedAgent: (params) => this.call("channel/agent/delete", params),
      startNamedAgent: (params) => this.call("channel/agent/start", params),
      resetNamedAgent: (params) => this.call("channel/agent/reset", params),
      resolveChannelAgentCreation: (params) => this.call("channel/agent-creation/resolve", params),
      createChannelRoom: (params) => this.call("channel/room/create", params),
      openChannelDirectMessage: (params) => this.call("channel/direct-message/open", params),
      updateChannelRoom: (params) => this.call("channel/room/update", params),
      deleteChannelRoom: (params) => this.call("channel/room/delete", params),
      markChannelRoomRead: (params) => this.call("channel/room/read", params),
      listChannelMessages: (params) => this.call("channel/message/list", params),
      sendChannelMessage: (params) => this.call("channel/message/send", params),
      createChannelTask: (params) => this.call("channel/task/create", params),
      updateChannelTask: (params) => this.call("channel/task/update", params),
      readManagedProcess: (params) => this.call("process/read", params),
      holdUserQuestion: (request_id) => this.call("user-question/hold", { request_id }),
      loadCodexModels: (provider) => this.call("config/codex/models", { provider }),
      removePluginPackage: (id) => this.call("plugin/package/remove", { id }),
      connectMCPServer: (name) => this.call("mcp/connect", { name }),
      disconnectMCPServer: (name) => this.call("mcp/disconnect", { name }),
      refreshMCPServer: (name) => this.call("mcp/refresh", { name }),
      startMCPAuth: (name) => this.call("mcp/auth/start", { name }),
      getMCPAuthStatus: (name) => this.call("mcp/auth/status", { name }),
      removeMCPAuth: (name) => this.call("mcp/auth/remove", { name }),
      pollXAILogin: (login_id) => this.call("auth/xai/login/poll", { login_id }),
      cancelXAILogin: (login_id) => this.call("auth/xai/login/cancel", { login_id }),
      listActivities: (thread_id) => this.call("activity/list", { thread_id }),
      getThreadContextComposition: (thread_id) => this.call("thread/context-composition", { thread_id }),
      polishText: (text) => this.call("text/polish", { text }),
      createSessionFolder: (name) => this.call("sessionFolder/create", { name }),
      reorderSessionFolders: (ids) => this.call("sessionFolder/reorder", { ids }),
      deleteSessionFolder: (id) => this.call("sessionFolder/delete", { id }),
      listManagedProcesses: (thread_id) => this.call("process/list", { thread_id }),
      takeoverActivity: (thread_id, activity_id) => this.call("activity/takeover", { thread_id, activity_id }),
      releaseActivity: (thread_id, activity_id) => this.call("activity/release", { thread_id, activity_id }),
      stopActivity: (thread_id, activity_id) => this.call("activity/stop", { thread_id, activity_id }),
      updateRuntimeSettings: (provider, model, effort, connection, variant, permissionMode, threadId) =>
        this.call("config/model/update", {
          ...(provider ? { provider } : {}),
          ...(model ? { model } : {}),
          ...(threadId ? { thread_id: threadId } : {}),
          ...connection,
          ...(effort === undefined ? {} : { effort }),
          ...(variant === undefined ? {} : { variant }),
          ...(permissionMode === undefined ? {} : { permission_mode: permissionMode }),
        }),
      removeProvider: (provider, options) => this.call("config/provider/remove", {
        provider, fallback_provider: options?.fallbackProvider, fallback_model: options?.fallbackModel,
      }),
      finishMCPAuth: (name, state, code) => this.call("mcp/auth/finish", { name, state, code }),
      forkThread: (thread_id, turn_id, item_id, mode, target) =>
        this.call("thread/fork", { thread_id, turn_id, item_id, mode, target }),
      editThreadMessage: (thread_id, turn_id, item_id) =>
        this.call("thread/edit-message", { thread_id, turn_id, item_id }),
      renameSessionFolder: (id, name) => this.call("sessionFolder/update", { id, name }),
      updateThreadOrganization: (thread_id, folder_id) =>
        this.call("thread/organization/update", { thread_id, folder_id }),
      writeManagedProcess: (thread_id, process_id, input) =>
        this.call("process/write", { thread_id, process_id, input }),
      resizeManagedProcess: (thread_id, process_id, cols, rows) =>
        this.call("process/resize", { thread_id, process_id, cols, rows }),
      stopManagedProcess: (thread_id, process_id) =>
        this.call("process/stop", { thread_id, process_id }),
      // Native event sources do not exist in a browser. These subscriptions
      // are inert; their actions are explicitly unavailable below.
      onSpeechRecognitionEvent: () => () => {},
      onRemoteControlEvent: () => () => {},
      onCodexPetJumpRequest: () => () => {},
      onTerminalEvent: () => () => {},
      onWindowResizeState: () => () => {},
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
      onVoiceInputSettingsChange: (listener: PreferenceListener<VoiceInputSettings>) => {
        this.voiceListeners.add(listener);
        return () => this.voiceListeners.delete(listener);
      },
      getMessageFlowFontSize: async () => storedMessageSize(),
      setMessageFlowFontSize: async (fontSize: MessageFlowFontSize) => {
        localStorage.setItem(MESSAGE_SIZE_KEY, String(fontSize));
        return { ok: true, fontSize };
      },
      updateChannelRoomPreferences: async (preferences: ChannelRoomPreferences) => {
        localStorage.setItem(CHANNEL_ROOM_PREFERENCES_KEY, JSON.stringify(preferences));
        return preferences;
      },
      getPluginConflictPreferences: async () => storedPluginConflictPreferences(),
      setPluginConflictPreference: async (key: string, pluginId: string) => {
        const preferences = { ...storedPluginConflictPreferences(), [key]: pluginId };
        localStorage.setItem(PLUGIN_CONFLICT_PREFERENCES_KEY, JSON.stringify(preferences));
        return preferences;
      },
      openExternal: async (url: string) => {
        const parsed = new URL(url);
        if (parsed.protocol !== "https:" && parsed.protocol !== "http:") {
          throw new Error("Only HTTP and HTTPS links can be opened");
        }
        window.open(parsed.href, "_blank", "noopener,noreferrer");
      },
      showSystemNotification: async ({ title, body }: { title: string; body: string }) => {
        if ("Notification" in window && Notification.permission === "granted") {
          new Notification(title, { body });
          return { shown: true };
        }
        return { shown: false };
      },
    };

    return target;
  }
}
