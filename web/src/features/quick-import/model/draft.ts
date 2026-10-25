import { createClientUUID } from "../../../uuid.js";
import { parseText } from "../parser/reference.js";
import {
  rows,
  fragmentRange,
  hash,
  normalized,
  phrase,
} from "../parser/source.js";
import { strictJSON, JSONIssue } from "../parser/strict-json.js";
import type {
  ImportDraft,
  ImportFolder,
  DraftCard,
  Issue,
  Range,
  ImportCommand,
  ProfileInput,
} from "./types.js";
export type {
  ImportDraft,
  ImportFolder,
  DraftCard,
  Issue,
  Range,
} from "./types.js";
const origin: Range = { start: 0, end: 0, line: 1 };
const aliases: Record<string, string> = {
  word: "foreign",
  term: "foreign",
  слово: "foreign",
  translation: "native",
  перевод: "native",
  meaning: "native",
  full_answer: "answer",
  source: "sources",
  notes: "note",
  example: "example",
  пример: "example",
  question: "question",
  вопрос: "question",
  answer: "answer",
  ответ: "short_answer",
  "short answer": "short_answer",
  заметка: "note",
};
export function parseDraft(
  source: string,
  kind: "text" | "json",
  folder: ImportFolder,
): ImportDraft {
  const draft: ImportDraft = {
    version: 1,
    draftId: createClientUUID(),
    folderId: folder.id,
    kind,
    source,
    cards: [],
    issues: [],
    automatic: { cards: [], issues: [] },
    history: [],
    updatedAt: Date.now(),
  };
  const counts = new Map<string, number>();
  const card = (values: Record<string, string>, range: Range): DraftCard => {
    const key = hash(JSON.stringify(values));
    const occurrence = (counts.get(key) ?? 0) + 1;
    counts.set(key, occurrence);
    const c: DraftCard = {
      id: `c-${key}-${occurrence}`,
      values,
      metadata: {},
      difficulty: "medium",
      examples: [],
      ranges: {},
      range,
      excluded: false,
      status: "ready",
      validation: [],
    };
    draft.cards.push(c);
    return c;
  };
  const issue = (
    code: string,
    fragment: string,
    range: Range,
    message: string,
    c?: DraftCard,
    path?: string,
    severity: Issue["severity"] = "warning",
    blocking = true,
  ) => {
    const i: Issue = {
      id: `i-${hash(code + range.start + fragment)}-${draft.issues.length}`,
      code,
      fragment,
      range,
      message,
      cardId: c?.id,
      path,
      severity,
      blocking,
      actions: ["Assign to field", "Exclude fragment"],
    };
    draft.issues.push(i);
    return i;
  };
  const activeValues = new Set(
    folder.config.schema.fields
      .filter((f) => f.active && f.key !== "transcription")
      .map((f) => f.key),
  );
  const activeMeta = new Set(
    folder.config.metadata_schema.fields
      .filter((f) => f.active)
      .map((f) => f.key),
  );
  const setField = (
    c: DraftCard,
    key: string,
    text: string,
    range: Range,
    path?: string,
  ) => {
    const target = activeValues.has(key)
      ? c.values
      : activeMeta.has(key)
        ? c.metadata
        : null;
    if (!target) {
      issue(
        "unsupported_field",
        source.slice(range.start, range.end),
        range,
        `Field ${key} is not supported by this folder`,
        c,
        path,
      );
      return;
    }
    if (key === "example") {
      c.examples.push({
        id: `e-${range.start}-${c.examples.length}`,
        text,
        range,
      });
    } else {
      target[key] = target[key] ? target[key] + "\n" + text : text;
    }
    c.ranges[key] = range;
  };
  const bytes = new TextEncoder().encode(source).length;
  if (bytes > (kind === "text" ? 1 << 20 : 10 << 20))
    issue(
      "source_limit",
      "",
      origin,
      "Source exceeds import size limit",
      undefined,
      undefined,
      "error",
    );
  else if (
    folder.template_key !== "english_words" &&
    folder.template_key !== "interview_questions"
  )
    issue(
      "template",
      "",
      origin,
      "This folder does not support Quick Import",
      undefined,
      undefined,
      "error",
    );
  else if (kind === "text") {
    const lines = rows(source);
    const covered = new Set<number>();
    if (folder.template_key === "english_words") {
      const extra = lines.filter((r) => {
        const m = r.text.match(/^\s*([^:]+):\s*(.*)$/);
        if (!m) return false;
        const key = aliases[m[1].trim().toLowerCase()] ?? m[1].trim();
        return (
          (activeValues.has(key) || activeMeta.has(key)) &&
          !["foreign", "native", "example"].includes(key)
        );
      });
      const extraLines = new Set(extra.map((r) => r.range.line));
      const parsed = parseText(
        lines
          .map((r) => (extraLines.has(r.range.line) ? "" : r.text))
          .join("\n"),
      );
      const idMap = new Map<string, string>();
      for (const raw of parsed.cards) {
        const start = lines[raw.startLine - 1];
        if (!start) continue;
        const c = card(
          { foreign: raw.word, native: raw.translation },
          start.range,
        );
        idMap.set(raw.id, c.id);
        const translationRow =
          raw.startLine === raw.endLine
            ? start
            : (lines.find(
                (r) =>
                  r.range.line >= raw.startLine &&
                  r.text.includes(raw.translation),
              ) ?? start);
        c.ranges.foreign = fragmentRange(start, raw.word);
        c.ranges.native = fragmentRange(translationRow, raw.translation);
        covered.add(start.range.line);
        covered.add(translationRow.range.line);
        raw.examples.forEach((text, index) => {
          const line = raw.exampleLines?.[index];
          const row = line ? lines[line - 1] : undefined;
          if (row) {
            text = row.text
              .replace(/^\s*(?:example|пример)\s*:\s*/i, "")
              .trim();
            c.examples.push({
              id: `e-${row.range.start}`,
              text,
              range: fragmentRange(row, text),
            });
            covered.add(row.range.line);
            if (
              !phrase(raw.word, text) &&
              !/^\s*(example|пример)\s*:/i.test(row.text)
            )
              issue(
                "weak_example",
                text,
                fragmentRange(row, text),
                "Attached by sentence structure; confirm this example",
                c,
              );
          }
        });
        if (translationRow !== start) {
          const candidates = draft.cards
            .filter(
              (other) => other !== c && phrase(other.values.foreign, raw.word),
            )
            .slice(-4);
          if (candidates.length) {
            const i = issue(
              "boundary_alternative",
              source.slice(start.range.start, translationRow.range.end),
              { ...start.range, end: translationRow.range.end },
              "This can be a new card or an example of an earlier card. Keep the card or assign the fragment.",
              c,
            );
            i.alternatives = [
              {
                kind: "boundary",
                cardId: c.id,
                score: 60,
                reasons: ["adjacent English/Russian lines"],
              },
              ...candidates.map((other) => ({
                kind: "example" as const,
                cardId: other.id,
                score: 65,
                reasons: ["exact phrase in earlier card"],
              })),
            ];
          }
        }
      }
      for (const raw of parsed.issues) {
        const row = lines[raw.line - 1];
        if (!row) continue;
        covered.add(raw.line);
        const owner = parsed.cards.find((c) => c.issues.includes(raw));
        const c = draft.cards.find((c) => c.id === idMap.get(owner?.id ?? ""));
        const i = issue(
          raw.code,
          raw.fragment,
          fragmentRange(row, raw.fragment),
          raw.message,
          c,
          undefined,
          raw.severity,
          raw.code !== "duplicate_word",
        );
        const review = parsed.reviews.find((r) => r.line === raw.line);
        if (review)
          i.alternatives = review.alternatives.map((a) => ({
            kind: "example",
            cardId: idMap.get(a.cardId),
            score: a.score,
            reasons: a.reasons,
          }));
      }
      for (const row of extra) {
        const m = row.text.match(/^\s*([^:]+):\s*(.*)$/)!;
        const key = aliases[m[1].trim().toLowerCase()] ?? m[1].trim();
        const c = draft.cards
          .filter((c) => c.range.line < row.range.line)
          .at(-1);
        if (c) {
          setField(c, key, m[2], fragmentRange(row, m[2]));
          covered.add(row.range.line);
        }
      }
    } else {
      let current: DraftCard | undefined;
      let field: string | undefined;
      for (const row of lines) {
        const text = row.text.trim();
        if (!text) {
          field = undefined;
          continue;
        }
        const label = text.match(/^(?:\d+[.)]\s*)?([^:]+):\s*(.*)$/);
        const key = label
          ? (aliases[label[1].trim().toLowerCase()] ??
            label[1].trim().toLowerCase())
          : undefined;
        if (
          key === "question" ||
          (!label &&
            (/^\d+[.)]\s+/.test(text) || (text.endsWith("?") && !field)))
        ) {
          const value = label ? label[2] : text.replace(/^\d+[.)]\s+/, "");
          current = card({}, row.range);
          setField(current, "question", value, fragmentRange(row, value));
          field = "question";
          covered.add(row.range.line);
        } else if (label && current) {
          setField(current, key!, label[2], fragmentRange(row, label[2]));
          field =
            activeValues.has(key!) || activeMeta.has(key!) ? key : undefined;
          covered.add(row.range.line);
        } else if (current && field) {
          setField(current, field, text, fragmentRange(row, text));
          covered.add(row.range.line);
        }
      }
    }
    // Coverage is checked independently of the parser's own issue list.
    for (const row of lines)
      if (row.text.trim() && !covered.has(row.range.line))
        issue(
          "unassigned",
          row.text,
          row.range,
          "Unassigned source text: assign or explicitly exclude",
        );
  } else {
    try {
      const parsed = strictJSON(source);
      const object = (v: unknown): v is Record<string, unknown> =>
        !!v && typeof v === "object" && !Array.isArray(v);
      const envelope = object(parsed.value) ? parsed.value : undefined;
      const items = envelope?.items ?? parsed.value;
      const prefix = envelope ? "items" : "";
      const range = (path: string) => parsed.ranges.get(path) ?? origin;
      if (envelope) {
        for (const key of Object.keys(envelope))
          if (!["format_version", "folder", "items"].includes(key))
            issue(
              "unknown_envelope_field",
              source.slice(range(key).start, range(key).end),
              range(key),
              `Unknown envelope field ${key}`,
              undefined,
              key,
              "error",
            );
        if (envelope.format_version !== 1)
          issue(
            "format_version",
            "",
            range("format_version"),
            "format_version must be 1",
            undefined,
            "format_version",
            "error",
          );
        const f = envelope.folder;
        if (!object(f) || f.template !== folder.template_key)
          issue(
            "template_mismatch",
            "",
            range("folder"),
            "Envelope folder template must match the destination",
            undefined,
            "folder.template",
            "error",
          );
        if (object(f))
          for (const key of Object.keys(f))
            if (!["name", "description", "template"].includes(key))
              issue(
                "unknown_folder_field",
                String(f[key]),
                range(`folder.${key}`),
                "Unknown folder envelope field",
                undefined,
                `folder.${key}`,
              );
      }
      if (!Array.isArray(items))
        issue(
          "items_type",
          "",
          origin,
          "Expected an array of items",
          undefined,
          "items",
          "error",
        );
      else if (items.length > 1000)
        issue(
          "item_limit",
          "",
          origin,
          "At most 1000 items are allowed",
          undefined,
          "items",
          "error",
        );
      else
        items.forEach((value, index) => {
          const path = `${prefix}[${index}]`;
          const display = `items[${index}]`;
          if (!object(value)) {
            issue(
              "item_type",
              JSON.stringify(value),
              range(path),
              "Each item must be an object",
              undefined,
              display,
              "error",
            );
            return;
          }
          const c = card({}, range(path));
          const seen = new Set<string>();
          for (const [rawKey, v] of Object.entries(value)) {
            const key = aliases[rawKey] ?? rawKey;
            const r = range(`${path}.${rawKey}`);
            const p = `${display}.${rawKey}`;
            if (rawKey === "difficulty") {
              if (["easy", "medium", "hard"].includes(String(v)))
                c.difficulty = v as DraftCard["difficulty"];
              else if (
                folder.template_key === "interview_questions" &&
                typeof v === "number" &&
                Number.isInteger(v) &&
                v >= 1 &&
                v <= 5
              ) {
                c.profile = { ...c.profile, difficulty: v };
              } else
                issue(
                  "difficulty",
                  String(v),
                  r,
                  "Invalid difficulty",
                  c,
                  p,
                  "error",
                );
              continue;
            }
            if (
              folder.template_key === "interview_questions" &&
              [
                "frequency",
                "concepts",
                "keywords",
                "subtopic",
                "level",
              ].includes(key)
            ) {
              if (
                key === "frequency" &&
                typeof v === "number" &&
                Number.isInteger(v) &&
                v >= 1 &&
                v <= 10
              )
                c.profile = { ...c.profile, frequency: v };
              else if (
                ["concepts", "keywords"].includes(key) &&
                Array.isArray(v) &&
                v.length <= 31 &&
                v.every(
                  (x) =>
                    typeof x === "string" &&
                    x.trim().length > 0 &&
                    x.length <= 200,
                )
              )
                c.profile = {
                  ...c.profile,
                  concepts: [
                    ...(c.profile?.concepts ?? []),
                    ...(v as string[]),
                  ],
                };
              else if (
                key === "subtopic" &&
                typeof v === "string" &&
                v.length <= 200
              )
                c.profile = { ...c.profile, subtopic: v };
              else if (
                key === "level" &&
                typeof v === "string" &&
                ["junior", "middle", "senior"].includes(v.toLowerCase())
              ) {
                const n = { junior: 1, middle: 3, senior: 5 }[v.toLowerCase()]!;
                c.profile = { ...c.profile, level_min: n, level_max: n };
              } else {
                issue(
                  "profile_value",
                  JSON.stringify(v),
                  r,
                  "Invalid interview profile value",
                  c,
                  p,
                  "error",
                );
                continue;
              }
              if (activeMeta.has(key))
                setField(
                  c,
                  key,
                  typeof v === "string" ? v : JSON.stringify(v),
                  r,
                  p,
                );
              continue;
            }
            if (seen.has(key)) {
              issue(
                "alias_conflict",
                JSON.stringify(v),
                r,
                `Two fields map to ${key}`,
                c,
                p,
                "error",
              );
              continue;
            }
            seen.add(key);
            if (typeof v === "string") setField(c, key, v, r, p);
            else if (
              key === "sources" &&
              Array.isArray(v) &&
              v.every((x) => typeof x === "string")
            )
              setField(c, key, v.join("\n"), r, p);
            else
              issue(
                "field_type",
                JSON.stringify(v),
                r,
                "Expected a string value",
                c,
                p,
                "error",
              );
          }
        });
    } catch (error) {
      if (error instanceof JSONIssue)
        issue(
          error.code,
          source.slice(error.range.start, error.range.end),
          error.range,
          error.message,
          undefined,
          error.path,
          "error",
        );
      else
        issue(
          "invalid_json",
          "",
          origin,
          "Invalid JSON",
          undefined,
          undefined,
          "error",
        );
    }
  }
  if (draft.cards.length > 1000)
    issue(
      "item_limit",
      "",
      origin,
      "At most 1000 cards are allowed",
      undefined,
      undefined,
      "error",
    );
  revalidate(draft, folder);
  draft.automatic = structuredClone({
    cards: draft.cards,
    issues: draft.issues,
  });
  return draft;
}
export function revalidate(
  draft: ImportDraft,
  folder: ImportFolder,
): ImportDraft {
  const seen = new Map<string, string>();
  for (const c of draft.cards) {
    c.validation = [];
    c.duplicateOf = undefined;
    const maps: [Record<string, string>, typeof folder.config.schema.fields][] =
      [
        [c.values, folder.config.schema.fields],
        [c.metadata, folder.config.metadata_schema.fields],
      ];
    for (const [values, fields] of maps) {
      for (const f of fields)
        if (
          f.active &&
          f.required &&
          !values[f.key]?.trim() &&
          !(f.key === "example" && c.examples.some((e) => e.text.trim()))
        )
          c.validation.push(`${f.label ?? f.key} is required`);
      for (const [key, value] of Object.entries(values))
        if (
          value &&
          (!fields.some((f) => f.key === key && f.active) ||
            key === "transcription")
        )
          c.validation.push(`Unsupported field: ${key}`);
        else if (new TextEncoder().encode(value).length > 100000)
          c.validation.push(`Field ${key} exceeds 100000 bytes`);
    }
    if (
      c.examples.length &&
      !folder.config.schema.fields.some((f) => f.key === "example" && f.active)
    )
      c.validation.push("Examples are disabled in this folder");
    if (
      new TextEncoder().encode(c.examples.map((e) => e.text).join("\n"))
        .length > 100000
    )
      c.validation.push("Examples exceed 100000 bytes");
    const key = normalized(
      c.values[
        folder.template_key === "english_words" ? "foreign" : "question"
      ] ?? "",
    );
    if (!c.excluded && key) {
      c.duplicateOf = seen.get(key);
      if (!seen.has(key)) seen.set(key, c.id);
    }
    const issues = draft.issues.filter(
      (i) => i.cardId === c.id && i.blocking && !i.resolved,
    );
    c.status = c.excluded
      ? "excluded"
      : c.validation.length
        ? "invalid"
        : issues.length
          ? "needs_review"
          : "ready";
  }
  return draft;
}
export function record(draft: ImportDraft, action: string) {
  draft.updatedAt = Date.now();
  draft.history.push({ at: draft.updatedAt, action });
}
export function command(
  draft: ImportDraft,
  cards: DraftCard[] = draft.cards.filter((c) => !c.excluded),
): ImportCommand {
  return {
    command_id: createClientUUID(),
    duplicate_policy: "skip",
    items: cards.map((c) => ({
      client_id: c.id,
      values: Object.fromEntries(
        Object.entries({
          ...c.values,
          ...(c.examples.length
            ? { example: c.examples.map((e) => e.text).join("\n") }
            : {}),
        }).filter(([, v]) => v.trim()),
      ),
      metadata: Object.fromEntries(
        Object.entries(c.metadata).filter(([, v]) => v.trim()),
      ),
      difficulty: c.difficulty,
      profile: c.profile,
      duplicate_policy: c.duplicatePolicy,
    })),
  };
}
export function merge(draft: ImportDraft, into: string, from: string) {
  const a = draft.cards.find((c) => c.id === into),
    b = draft.cards.find((c) => c.id === from);
  if (!a || !b || a === b) return;
  for (const map of ["values", "metadata"] as const)
    for (const [k, v] of Object.entries(b[map]))
      if (v && !a[map][k]) a[map][k] = v;
      else if (v && a[map][k] !== v) a[map][k] += "\n" + v;
  a.examples.push(...b.examples);
  a.examples.sort((x, y) => x.range.start - y.range.start);
  b.excluded = true;
  for (const i of draft.issues) if (i.cardId === from) i.cardId = into;
  record(draft, `Merge ${from} into ${into}`);
}
export function split(draft: ImportDraft, id: string): DraftCard | undefined {
  const c = draft.cards.find((c) => c.id === id);
  if (!c) return;
  const copy = structuredClone(c);
  copy.id = `c-${createClientUUID()}`;
  copy.examples = [];
  copy.excluded = false;
  copy.values = {};
  copy.validation = [];
  draft.cards.splice(draft.cards.indexOf(c) + 1, 0, copy);
  record(draft, `Split ${id}: edit both cards to set the boundary`);
  return copy;
}
export function assign(
  draft: ImportDraft,
  issueId: string,
  cardId: string,
  field: string,
) {
  const i = draft.issues.find((i) => i.id === issueId),
    c = draft.cards.find((c) => c.id === cardId);
  if (!i || !c) return;
  if (field === "example")
    c.examples.push({
      id: `e-${createClientUUID()}`,
      text: i.fragment,
      range: i.range,
    });
  else
    c.values[field] = c.values[field]
      ? c.values[field] + "\n" + i.fragment
      : i.fragment;
  c.ranges[field] = i.range;
  i.resolved = "assigned";
  record(draft, `Assign ${issueId} to ${cardId}.${field}`);
}
export function moveExample(
  draft: ImportDraft,
  from: string,
  exampleId: string,
  to: string,
  copy = false,
) {
  const a = draft.cards.find((c) => c.id === from),
    b = draft.cards.find((c) => c.id === to),
    e = a?.examples.find((e) => e.id === exampleId);
  if (!a || !b || !e || a === b) return;
  b.examples.push({ ...e, id: copy ? createClientUUID() : e.id });
  b.examples.sort((x, y) => x.range.start - y.range.start);
  if (!copy) a.examples = a.examples.filter((e) => e.id !== exampleId);
  record(draft, `${copy ? "Copy" : "Move"} example ${exampleId} to ${to}`);
}

// A boundary alternative is a real state change. Keep the displaced translation
// as an unresolved fragment instead of silently dropping it.
export function chooseBoundary(
  draft: ImportDraft,
  issueId: string,
  targetId: string,
) {
  const issue = draft.issues.find((i) => i.id === issueId),
    card = draft.cards.find((c) => c.id === issue?.cardId),
    target = draft.cards.find((c) => c.id === targetId);
  if (!issue || !card || !target || card === target) return;
  target.examples.push({
    id: createClientUUID(),
    text: card.values.foreign,
    range: card.ranges.foreign ?? card.range,
  });
  target.examples.sort((a, b) => a.range.start - b.range.start);
  target.examples.push(...card.examples);
  card.excluded = true;
  issue.resolved = "assigned";
  const range = card.ranges.native ?? card.range;
  draft.issues.push({
    id: `i-${createClientUUID()}`,
    code: "boundary_translation",
    severity: "warning",
    fragment: draft.source.slice(range.start, range.end),
    range,
    message:
      "Translation displaced by the chosen boundary. Assign it to a supported field or explicitly exclude it.",
    blocking: true,
    actions: ["Assign to field", "Exclude fragment"],
  });
  record(
    draft,
    `Use ${card.id} as example of ${targetId}; translation preserved for review`,
  );
}
