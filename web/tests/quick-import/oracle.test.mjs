import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { performance } from "node:perf_hooks";
import {
  parseDraft,
  command,
  merge,
  split,
  moveExample,
  revalidate,
} from "../../.cache/quick-import/features/quick-import/model/draft.js";
const folder = {
  id: "f",
  template_key: "english_words",
  config: {
    schema: {
      fields: [
        { key: "foreign", active: true, required: true },
        { key: "native", active: true, required: true },
        { key: "example", active: true },
        { key: "note", active: true },
      ],
    },
    metadata_schema: { fields: [] },
  },
};
const load = (name) =>
  readFileSync(new URL("./fixtures/" + name, import.meta.url), "utf8");
const oracle = JSON.parse(load("lexicon-500.json"));
function entry(c, i, damaged = false) {
  const pair = [
    `${c.foreign} ${c.native}`,
    `${c.foreign} - ${c.native}`,
    `${c.foreign} — ${c.native}`,
    `${c.foreign}: ${c.native}`,
    `${c.foreign} = ${c.native}`,
    `${c.foreign}\t${c.native}`,
    `${i + 1}. ${c.foreign}; ${c.native}`,
    `Word: ${c.foreign}\nTranslation: ${c.native}`,
    `${c.foreign}\n${c.native}`,
    `${c.foreign}, ${c.native}`,
  ][i % 10];
  if (damaged && i % 20 === 0) return `${c.foreign} -`;
  return (
    pair +
    (damaged && i % 20 === 1 ? " - неизвестный" : "") +
    c.examples.map((e) => "\nExample: " + e).join("")
  );
}
const actual = (d) =>
  d.cards.map((c) => ({
    foreign: c.values.foreign,
    native: c.values.native,
    examples: c.examples.map((e) => e.text),
  }));
function checkRanges(d) {
  for (const c of d.cards) {
    assert.ok(c.ranges.foreign.start >= 0);
    assert.ok(c.ranges.native.end <= d.source.length);
    for (const e of c.examples)
      assert.equal(d.source.slice(e.range.start, e.range.end), e.text);
  }
  for (const i of d.issues)
    if (i.fragment)
      assert.equal(d.source.slice(i.range.start, i.range.end), i.fragment);
}
test("all 14 real-note cards and 12 examples match a fixed independent oracle", () => {
  const source = load("user-notes.txt");
  const d = parseDraft(source, "text", folder);
  assert.deepEqual(actual(d), JSON.parse(load("user-notes-oracle.json")));
  assert.deepEqual(
    d.issues.map((i) => i.fragment),
    ["сатл"],
  );
  checkRanges(d);
});
test("500 diverse vocabulary entries, mixed boundaries, 0–3 examples: complete oracle", () => {
  assert.equal(new Set(oracle.map((c) => c.foreign)).size, 500);
  const source = oracle.map((c, i) => entry(c, i)).join("\n\n");
  const heap = process.memoryUsage().heapUsed,
    start = performance.now();
  const d = parseDraft(source, "text", folder);
  const elapsed = performance.now() - start;
  assert.deepEqual(actual(d), oracle);
  checkRanges(d);
  assert.equal(d.cards.length, 500);
  assert.ok(elapsed < 10000, `parse took ${elapsed} ms`);
  console.log(
    "QUICK_IMPORT_DIVERSE_500",
    JSON.stringify({
      cards: d.cards.length,
      examples: d.cards.reduce((n, c) => n + c.examples.length, 0),
      elapsedMs: Math.round(elapsed),
      heapDeltaBytes: process.memoryUsage().heapUsed - heap,
      diagnostics: d.issues.length,
    }),
  );
});
test("damaged 500-entry corpus preserves 475 complete entries and every damaged fragment", () => {
  const source = oracle.map((c, i) => entry(c, i, true)).join("\n\n");
  const start = performance.now();
  const d = parseDraft(source, "text", folder);
  assert.deepEqual(
    actual(d),
    oracle.filter((_, i) => i % 20 !== 0),
  );
  assert.equal(d.issues.filter((i) => i.code === "incomplete_pair").length, 25);
  assert.equal(d.issues.filter((i) => i.fragment === "неизвестный").length, 25);
  checkRanges(d);
  console.log(
    "QUICK_IMPORT_DAMAGED_500",
    JSON.stringify({
      cards: d.cards.length,
      incomplete: 25,
      unknown: 25,
      elapsedMs: Math.round(performance.now() - start),
    }),
  );
});
test("global example alternatives and exact lexical token boundaries", () => {
  const d = parseDraft(
    "apple - яблоко\nbook - книга\nI ate an apple\nI bought a book\nApple book\nrun - бегать\nrunning",
    "text",
    folder,
  );
  assert.deepEqual(
    d.cards[0].examples.map((e) => e.text),
    ["I ate an apple"],
  );
  assert.deepEqual(
    d.cards[1].examples.map((e) => e.text),
    ["I bought a book"],
  );
  assert.ok(
    d.issues.some(
      (i) => i.code === "ambiguous_example" && i.alternatives.length >= 2,
    ),
  );
  assert.ok(d.issues.some((i) => i.fragment === "running"));
});
test("manual operations retain original snapshot and example serialization order", () => {
  const d = parseDraft(
    "apple - яблоко\nExample: first\nExample: second\nbook - книга",
    "text",
    folder,
  );
  const [a, b] = d.cards;
  moveExample(d, a.id, a.examples[1].id, b.id);
  merge(d, b.id, a.id);
  b.values.native = "исправлено";
  revalidate(d, folder);
  assert.equal(b.values.native, "исправлено");
  assert.deepEqual(command(d).items[0].values.example, "first\nsecond");
  assert.equal(d.automatic.cards[0].examples.length, 2);
  const c = split(d, b.id);
  assert.equal(c.status, "ready");
  revalidate(d, folder);
  assert.equal(c.status, "invalid");
  assert.equal(d.source.includes("apple"), true);
});
test("JSON syntax, duplicate keys, template and limits cannot silently discard content", () => {
  for (const [s, code] of [
    ['[{"word":"a","word":"b"}]', "duplicate_json_key"],
    ["[] garbage", "invalid_json"],
    [
      '{"format_version":2,"folder":{"template":"english_words"},"items":[]}',
      "format_version",
    ],
    [
      '{"format_version":1,"folder":{"template":"interview_questions"},"items":[]}',
      "template_mismatch",
    ],
  ])
    assert.ok(
      parseDraft(s, "json", folder).issues.some((i) => i.code === code),
    );
  assert.ok(
    parseDraft(
      JSON.stringify(
        Array.from({ length: 1001 }, () => ({ word: "x", translation: "х" })),
      ),
      "json",
      folder,
    ).issues.some((i) => i.code === "item_limit"),
  );
});
test("interview labels and supported custom fields do not invoke EN/RU pairing", () => {
  const f = {
    ...folder,
    template_key: "interview_questions",
    config: {
      schema: {
        fields: ["question", "short_answer", "answer", "note", "sources"].map(
          (key) => ({ key, active: true, required: key === "question" }),
        ),
      },
      metadata_schema: { fields: [] },
    },
  };
  const d = parseDraft(
    "Question: What is a closure?\nShort answer: It captures lexical state.\nNote: inspect the scope\n\nQuestion: Зачем mutex?",
    "text",
    f,
  );
  assert.equal(d.cards.length, 2);
  assert.equal(d.cards[0].values.short_answer, "It captures lexical state.");
  assert.equal(d.cards[1].values.short_answer, undefined);
  assert.equal(d.issues.length, 0);
});
