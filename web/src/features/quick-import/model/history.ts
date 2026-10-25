import type { ImportDraft } from "./types.js";
type Snapshot = Pick<ImportDraft, "cards" | "issues">;
export type EditHistory = {
  past: Snapshot[];
  future: Snapshot[];
  group?: string;
  at: number;
};
export const createHistory = (): EditHistory => ({
  past: [],
  future: [],
  at: 0,
});
const snapshot = (d: ImportDraft): Snapshot =>
  structuredClone({ cards: d.cards, issues: d.issues });
export function recordEdit(
  h: EditHistory,
  before: ImportDraft,
  after: ImportDraft,
  options: { group?: string; now?: number } = {},
): EditHistory {
  if (
    JSON.stringify([before.cards, before.issues]) ===
    JSON.stringify([after.cards, after.issues])
  )
    return h;
  const now = options.now ?? Date.now();
  const grouped =
    !!options.group && h.group === options.group && now - h.at < 900;
  return {
    past: grouped ? h.past : [...h.past, snapshot(before)].slice(-5),
    future: [],
    group: options.group,
    at: now,
  };
}
export function undoEdit(h: EditHistory, d: ImportDraft) {
  if (d.pending || !h.past.length) return { history: h, draft: d };
  return {
    history: {
      past: h.past.slice(0, -1),
      future: [...h.future, snapshot(d)],
      at: 0,
    },
    draft: {
      ...d,
      ...structuredClone(h.past[h.past.length - 1]),
      updatedAt: Date.now(),
    },
  };
}
export function redoEdit(h: EditHistory, d: ImportDraft) {
  if (d.pending || !h.future.length) return { history: h, draft: d };
  return {
    history: {
      past: [...h.past, snapshot(d)].slice(-5),
      future: h.future.slice(0, -1),
      at: 0,
    },
    draft: {
      ...d,
      ...structuredClone(h.future[h.future.length - 1]),
      updatedAt: Date.now(),
    },
  };
}
