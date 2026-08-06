import { useCallback, useEffect, useState } from "react";
import type { ManagedProcessSummary } from "../shared/protocol";

const REFRESH_INTERVAL_MS = 1500;
const REFRESH_RETRY_MS = 3000;

/**
 * A process is live while the app-server can still control it. Every settled
 * status is excluded on purpose: there is nothing left to take over, so a
 * terminal opened onto one would have no process behind it.
 */
export function isManagedProcessLive(process: ManagedProcessSummary): boolean {
  return process.status === "starting" || process.status === "running" || process.status === "stopping";
}

/**
 * Reconciles two views of the same process. Polling and push events race, so
 * the older snapshot must not overwrite the newer one, and a live snapshot
 * must not resurrect a process that already settled.
 */
export function preferManagedProcess(
  current: ManagedProcessSummary | undefined,
  next: ManagedProcessSummary,
): ManagedProcessSummary {
  if (!current) {
    return next;
  }
  if (!isManagedProcessLive(current) && isManagedProcessLive(next)) {
    return current;
  }
  return Date.parse(next.updated_at) < Date.parse(current.updated_at) ? current : next;
}

type ProcessMap = Record<string, ManagedProcessSummary>;
type Subscriber = (processes: ProcessMap) => void;

type ThreadStore = {
  processes: ProcessMap;
  subscribers: Set<Subscriber>;
  refs: number;
  timer?: number;
  disposed: boolean;
};

// One store per thread, shared by every surface watching it. The terminal
// workspace and the environment panel are visible at the same time, and two
// independent pollers would both cost an extra request every interval and let
// the two lists disagree about what is still running.
const threadStores = new Map<string, ThreadStore>();

function emit(store: ThreadStore): void {
  for (const subscriber of store.subscribers) {
    subscriber(store.processes);
  }
}

function scheduleRefresh(threadID: string, store: ThreadStore, delay: number): void {
  store.timer = window.setTimeout(() => void refreshStore(threadID, store), delay);
}

async function refreshStore(threadID: string, store: ThreadStore): Promise<void> {
  try {
    const result = await window.wuu.listManagedProcesses(threadID);
    if (store.disposed) {
      return;
    }
    const incoming = result.processes.filter(isManagedProcessLive);
    store.processes = Object.fromEntries(incoming.map((process) => [
      process.id,
      preferManagedProcess(store.processes[process.id], process),
    ]));
    emit(store);
    scheduleRefresh(threadID, store, REFRESH_INTERVAL_MS);
  } catch {
    if (!store.disposed) {
      scheduleRefresh(threadID, store, REFRESH_RETRY_MS);
    }
  }
}

function acquireStore(threadID: string): ThreadStore {
  const existing = threadStores.get(threadID);
  if (existing) {
    existing.refs += 1;
    return existing;
  }
  const store: ThreadStore = { processes: {}, subscribers: new Set(), refs: 1, disposed: false };
  threadStores.set(threadID, store);
  void refreshStore(threadID, store);
  return store;
}

function releaseStore(threadID: string, store: ThreadStore): void {
  store.refs -= 1;
  if (store.refs > 0) {
    return;
  }
  store.disposed = true;
  if (store.timer !== undefined) {
    window.clearTimeout(store.timer);
  }
  threadStores.delete(threadID);
}

/** Test-only: drops every shared store so cases cannot leak polling into each other. */
export function resetLiveManagedProcessStores(): void {
  for (const [threadID, store] of threadStores) {
    store.refs = 1;
    releaseStore(threadID, store);
  }
  threadStores.clear();
}

/**
 * Tracks the background commands a thread can still control.
 *
 * Every caller watching the same thread shares one subscription, so the
 * surfaces cannot disagree and the endpoint is polled once regardless of how
 * many of them are on screen.
 */
export function useLiveManagedProcesses(threadID?: string): {
  processes: ProcessMap;
  applyProcessChange: (next: ManagedProcessSummary) => void;
} {
  const [processes, setProcesses] = useState<ProcessMap>({});

  useEffect(() => {
    if (!threadID) {
      setProcesses({});
      return undefined;
    }
    const store = acquireStore(threadID);
    const subscriber: Subscriber = (next) => setProcesses(next);
    store.subscribers.add(subscriber);
    setProcesses(store.processes);
    return () => {
      store.subscribers.delete(subscriber);
      releaseStore(threadID, store);
    };
  }, [threadID]);

  const applyProcessChange = useCallback((next: ManagedProcessSummary) => {
    if (!threadID) {
      return;
    }
    const store = threadStores.get(threadID);
    if (!store) {
      return;
    }
    if (!isManagedProcessLive(next)) {
      const { [next.id]: _removed, ...rest } = store.processes;
      store.processes = rest;
    } else {
      store.processes = {
        ...store.processes,
        [next.id]: preferManagedProcess(store.processes[next.id], next),
      };
    }
    emit(store);
  }, [threadID]);

  return { processes, applyProcessChange };
}

/** Live processes newest-first, which is the order both surfaces present. */
export function liveManagedProcessList(processes: ProcessMap): ManagedProcessSummary[] {
  return Object.values(processes)
    .filter(isManagedProcessLive)
    .sort((left, right) => Date.parse(right.updated_at) - Date.parse(left.updated_at));
}

/**
 * The terminal workspace addresses a live process by this resource id, so the
 * environment panel can hand one straight to it without inventing a second
 * addressing scheme.
 */
export function managedProcessResourceID(processID: string): string {
  return `managed:${processID}`;
}
