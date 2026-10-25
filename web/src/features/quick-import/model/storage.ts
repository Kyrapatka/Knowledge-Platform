import type { ImportDraft } from "./types.js";

export type SavedDraft = {
  key: string;
  updatedAt: number;
  input?: string;
  kind: "text" | "json";
  draft: ImportDraft | null;
  /** Missing on version-one records written before optimistic concurrency. */
  revision?: number;
};
const AGE = 7 * 86400000;
const MAX = 80 * 1024 * 1024;
const RECORD_MAX = 40 * 1024 * 1024;
const COUNT = 5;
// This cannot collide with storageKey(), which always returns a JSON array.
// Keeping the clock after deletion also prevents an old tab from matching the
// revision of a later draft recreated under the same user/folder key.
const CLOCK_KEY = "quick-import-revision-clock";
type RevisionClock = { key: typeof CLOCK_KEY; revisionCounter: number };
let connection: Promise<IDBDatabase> | undefined;

export class DraftConflictError extends Error {
  constructor() {
    super(
      "This draft changed in another tab. Reopen Quick import to load the latest saved draft.",
    );
    this.name = "DraftConflictError";
  }
}
export class PendingCommandError extends Error {
  constructor() {
    super(
      "An unconfirmed import is saved. Retry that command before replacing or deleting this draft.",
    );
    this.name = "PendingCommandError";
  }
}

function db() {
  return (connection ??= new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open("knowledge-quick-import", 1);
    request.onupgradeneeded = () =>
      request.result.createObjectStore("drafts", { keyPath: "key" });
    request.onsuccess = () => {
      const database = request.result;
      database.onversionchange = () => {
        database.close();
        connection = undefined;
      };
      resolve(database);
    };
    request.onerror = () => {
      connection = undefined;
      reject(request.error);
    };
    request.onblocked = () => {
      connection = undefined;
      reject(
        new Error(
          "Local draft storage is busy in another tab. Close that tab and retry.",
        ),
      );
    };
  }));
}
export const storageKey = (user: string, folder: string) =>
  JSON.stringify([user, folder]);

export function savedInput(saved: SavedDraft): string {
  return typeof saved.input === "string"
    ? saved.input
    : (saved.draft?.source ?? "");
}
function retained(saved: SavedDraft, now: number): boolean {
  return !!saved.draft?.pending || saved.updatedAt > now - AGE;
}
function encodedSize(saved: SavedDraft): number {
  return new TextEncoder().encode(JSON.stringify(saved)).length;
}

export async function loadDraft(key: string): Promise<SavedDraft | null> {
  const database = await db();
  return new Promise((resolve, reject) => {
    const transaction = database.transaction("drafts");
    const request = transaction.objectStore("drafts").get(key);
    let saved: SavedDraft | null = null;
    request.onsuccess = () => {
      const result = request.result as SavedDraft | undefined;
      if (
        result &&
        retained(result, Date.now()) &&
        (!result.draft || result.draft.version === 1)
      )
        saved = result;
    };
    transaction.oncomplete = () => resolve(saved);
    transaction.onerror = () => reject(transaction.error);
    transaction.onabort = () =>
      reject(transaction.error ?? new Error("Local draft could not be loaded"));
  });
}

/**
 * Save only if the last observed revision still owns this key. null means the
 * caller observed no recoverable record; zero denotes an existing legacy row.
 * The caller must serialize its own writes and retain the returned revision.
 * confirmedCommandId is only for a definitive server response, never a timeout.
 */
export async function saveDraft(
  saved: SavedDraft,
  expectedRevision: number | null = saved.revision ?? null,
  confirmedCommandId?: string,
): Promise<number> {
  if (encodedSize(saved) > RECORD_MAX)
    throw new Error("Draft exceeds the 40 MiB local recovery limit");
  const database = await db();
  return new Promise((resolve, reject) => {
    const tx = database.transaction("drafts", "readwrite");
    const store = tx.objectStore("drafts");
    const request = store.getAll();
    let failure: unknown;
    let revision = 0;
    request.onsuccess = () => {
      try {
        const now = Date.now();
        const records = request.result as (SavedDraft | RevisionClock)[];
        const clock = records.find(
          (row): row is RevisionClock => row.key === CLOCK_KEY,
        );
        const existing = records.filter(
          (row): row is SavedDraft => row.key !== CLOCK_KEY,
        );
        const current = existing.find(
          (row) => row.key === saved.key && retained(row, now),
        );
        if ((current ? (current.revision ?? 0) : null) !== expectedRevision)
          throw new DraftConflictError();
        const pending = current?.draft?.pending;
        if (pending) {
          const nextPending = saved.draft?.pending;
          // A confirmed rejection may clear the command, but it cannot authorize
          // changing the payload or installing another command under its ID.
          if (nextPending) {
            if (JSON.stringify(nextPending) !== JSON.stringify(pending))
              throw new PendingCommandError();
          } else if (confirmedCommandId !== pending.command_id) {
            throw new PendingCommandError();
          }
        }
        revision =
          Math.max(
            clock?.revisionCounter ?? 0,
            ...existing.map((row) => row.revision ?? 0),
          ) + 1;
        const next: SavedDraft = { ...saved, revision };
        const pinned = existing.filter(
          (row) => row.key !== saved.key && row.draft?.pending,
        );
        const candidates = existing
          .filter(
            (row) =>
              row.key !== saved.key &&
              !row.draft?.pending &&
              retained(row, now),
          )
          .sort((a, b) => b.updatedAt - a.updatedAt);
        const keep = new Set(pinned.map((row) => row.key));
        let bytes =
          pinned.reduce((sum, row) => sum + encodedSize(row), 0) +
          encodedSize(next);
        // Never evict unresolved commands to make a new record fit. Failure is
        // reported before a caller sends any command that cannot be recovered.
        if (bytes > MAX || (keep.size >= COUNT && !current?.draft?.pending))
          throw new Error(
            "Local recovery storage is full of unconfirmed imports. Resolve an existing import before starting another.",
          );
        keep.add(next.key);
        for (const candidate of candidates) {
          const size = encodedSize(candidate);
          if (keep.size < COUNT && bytes + size <= MAX) {
            keep.add(candidate.key);
            bytes += size;
          }
        }
        for (const row of existing)
          if (!keep.has(row.key)) store.delete(row.key);
        store.put(next);
        store.put({
          key: CLOCK_KEY,
          revisionCounter: revision,
        } satisfies RevisionClock);
      } catch (error) {
        failure = error;
        tx.abort();
      }
    };
    tx.oncomplete = () => resolve(revision);
    tx.onerror = () =>
      reject(
        failure ?? tx.error ?? new Error("Local draft could not be saved"),
      );
    tx.onabort = () =>
      reject(
        failure ?? tx.error ?? new Error("Local draft could not be saved"),
      );
  });
}

/** Confirmed success may delete only its own pending command. */
export async function deleteDraft(
  key: string,
  expectedRevision?: number | null,
  confirmedCommandId?: string,
): Promise<void> {
  const database = await db();
  return new Promise((resolve, reject) => {
    const tx = database.transaction("drafts", "readwrite");
    const store = tx.objectStore("drafts");
    const request = store.get(key);
    let failure: unknown;
    request.onsuccess = () => {
      try {
        const current = request.result as SavedDraft | undefined;
        if (
          expectedRevision !== undefined &&
          (current ? (current.revision ?? 0) : null) !== expectedRevision
        )
          throw new DraftConflictError();
        if (
          current?.draft?.pending &&
          current.draft.pending.command_id !== confirmedCommandId
        )
          throw new PendingCommandError();
        store.delete(key);
      } catch (error) {
        failure = error;
        tx.abort();
      }
    };
    tx.oncomplete = () => resolve();
    tx.onerror = () =>
      reject(
        failure ?? tx.error ?? new Error("Local draft could not be deleted"),
      );
    tx.onabort = () =>
      reject(
        failure ?? tx.error ?? new Error("Local draft could not be deleted"),
      );
  });
}
