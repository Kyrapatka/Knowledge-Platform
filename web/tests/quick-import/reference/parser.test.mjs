import test from "node:test";
import assert from "node:assert/strict";
import { parseText } from "../../../.cache/quick-import/features/quick-import/parser/reference.js";
const check = (title, source, count, examples = 0, issues = 0) =>
  test(title, () => {
    const x = parseText(source);
    assert.equal(x.cards.length, count, JSON.stringify(x));
    assert.equal(
      x.cards.reduce((a, c) => a + c.examples.length, 0),
      examples,
      JSON.stringify(x),
    );
    assert.equal(x.issues.length, issues, JSON.stringify(x));
  });
check("short bilingual pair", "ate ел", 1);
check("multitoken phrase", "take care of заботиться о", 1);
check("slash phrase stays single", "reliable/rely надёжный/полагаться", 1);
check(
  "short example with repetition",
  "take care of заботиться о\ntake care of dog",
  1,
  1,
);
check("four word example", "ate ел\nI ate already today", 1, 1);
check("new cards without blank line", "apple яблоко\nbook книга", 2);
check(
  "interleaved cards examples",
  "run бегать\nI run.\nbook книга\nI bought a book yesterday.",
  2,
  2,
);
check(
  "global matching",
  "apple яблоко\nbook книга\nI bought a book yesterday.\nI ate an apple this morning.",
  2,
  2,
);
check(
  "two examples",
  "subtle - тонкий\nThere was a subtle difference in her voice.\nI like the subtle design of this website.",
  1,
  2,
);
check(
  "unknown pronunciation",
  "subtle - тонкий - сатл\nThere was a subtle difference in her voice.",
  1,
  1,
  1,
);
check(
  "named fields",
  "word: scowl\ntranslation: сердитый взгляд\nexample: She answered with a scowl.",
  1,
  1,
);
check(
  "mixed format",
  "odd = странный\nIt is odd.\nbiased — предвзятый\nYou are biased.",
  2,
  2,
);
check("duplicate check", "run бегать\nrun управлять", 2, 0, 1);
check("no cards warning", "I ate an apple.", 0, 0, 1);
check(
  "multiple matches review",
  "apple яблоко\nbook книга\nI bought a book about apple.",
  2,
  0,
  1,
);
check("hyphenated term", "well-known общеизвестный", 1);
check("English phrase and Russian phrase", "I love you Я тебя люблю", 1);
check(
  "paragraphs separated",
  "chick-lit - литература для девушек\n\nreliable/rely - надёжный/полагаться",
  2,
);
check("line break word and translation", "apple\nяблоко", 1);
check(
  "many cards",
  Array.from({ length: 500 }, (_, i) => `word${i} слово${i}`).join("\n"),
  500,
);
