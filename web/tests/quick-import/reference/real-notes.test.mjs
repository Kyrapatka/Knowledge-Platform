import test from "node:test";
import assert from "node:assert/strict";
import { parseText } from "../../../.cache/quick-import/features/quick-import/parser/reference.js";
export const text = `subtle - едва уловимый/тонкий - сатл
There was a subtle difference in her voice.
I like the subtle design of this website.

mature - зрелый/взрослый
Jane made a mature decision to apologize

chick-lit - литература для девушек

reliable/rely - надёжный/полагаться

wields - владеть (уметь обращаться)
Nobody quite wields a heavy mace like me

Hissed - прошипел
"Don't move," she hissed in my ear.

abomination - жуткий монстр (мерзость)
That undead monster is an abomination.

squinting - прищуриваться
He was squinting at the screen, trying to read the tiny text

scowl - сердитый взгляд
She answered with a scowl on her face.

odd - странно
It's odd that he didn't call

biased - предвзятый
You are biased because he is your friend

It slips my mind - вылетело из головы

slips - мелкие ошибки
The teacher noticed a few minor slips in his essay.

striving - стремится (к успеху)
She is always striving to be the best in her field`;
test("original user notes count and integrity", () => {
  const result = parseText(text);
  assert.equal(result.cards.length, 14);
  assert.equal(
    result.cards.reduce((n, c) => n + c.examples.length, 0),
    12,
  );
  assert.equal(result.issues.length, 1);
  assert.equal(result.issues[0].fragment, "сатл");
  assert.equal(
    result.cards.find((c) => c.word === "reliable/rely")?.translation,
    "надёжный/полагаться",
  );
});
