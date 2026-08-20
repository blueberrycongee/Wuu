import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import type { SessionOrganizationGroup } from "../shared/protocol";
import type { ThreadSummary } from "./AppState";

export type SessionGroup = Pick<SessionOrganizationGroup, "id" | "name">;
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
  createFolder: (name: string, threadID?: string) => void;
  createPinGroup: (name: string, threadID?: string) => void;
  renameFolder: (id: string, name: string) => void;
  renamePinGroup: (id: string, name: string) => void;
  deleteFolder: (id: string) => void;
  deletePinGroup: (id: string) => void;
  moveThreadToFolder: (threadID: string, folderID?: string) => void;
  moveThreadToPinGroup: (threadID: string, pinGroupID?: string) => void;
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
    }).catch(() => undefined);
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

  function moveThread(threadID: string, axis: "folder" | "pin", value?: string): void {
    const assignment = value ?? "";
    const setOptimistic = axis === "folder" ? setOptimisticFolders : setOptimisticPins;
    setOptimistic((current) => ({ ...current, [threadID]: assignment }));
    const request = axis === "folder"
      ? window.wuu.updateThreadOrganization(threadID, assignment, undefined)
      : window.wuu.updateThreadOrganization(threadID, undefined, assignment);
    void request.then(
      () => setOptimistic((current) => clearOptimisticAssignment(current, threadID, assignment)),
      () => setOptimistic((current) => clearOptimisticAssignment(current, threadID, assignment)),
    );
  }

  return {
    ...groups,
    ...assignments,
    createFolder: (name, threadID) => { void window.wuu.createSessionFolder(name).then(({ group }) => {
      setGroups((current) => ({ ...current, folders: [...current.folders, group] }));
      if (threadID) moveThread(threadID, "folder", group.id);
    }); },
    createPinGroup: (name, threadID) => { void window.wuu.createPinGroup(name).then(({ group }) => {
      setGroups((current) => ({ ...current, pinGroups: [...current.pinGroups, group] }));
      if (threadID) moveThread(threadID, "pin", group.id);
    }); },
    renameFolder: (id, name) => { void window.wuu.renameSessionFolder(id, name).then(({ group }) => {
      setGroups((current) => ({ ...current, folders: current.folders.map((item) => item.id === id ? group : item) }));
    }); },
    renamePinGroup: (id, name) => { void window.wuu.renamePinGroup(id, name).then(({ group }) => {
      setGroups((current) => ({ ...current, pinGroups: current.pinGroups.map((item) => item.id === id ? group : item) }));
    }); },
    deleteFolder: (id) => { void window.wuu.deleteSessionFolder(id).then(() => {
      setGroups((current) => ({ ...current, folders: current.folders.filter((item) => item.id !== id) }));
      setOptimisticFolders((current) => {
        const next = { ...current };
        for (const item of threads) if (item.folder_id === id) next[item.id] = "";
        return next;
      });
    }); },
    deletePinGroup: (id) => { void window.wuu.deletePinGroup(id).then(() => {
      setGroups((current) => ({ ...current, pinGroups: current.pinGroups.filter((item) => item.id !== id) }));
      setOptimisticPins((current) => {
        const next = { ...current };
        for (const item of threads) if (item.pin_group_id === id) next[item.id] = "default";
        return next;
      });
    }); },
    moveThreadToFolder: (id, folderID) => moveThread(id, "folder", folderID),
    moveThreadToPinGroup: (id, pinGroupID) => moveThread(id, "pin", pinGroupID),
  };
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
};
const SessionOrganizationContext = createContext<SessionOrganizationActions | null>(null);
export function SessionOrganizationProvider({ value, children }: { value: SessionOrganizationActions; children: ReactNode }): JSX.Element {
  return <SessionOrganizationContext.Provider value={value}>{children}</SessionOrganizationContext.Provider>;
}
export function useSessionOrganizationActions(): SessionOrganizationActions | null {
  return useContext(SessionOrganizationContext);
}
