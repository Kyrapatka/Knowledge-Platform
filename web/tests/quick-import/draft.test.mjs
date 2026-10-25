import test from "node:test";
import assert from "node:assert/strict";
import {
  parseDraft,
  revalidate,
} from "../../.cache/quick-import/features/quick-import/model/draft.js";
export const folder = {
  id: "folder",
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
test("mixed source has exact field/example ranges and stable IDs", () => {
  const source =
    "1. take care of заботиться о\r\ntake care of dog\r\nsubtle - тонкий - сатл";
  const d = parseDraft(source, "text", folder);
  assert.equal(d.cards.length, 2);
  assert.equal(d.cards[0].values.foreign, "take care of");
  assert.equal(d.cards[0].examples[0].text, "take care of dog");
  for (const c of d.cards)
    for (const e of c.examples)
      assert.equal(source.slice(e.range.start, e.range.end), e.text);
  const unknown = d.issues.find((i) => i.fragment === "сатл");
  assert.ok(unknown);
  assert.equal(source.slice(unknown.range.start, unknown.range.end), "сатл");
  d.cards[0].values.native = "присматривать";
  assert.equal(revalidate(d, folder).cards[0].values.native, "присматривать");
  assert.equal(revalidate(d, folder).cards[0].id, d.cards[0].id);
});
test("JSON aliases and strict duplicate keys", () => {
  const d = parseDraft(
    '[{"word":"run","translation":"бежать","pronunciation":"ран"}]',
    "json",
    folder,
  );
  assert.equal(d.cards[0].values.foreign, "run");
  assert.equal(d.cards[0].values.transcription, undefined);
  assert.ok(d.issues.some((i) => i.path === "items[0].pronunciation"));
  const bad = parseDraft(
    '[{"word":"run","word":"walk","translation":"идти"}]',
    "json",
    folder,
  );
  assert.ok(bad.issues.some((i) => i.code === "duplicate_json_key"));
});
test("two-line boundary ambiguity and overwritten named word stay reviewable", () => {
  const d = parseDraft(
    "run - бегать\nrun fast\nбыстро бежать\nWord: lost\nWord: kept\nTranslation: сохранённый",
    "text",
    folder,
  );
  assert.ok(d.issues.some((i) => i.code === "boundary_alternative"));
  assert.ok(d.issues.some((i) => i.fragment.includes("lost")));
});
