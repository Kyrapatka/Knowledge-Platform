import type { ImportDraft } from "./types.js";
export type SavedDraft = {
  key: string;
  updatedAt: number;
  input: string;
  kind: "text" | "json";
  draft: ImportDraft | null;
};
const AGE = 7 * 86400000,
  MAX = 80 * 1024 * 1024;
let connection: Promise<IDBDatabase> | undefined;
function db() {
  return (connection ??= new Promise<IDBDatabase>((resolve, reject) => {
    const r = indexedDB.open("knowledge-quick-import", 1);
    r.onupgradeneeded = () =>
      r.result.createObjectStore("drafts", { keyPath: "key" });
    r.onsuccess = () => resolve(r.result);
    r.onerror = () => {
      connection = undefined;
      reject(r.error);
    };
  }));
}
export const storageKey = (user: string, folder: string) =>
  JSON.stringify([user, folder]);
export async function loadDraft(key: string): Promise<SavedDraft | null> {
  const database = await db();
  return new Promise((resolve, reject) => {
    const request = database
      .transaction("drafts")
      .objectStore("drafts")
      .get(key);
    request.onsuccess = () => {
      const s = request.result as SavedDraft | undefined;
      resolve(
        s &&
          s.updatedAt > Date.now() - AGE &&
          (!s.draft || s.draft.version === 1)
          ? s
          : null,
      );
    };
    request.onerror = () => reject(request.error);
  });
}
export async function saveDraft(saved: SavedDraft): Promise<void> {
  const encoded = JSON.stringify(saved);
  if (new TextEncoder().encode(encoded).length > 40 * 1024 * 1024)
    throw new Error("Draft exceeds the 40 MiB local recovery limit");
  const database = await db();
  return new Promise((resolve, reject) => {
    const tx = database.transaction("drafts", "readwrite"),
      store = tx.objectStore("drafts");
    const req = store.getAll();
    req.onsuccess = () => {
      const existing = req.result as SavedDraft[];
      if (
        existing.some(
          (s) => s.key === saved.key && s.updatedAt > saved.updatedAt,
        )
      )
        return;
      const candidates = [
        saved,
        ...existing
          .filter((s) => s.key !== saved.key && s.updatedAt > Date.now() - AGE)
          .sort((a, b) => b.updatedAt - a.updatedAt),
      ];
      let bytes = 0;
      const keep = new Set<string>();
      for (const s of candidates) {
        const size = new TextEncoder().encode(JSON.stringify(s)).length;
        if (keep.size < 5 && bytes + size <= MAX) {
          keep.add(s.key);
          bytes += size;
        }
      }
      for (const s of existing) if (!keep.has(s.key)) store.delete(s.key);
      store.put(saved);
    };
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () =>
      reject(tx.error ?? new Error("Local draft could not be saved"));
  });
}
export async function deleteDraft(key: string) {
  const database = await db();
  return new Promise<void>((resolve, reject) => {
    const tx = database.transaction("drafts", "readwrite");
    tx.objectStore("drafts").delete(key);
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
  });
}
