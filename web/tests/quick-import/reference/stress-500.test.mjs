import test from "node:test";
import assert from "node:assert/strict";
import { performance } from "node:perf_hooks";
import { parseText } from "../../../.cache/quick-import/features/quick-import/parser/reference.js";

// Fixed-seed 500-card mixed-format fixture with an independent oracle.
// The oracle is created before calling the parser, rather than inferred from its output.
const adjectives = [
  ["quiet", "тихий"],
  ["bright", "яркий"],
  ["subtle", "тонкий"],
  ["mature", "зрелый"],
  ["reliable", "надёжный"],
  ["careful", "осторожный"],
  ["strange", "странный"],
  ["gentle", "мягкий"],
  ["simple", "простой"],
  ["clever", "умный"],
  ["strong", "сильный"],
  ["warm", "тёплый"],
  ["fresh", "свежий"],
  ["modern", "современный"],
  ["secret", "секретный"],
  ["rare", "редкий"],
  ["honest", "честный"],
  ["silent", "безмолвный"],
  ["local", "местный"],
  ["distant", "далёкий"],
];
const nouns = [
  ["signal", "сигнал"],
  ["story", "рассказ"],
  ["plan", "план"],
  ["answer", "ответ"],
  ["message", "сообщение"],
  ["idea", "идея"],
  ["friend", "друг"],
  ["project", "проект"],
  ["question", "вопрос"],
  ["place", "место"],
  ["change", "изменение"],
  ["design", "дизайн"],
  ["path", "путь"],
  ["decision", "решение"],
  ["problem", "проблема"],
  ["lesson", "урок"],
  ["example", "пример"],
  ["method", "метод"],
  ["system", "система"],
  ["result", "результат"],
  ["detail", "деталь"],
  ["moment", "момент"],
  ["picture", "картина"],
  ["memory", "память"],
  ["conversation", "разговор"],
];
let state = 20261024;
function rand() {
  state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
  return state / 4294967296;
}
const shuffle = (a) => {
  for (let i = a.length - 1; i > 0; i--) {
    let j = Math.floor(rand() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
};
export function makeStressFixture() {
  const pairs = shuffle(
    adjectives.flatMap(([a, ar]) =>
      nouns.map(([n, nr]) => ({
        word: `${a} ${n}`,
        translation: `${ar} ${nr}`,
      })),
    ),
  );
  const chunks = [],
    expected = [],
    faultLines = [];
  for (let i = 0; i < pairs.length; i++) {
    const p = pairs[i],
      format = i % 8;
    const ex = [];
    if (i % 3 !== 0) ex.push(`I remember the ${p.word} from yesterday.`);
    if (i % 7 === 0) ex.push(`The ${p.word} seems important to everyone.`);
    let head;
    if (format === 0) head = `${p.word} - ${p.translation}`;
    else if (format === 1) head = `${p.word} — ${p.translation}`;
    else if (format === 2) head = `${p.word} ${p.translation}`;
    else if (format === 3)
      head = `word: ${p.word}\ntranslation: ${p.translation}`;
    else if (format === 4) head = `${p.word}\n${p.translation}`;
    else if (format === 5) head = `${i + 1}. ${p.word} - ${p.translation}`;
    else if (format === 6) head = `${p.word} = ${p.translation}`;
    else head = `${p.word}: ${p.translation}`;
    const lines = [head];
    if (ex.length) {
      lines.push(
        ...ex.map((s, j) => (j === 0 && i % 5 === 0 ? `Example: ${s}` : s)),
      );
    }
    if (i % 19 === 0) {
      lines.push("transcription: тестовая запись");
      faultLines.push(i);
    }
    chunks.push(lines.join("\n"));
    expected.push({ ...p, examples: ex });
  }
  // Extra malformed fragments are intentionally outside the 500 expected cards.
  chunks.splice(250, 0, "orphan -\n");
  chunks.splice(400, 0, "part of speech: noun\n");
  return { input: chunks.join("\n\n"), expected, faultLines };
}

test("stress: 500 varied cards preserve boundaries, values, examples, and flagged data", () => {
  const fixture = makeStressFixture();
  const start = performance.now();
  const actual = parseText(fixture.input);
  const elapsed = performance.now() - start;
  const actualByWord = new Map(actual.cards.map((c) => [c.word, c]));
  const missing = fixture.expected.filter((e) => !actualByWord.has(e.word));
  const wrongTranslations = fixture.expected.filter(
    (e) =>
      actualByWord.has(e.word) &&
      actualByWord.get(e.word).translation !== e.translation,
  );
  const wrongExamples = fixture.expected.filter(
    (e) =>
      actualByWord.has(e.word) &&
      JSON.stringify(actualByWord.get(e.word).examples) !==
        JSON.stringify(e.examples),
  );
  const summary = {
    expected: 500,
    actual: actual.cards.length,
    missing: missing.length,
    wrongTranslations: wrongTranslations.length,
    wrongExamples: wrongExamples.length,
    issues: actual.issues.length,
    unassigned: actual.unassigned.length,
    elapsedMs: Math.round(elapsed),
    firstWrongExamples: wrongExamples.slice(0, 4).map((e) => ({
      word: e.word,
      expected: e.examples,
      actual: actualByWord.get(e.word)?.examples,
    })),
  };
  console.log("STRESS_500_RESULT", JSON.stringify(summary));
  assert.equal(actual.cards.length, 500, "exact count");
  assert.deepEqual(missing, [], "no missing words");
  assert.deepEqual(wrongTranslations, [], "translations equal oracle");
  assert.deepEqual(wrongExamples, [], "examples equal oracle");
  assert.ok(
    actual.issues.filter((i) => i.code === "unsupported_field").length >=
      fixture.faultLines.length + 1,
    "all unsupported fields are reported",
  );
  assert.ok(
    actual.issues.some((i) => i.code === "incomplete_pair"),
    "orphan broken pair is reported",
  );
  assert.ok(elapsed < 5000, "stress parse under 5s on this machine");
});

test("stress: 500 entries with 25 malformed cards do not silently become valid", () => {
  const fixture = makeStressFixture();
  const src = fixture.input.split(/\n\n/);
  const removed = new Set();
  let replaced = 0;
  for (let i = 0; i < src.length && replaced < 25; i++) {
    const head = src[i].split("\n")[0];
    if (
      head.startsWith("word:") ||
      head.startsWith("part of speech:") ||
      head === "orphan -"
    )
      continue;
    if (i % 17 === 0) {
      const word = fixture.expected.find((x) => head.includes(x.word))?.word;
      if (!word) continue;
      src[i] = `${word} -`;
      removed.add(word);
      replaced++;
    }
  }
  assert.equal(replaced, 25);
  const result = parseText(src.join("\n\n"));
  const actualWords = new Set(result.cards.map((c) => c.word));
  const missingGood = fixture.expected.filter(
    (e) => !removed.has(e.word) && !actualWords.has(e.word),
  );
  const mistakenlyCreated = [...removed].filter((w) => actualWords.has(w));
  console.log(
    "STRESS_MALFORMED_RESULT",
    JSON.stringify({
      expectedValid: 475,
      actualCards: result.cards.length,
      missingGood: missingGood.length,
      mistakenlyCreated: mistakenlyCreated.length,
      incompletePairIssues: result.issues.filter(
        (x) => x.code === "incomplete_pair",
      ).length,
    }),
  );
  assert.equal(result.cards.length, 475);
  assert.deepEqual(missingGood, []);
  assert.deepEqual(mistakenlyCreated, []);
  assert.ok(
    result.issues.filter((x) => x.code === "incomplete_pair").length >= 25,
  );
});
