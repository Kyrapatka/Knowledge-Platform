import test from "node:test";
import assert from "node:assert/strict";
import { parseText } from "../../../.cache/quick-import/features/quick-import/parser/reference.js";
function expectCards(name, input, words, examples = []) {
  test(name, () => {
    const r = parseText(input);
    assert.deepEqual(
      r.cards.map((c) => c.word),
      words,
    );
    assert.deepEqual(
      r.cards.map((c) => c.examples),
      examples.length ? examples : words.map(() => []),
    );
  });
}
expectCards("english cyrillic separated with tabs", "apple\tяблоко", ["apple"]);
expectCards(
  "no separators mixed lengths",
  "take care of заботиться о\ntake care of dog",
  ["take care of"],
  [["take care of dog"]],
);
expectCards(
  "5 words example without matching word",
  "apple яблоко\nThis is a nice little sentence.",
  ["apple"],
  [["This is a nice little sentence."]],
);
expectCards(
  "multi card separated by named fields",
  "apple яблоко\nword: book\ntranslation: книга",
  ["apple", "book"],
);
expectCards("two line cards after existing card", "apple яблоко\nbook\nкнига", [
  "apple",
  "book",
]);
expectCards("long phrase bilingual", "I love you Я тебя люблю", ["I love you"]);
expectCards("word with slash", "reliable/rely надёжный/полагаться", [
  "reliable/rely",
]);
expectCards("punctuation in translation", "run бегать, управлять, запускать", [
  "run",
]);
expectCards(
  "example explicitly bound to local card",
  "apple яблоко\nbook книга\nExample: Apple is delicious.",
  ["apple", "book"],
  [[], ["Apple is delicious."]],
);
expectCards(
  "lowercase phrase and quotes",
  'hissed прошипел\n"Don\'t move," she hissed.',
  ["hissed"],
  [['"Don\'t move," she hissed.']],
);
expectCards(
  "CRLF line ending",
  "apple яблоко\r\nI ate an apple.",
  ["apple"],
  [["I ate an apple."]],
);
expectCards("numbered list", "1. apple яблоко\n2. book книга", [
  "apple",
  "book",
]);
test("short unrelated line not silently attached", () => {
  const r = parseText("apple яблоко\nHe smiled.");
  assert.equal(r.cards[0].examples.length, 0);
  assert.equal(r.unassigned.length, 1);
  assert.equal(r.issues[0].code, "unassigned_example");
});
test("unsupported notes not imported as examples", () => {
  const r = parseText(
    "apple яблоко\nчасть речи: существительное\nI ate an apple.",
  );
  assert.equal(r.cards[0].examples.length, 1);
  assert.ok(r.issues.some((i) => i.code === "unsupported_field"));
});
test("unsupported pronunciation named field", () => {
  const r = parseText("apple яблоко\ntranscription: эпл");
  assert.equal(r.cards[0].examples.length, 0);
  assert.equal(r.issues[0].code, "unsupported_field");
});
test("missing right side of pair error", () => {
  const r = parseText("apple -");
  assert.equal(r.cards.length, 0);
  assert.ok(r.issues.some((i) => i.code === "incomplete_pair"));
});
test("missing left side of pair error", () => {
  const r = parseText("- яблоко");
  assert.equal(r.cards.length, 0);
  assert.ok(r.issues.some((i) => i.code === "incomplete_pair"));
});
test("two matches remain unassigned", () => {
  const r = parseText("apple яблоко\nbook книга\nI wrote a book about apple.");
  assert.equal(r.unassigned.length, 1);
  assert.equal(r.issues[0].code, "ambiguous_example");
});
