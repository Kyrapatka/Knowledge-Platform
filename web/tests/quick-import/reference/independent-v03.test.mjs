import test from "node:test";
import assert from "node:assert/strict";
import { parseText } from "../../../.cache/quick-import/features/quick-import/parser/reference.js";
const cases = [
  ["bare bilingual pair", "ate ел", ["ate"], [[]]],
  [
    "phrase without delimiter",
    "take care of заботиться о\ntake care of dog",
    ["take care of"],
    [["take care of dog"]],
  ],
  [
    "interleaved syntax",
    "word: calm\ntranslation: спокойный\nreliable/rely надёжный/полагаться",
    ["calm", "reliable/rely"],
    [[], []],
  ],
  [
    "cross-card example",
    "apple яблоко\nbook книга\nI ate an apple yesterday.",
    ["apple", "book"],
    [["I ate an apple yesterday."], []],
  ],
  [
    "duplicate word caution",
    "run бегать\nrun управлять",
    ["run", "run"],
    [[], []],
  ],
  ["unsupported field", "subtle тонкий\npronunciation: сатл", ["subtle"], [[]]],
  [
    "mixed examples",
    "apple яблоко\nI ate an apple.\nbook книга\nI bought a book.",
    ["apple", "book"],
    [["I ate an apple."], ["I bought a book."]],
  ],
  ["full phrase translation", "I love you Я тебя люблю", ["I love you"], [[]]],
  ["unmatched english note", "apple яблоко\nHe smiled.", ["apple"], [[]]],
  ["multitranslation", "run - бегать, управлять, запускать", ["run"], [[]]],
];
for (const [title, input, words, examples] of cases)
  test("heldout: " + title, () => {
    const result = parseText(input);
    assert.deepEqual(
      result.cards.map((c) => c.word),
      words,
    );
    assert.deepEqual(
      result.cards.map((c) => c.examples),
      examples,
    );
  });
test("heldout: review offers grounded alternatives", () => {
  const r = parseText("apple яблоко\nbook книга\nI bought a book about apple.");
  assert.equal(r.unassigned.length, 1);
  assert.equal(r.reviews.length, 1);
  assert.deepEqual(
    new Set(r.reviews[0].alternatives.map((a) => a.word)),
    new Set(["apple", "book"]),
  );
  assert.ok(r.reviews[0].alternatives[0].reasons.length);
});
test("heldout: unsupported field not silently discarded", () => {
  const r = parseText("subtle тонкий\ntranscription: сатл");
  assert.ok(
    r.issues.some(
      (x) => x.code === "unsupported_field" && x.fragment.includes("сатл"),
    ),
  );
});
