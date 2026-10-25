import type {
  ImportDraft,
  ImportFolder,
  ImportCommand,
  ImportResult,
  Range,
} from "./types.js";
export type Diagnostic = {
  id: string;
  cardId?: string;
  field?: string;
  path?: string;
  severity: "error" | "warning" | "info";
  blocking: boolean;
  code: string;
  message: string;
  why: string;
  action: string;
  sources: string[];
  range?: Range;
  issueIds: string[];
};
export type Preview = { result: ImportResult; command: ImportCommand };
export function diagnosticsFor(
  draft: ImportDraft,
  folder: ImportFolder,
  preview?: Preview,
): Diagnostic[] {
  const out: Diagnostic[] = [];
  const cards = new Map(draft.cards.map((c) => [c.id, c]));
  const add = (d: Diagnostic) => {
    if (d.cardId && cards.get(d.cardId)?.excluded) return;
    const same = out.find(
      (x) =>
        x.cardId === d.cardId &&
        x.field === d.field &&
        (x.code === d.code ||
          (!!d.field &&
            /required/i.test(x.message) &&
            /required/i.test(d.message))) &&
        (d.field || x.message === d.message),
    );
    if (same) {
      same.sources = [...new Set([...same.sources, ...d.sources])];
      same.issueIds.push(...d.issueIds);
      same.blocking ||= d.blocking;
      return;
    }
    out.push(d);
  };
  for (const c of draft.cards) {
    for (const [values, fields, prefix] of [
      [c.values, folder.config.schema.fields, "values"],
      [c.metadata, folder.config.metadata_schema.fields, "metadata"],
    ] as const) {
      for (const f of fields) {
        if (
          f.active &&
          f.required &&
          !values[f.key]?.trim() &&
          !(f.key === "example" && c.examples.some((e) => e.text.trim()))
        )
          add({
            id: `${c.id}:${prefix}.${f.key}:required`,
            cardId: c.id,
            field: `${prefix}.${f.key}`,
            severity: "error",
            blocking: true,
            code: "required",
            message: `${f.label ?? f.key} is required`,
            why: "This folder requires a value.",
            action: "Fill in this field.",
            sources: ["local"],
            issueIds: [],
            range: c.ranges[f.key] ?? c.range,
          });
      }
    }
    for (const message of c.validation) {
      if (/ is required$/.test(message)) continue;
      const key = message.match(/(?:Unsupported field: |Field )(\w+)/)?.[1];
      add({
        id: `${c.id}:local:${message}`,
        cardId: c.id,
        field: key
          ? `${key in c.metadata ? "metadata" : "values"}.${key}`
          : undefined,
        severity: "error",
        blocking: true,
        code: "local_validation",
        message,
        why: "This value does not match the folder configuration.",
        action: "Edit the field or exclude this card.",
        sources: ["local"],
        issueIds: [],
        range: c.range,
      });
    }
    if (c.duplicateOf) {
      const conflict = c.duplicatePolicy === "conflict";
      add({
        id: `${c.id}:duplicate`,
        cardId: c.id,
        severity: conflict ? "error" : "info",
        blocking: conflict,
        code: "duplicate",
        message: conflict
          ? "Repeated material would stop this import."
          : c.duplicatePolicy === "allow"
            ? "An additional copy will be created."
            : "Repeated material will be skipped.",
        why: "The same word or question appears earlier in this import.",
        action: "Choose how to handle this repeat.",
        sources: ["local"],
        issueIds: [],
        range: c.range,
      });
    }
  }
  for (const i of draft.issues) {
    if (i.resolved || i.code === "duplicate_word") continue;
    const unsupported = i.code === "unsupported_fragment";
    // Required-field diagnostics are recomputed from current values, not parser snapshots.
    if (
      i.cardId &&
      /required|missing_(?:word|translation|question)/.test(i.code)
    )
      continue;
    add({
      id: i.id,
      cardId: i.cardId,
      field: i.path?.match(/(?:values|metadata|profile)\..+$/)?.[0],
      path: i.path,
      severity: i.severity,
      blocking: i.blocking,
      code: i.code,
      message: unsupported ? "Unrecognized fragment" : i.message,
      why: unsupported
        ? "This may be a pronunciation hint. Pronunciation is not imported."
        : i.message,
      action: unsupported
        ? "Remove the fragment, assign it to a field, or edit the source."
        : "Review the fragment and choose an interpretation, or edit the source.",
      sources: ["parser"],
      issueIds: [i.id],
      range: i.range,
    });
  }
  if (preview) {
    const { result, command } = preview;
    for (const i of result.issues) {
      const index = i.path.match(/^items\[(\d+)\]/)?.[1];
      const cardId =
        i.client_id ||
        (index !== undefined
          ? command.items[Number(index)]?.client_id
          : undefined);
      const item = result.items.find((x) => x.client_id === cardId);
      const skip =
        item?.status === "skipped" &&
        /duplicate/i.test(i.code + " " + i.message);
      const field = i.path.match(/(?:values|metadata|profile)\.[\w]+/)?.[0];
      add({
        id: `server:${cardId ?? "global"}:${field ?? i.path}:${i.code}`,
        cardId,
        field,
        path: i.path,
        severity: skip
          ? "info"
          : i.severity === "warning"
            ? "warning"
            : "error",
        blocking: !skip,
        code: i.code,
        message: skip
          ? "Already in this folder; this repeat will be skipped."
          : i.message,
        why: skip
          ? "Your duplicate policy is Skip."
          : "The server could not accept this value.",
        action: skip
          ? "Change the repeat policy only if another copy is intended."
          : field
            ? "Correct this field and try importing again."
            : "Review this card or source and try again.",
        sources: ["server"],
        issueIds: [],
        range: cardId ? cards.get(cardId)?.range : undefined,
      });
    }
    for (const item of result.items.filter(
      (i) => i.duplicate_kind || i.status === "skipped",
    )) {
      if (
        out.some(
          (d) =>
            d.cardId === item.client_id &&
            /duplicate|repeat/i.test(d.code + " " + d.message),
        )
      )
        continue;
      const skipped = item.status === "skipped";
      add({
        id: `${item.client_id}:server-duplicate`,
        cardId: item.client_id,
        severity: "info",
        blocking: false,
        code: "duplicate",
        message: skipped
          ? "Already in this folder; this repeat will be skipped."
          : "Another copy will be created as requested.",
        why: "The server found matching material.",
        action: "Choose how to handle this repeat.",
        sources: ["server"],
        issueIds: [],
      });
    }
  }
  return out;
}
export function summarize(draft: ImportDraft, diagnostics: Diagnostic[]) {
  const active = diagnostics.filter((d) => d.severity !== "info");
  const issueIds = new Set(active.map((d) => d.cardId));
  const blocked = new Set(
    active
      .filter((d) => d.blocking || d.severity === "error")
      .map((d) => d.cardId),
  );
  return {
    readyCards: draft.cards.filter((c) => !c.excluded && !blocked.has(c.id)),
    issueCards: draft.cards.filter((c) => !c.excluded && issueIds.has(c.id)),
    globalIssues: active.filter((d) => !d.cardId),
    active,
  };
}
export function filteredCards(
  draft: ImportDraft,
  diagnostics: Diagnostic[],
  filter: string,
) {
  const s = summarize(draft, diagnostics);
  return filter === "issues"
    ? s.issueCards
    : filter === "ready"
      ? s.readyCards
      : draft.cards;
}
export const isCurrentPreview = (submitted: number, current: number) =>
  submitted === current;
