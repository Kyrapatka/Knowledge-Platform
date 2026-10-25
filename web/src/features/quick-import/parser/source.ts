import type { Range } from "../model/types.js";
export type Row = { text: string; range: Range };
export function rows(source: string): Row[] {
  const out: Row[] = [];
  const re = /[^\r\n]*(?:\r\n|\r|\n|$)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(source)) && m[0]) {
    const text = m[0].replace(/[\r\n]+$/, "");
    out.push({
      text,
      range: {
        start: m.index,
        end: m.index + text.length,
        line: out.length + 1,
      },
    });
  }
  return out;
}
export function fragmentRange(row: Row, text: string): Range {
  let index = row.text.indexOf(text);
  let length = text.length;
  if (index < 0 && text.trim()) {
    const escaped = text
      .trim()
      .split(/\s+/)
      .map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
      .join("\\s+");
    const match = new RegExp(escaped).exec(row.text);
    if (match) {
      index = match.index;
      length = match[0].length;
    }
  }
  return index < 0
    ? row.range
    : {
        start: row.range.start + index,
        end: row.range.start + index + length,
        line: row.range.line,
      };
}
export function hash(text: string) {
  let h = 2166136261;
  for (let i = 0; i < text.length; i++)
    h = Math.imul(h ^ text.charCodeAt(i), 16777619);
  return (h >>> 0).toString(36);
}
export const normalized = (s: string) =>
  s.trim().replace(/\s+/g, " ").toLowerCase();
export const tokens = (s: string) =>
  normalized(s)
    .split(/[^\p{L}\p{N}'+-]+/u)
    .filter(Boolean);
export function phrase(word: string, text: string) {
  const w = tokens(word),
    t = tokens(text);
  return w.length > 0 && t.some((_, i) => w.every((v, j) => t[i + j] === v));
}
