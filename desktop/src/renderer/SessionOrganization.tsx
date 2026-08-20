import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import type { SessionOrganizationGroup } from "../shared/protocol";
import type { ThreadSummary } from "./AppState";
import { showErrorToast } from "./Toast";
import { translateCurrent } from "./i18n";

export type SessionGroup = Pick<SessionOrganizationGroup, "id" | "name">;
export const SESSION_FOLDER_DRAG_MIME = "application/x-wuu-session-folder";
export type SessionOrganization = {
  folders: SessionGroup[];
  pinGroups: SessionGroup[];
  folderByThreadID: Record<string, string>;
  pinGroupByThreadID: Record<string, string>;
};

export const emptySessionOrganization: SessionOrganization = {
  folders: [], pinGroups: [], folderByThreadID: {}, pinGroupByThreadID: {},
};

export function parseSessionOrganization(value: unknown): SessionOrganization {
  if (!value || typeof value !== "object" || Array.isArray(value)) return emptySessionOrganization;
  const record = value as Record<string, unknown>;
  const groups = (candidate: unknown): SessionGroup[] => Array.isArray(candidate)
    ? candidate.flatMap((item) => {
        if (!item || typeof item !== "object" || Array.isArray(item)) return [];
        const entry = item as Record<string, unknown>;
        return typeof entry.id === "string" && typeof entry.name === "string"
          ? [{ id: entry.id, name: entry.name }]
          : [];
      })
    : [];
  return {
    folders: groups(record.folders),
    pinGroups: groups(record.pin_groups ?? record.pinGroups),
    folderByThreadID: {},
    pinGroupByThreadID: {},
  };
}

export type SessionOrganizationController = SessionOrganization & {
  createFolder: (name: string, threadID?: string) => Promise<void>;
  createPinGroup: (name: string, threadID?: string) => Promise<void>;
  renameFolder: (id: string, name: string) => Promise<void>;
  renamePinGroup: (id: string, name: string) => Promise<void>;
  reorderFolder: (id: string, direction: -1 | 1) => Promise<void>;
  reorderPinGroup: (id: string, direction: -1 | 1) => Promise<void>;
  deleteFolder: (id: string) => Promise<void>;
  deletePinGroup: (id: string) => Promise<void>;
  moveThreadToFolder: (threadID: string, folderID?: string) => Promise<void>;
  moveThreadToPinGroup: (threadID: string, pinGroupID?: string) => Promise<void>;
};

export function useSessionOrganization(threads: ThreadSummary[]): SessionOrganizationController {
  const [groups, setGroups] = useState<Pick<SessionOrganization, "folders" | "pinGroups">>({ folders: [], pinGroups: [] });
  const [optimisticFolders, setOptimisticFolders] = useState<Record<string, string>>({});
  const [optimisticPins, setOptimisticPins] = useState<Record<string, string>>({});

  useEffect(() => {
    if (!window.wuu?.getSessionOrganization) return;
    let cancelled = false;
    void window.wuu.getSessionOrganization().then((result) => {
      if (cancelled) return;
      const parsed = parseSessionOrganization(result.organization);
      setGroups({ folders: parsed.folders, pinGroups: parsed.pinGroups.filter((group) => group.id !== "default") });
    }).catch((error) => {
      if (!cancelled) showErrorToast(error, translateCurrent("sessionOrganization.loadFailed"), "session-organization-load");
    });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    const byID = new Map(threads.map((thread) => [thread.id, thread]));
    setOptimisticFolders((current) => clearReconciledAssignments(current, byID, "folder_id"));
    setOptimisticPins((current) => clearReconciledAssignments(current, byID, "pin_group_id"));
  }, [threads]);

  const assignments = useMemo(() => {
    const folderByThreadID: Record<string, string> = {};
    const pinGroupByThreadID: Record<string, string> = {};
    for (const thread of threads) {
      const folderID = Object.hasOwn(optimisticFolders, thread.id) ? optimisticFolders[thread.id] : thread.folder_id;
      const pinGroupID = Object.hasOwn(optimisticPins, thread.id) ? optimisticPins[thread.id] : thread.pin_group_id;
      if (folderID) folderByThreadID[thread.id] = folderID;
      if (pinGroupID && pinGroupID !== "default") pinGroupByThreadID[thread.id] = pinGroupID;
    }
    return { folderByThreadID, pinGroupByThreadID };
  }, [optimisticFolders, optimisticPins, threads]);

  async function moveThread(threadID: string, axis: "folder" | "pin", value?: string): Promise<void> {
    const assignment = value ?? "";
    const setOptimistic = axis === "folder" ? setOptimisticFolders : setOptimisticPins;
    setOptimistic((current) => ({ ...current, [threadID]: assignment }));
    try {
      if (axis === "folder") {
        await window.wuu.updateThreadOrganization(threadID, assignment, undefined);
      } else {
        await window.wuu.updateThreadOrganization(threadID, undefined, assignment);
      }
    } catch (error) {
      setOptimistic((current) => clearOptimisticAssignment(current, threadID, assignment));
      showErrorToast(
        error,
        translateCurrent(axis === "folder" ? "sessionOrganization.moveToFolderFailed" : "sessionOrganization.moveToPinGroupFailed"),
      );
    }
  }

  return {
    ...groups,
    ...assignments,
    createFolder: async (name, threadID) => {
      try {
        const { group } = await window.wuu.createSessionFolder(name);
        setGroups((current) => ({ ...current, folders: [...current.folders, group] }));
        if (threadID) void moveThread(threadID, "folder", group.id);
      } catch (error) {
        showErrorToast(error, translateCurrent("sessionOrganization.createFolderFailed"));
        throw error;
      }
    },
    createPinGroup: async (name, threadID) => {
      try {
        const { group } = await window.wuu.createPinGroup(name);
        setGroups((current) => ({ ...current, pinGroups: [...current.pinGroups, group] }));
        if (threadID) void moveThread(threadID, "pin", group.id);
      } catch (error) {
        showErrorToast(error, translateCurrent("sessionOrganization.createPinGroupFailed"));
        throw error;
      }
    },
    renameFolder: async (id, name) => {
      try {
        const { group } = await window.wuu.renameSessionFolder(id, name);
        setGroups((current) => ({ ...current, folders: current.folders.map((item) => item.id === id ? group : item) }));
      } catch (error) {
        showErrorToast(error, translateCurrent("sessionOrganization.renameFolderFailed"));
        throw error;
      }
    },
    renamePinGroup: async (id, name) => {
      try {
        const { group } = await window.wuu.renamePinGroup(id, name);
        setGroups((current) => ({ ...current, pinGroups: current.pinGroups.map((item) => item.id === id ? group : item) }));
      } catch (error) {
        showErrorToast(error, translateCurrent("sessionOrganization.renamePinGroupFailed"));
        throw error;
      }
    },
    reorderFolder: async (id, direction) => {
      const reordered = moveOrganizationGroup(groups.folders, id, direction);
      if (reordered === groups.folders) return;
      try {
        const { organization } = await window.wuu.reorderSessionFolders(reordered.map((group) => group.id));
        const parsed = parseSessionOrganization(organization);
        setGroups((current) => ({ ...current, folders: parsed.folders }));
      } catch (error) {
        showErrorToast(error, translateCurrent("sessionOrganization.reorderFolderFailed"));
      }
    },
    reorderPinGroup: async (id, direction) => {
      const reordered = moveOrganizationGroup(groups.pinGroups, id, direction);
      if (reordered === groups.pinGroups) return;
      try {
        const { organization } = await window.wuu.reorderPinGroups(reordered.map((group) => group.id));
        const parsed = parseSessionOrganization(organization);
        setGroups((current) => ({ ...current, pinGroups: parsed.pinGroups.filter((group) => group.id !== "default") }));
      } catch (error) {
        showErrorToast(error, translateCurrent("sessionOrganization.reorderPinGroupFailed"));
      }
    },
    deleteFolder: async (id) => {
      try {
        await window.wuu.deleteSessionFolder(id);
        setGroups((current) => ({ ...current, folders: current.folders.filter((item) => item.id !== id) }));
        setOptimisticFolders((current) => {
          const next = { ...current };
          for (const item of threads) if (item.folder_id === id) next[item.id] = "";
          return next;
        });
      } catch (error) {
        showErrorToast(error, translateCurrent("sessionOrganization.deleteFolderFailed"));
      }
    },
    deletePinGroup: async (id) => {
      try {
        await window.wuu.deletePinGroup(id);
        setGroups((current) => ({ ...current, pinGroups: current.pinGroups.filter((item) => item.id !== id) }));
        setOptimisticPins((current) => {
          const next = { ...current };
          for (const item of threads) if (item.pin_group_id === id) next[item.id] = "default";
          return next;
        });
      } catch (error) {
        showErrorToast(error, translateCurrent("sessionOrganization.deletePinGroupFailed"));
      }
    },
    moveThreadToFolder: (id, folderID) => moveThread(id, "folder", folderID),
    moveThreadToPinGroup: (id, pinGroupID) => moveThread(id, "pin", pinGroupID),
  };
}

export function moveOrganizationGroup(groups: SessionGroup[], id: string, direction: -1 | 1): SessionGroup[] {
  const index = groups.findIndex((group) => group.id === id);
  const target = index + direction;
  if (index < 0 || target < 0 || target >= groups.length) return groups;
  const next = [...groups];
  [next[index], next[target]] = [next[target], next[index]];
  return next;
}

export function clearOptimisticAssignment(
  optimistic: Record<string, string>,
  threadID: string,
  expected: string,
): Record<string, string> {
  if (optimistic[threadID] !== expected) return optimistic;
  const next = { ...optimistic };
  delete next[threadID];
  return next;
}

export function clearReconciledAssignments(
  optimistic: Record<string, string>,
  threads: Map<string, ThreadSummary>,
  field: "folder_id" | "pin_group_id",
): Record<string, string> {
  let next = optimistic;
  for (const [threadID, value] of Object.entries(optimistic)) {
    const thread = threads.get(threadID);
    if (!thread || (thread[field] ?? "") !== value) continue;
    if (next === optimistic) next = { ...optimistic };
    delete next[threadID];
  }
  return next;
}

export type SessionOrganizationActions = SessionOrganization & {
  togglePinned: (thread: ThreadSummary, fallback: (thread: ThreadSummary) => void) => void;
  pinToGroup: (thread: ThreadSummary, pinGroupID?: string) => void;
  moveToFolder: (thread: ThreadSummary, folderID?: string) => void;
  createFolderForThread: (thread: ThreadSummary) => void;
  createPinGroupForThread: (thread: ThreadSummary) => void;
  folderDragThreadID?: string;
  startFolderDrag: (threadID: string) => void;
  endFolderDrag: () => void;
};
const SessionOrganizationContext = createContext<SessionOrganizationActions | null>(null);
export function SessionOrganizationProvider({ value, children }: { value: SessionOrganizationActions; children: ReactNode }): JSX.Element {
  return <SessionOrganizationContext.Provider value={value}>{children}</SessionOrganizationContext.Provider>;
}
export function useSessionOrganizationActions(): SessionOrganizationActions | null {
  return useContext(SessionOrganizationContext);
}
