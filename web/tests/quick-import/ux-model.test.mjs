import test from "node:test";
import assert from "node:assert/strict";
import {
  parseDraft,
  revalidate,
  command,
} from "../../.cache/quick-import/features/quick-import/model/draft.js";
import {
  diagnosticsFor,
  summarize,
  filteredCards,
  isCurrentPreview,
} from "../../.cache/quick-import/features/quick-import/model/diagnostics.js";
import {
  createHistory,
  recordEdit,
  undoEdit,
  redoEdit,
} from "../../.cache/quick-import/features/quick-import/model/history.js";
const folder = {
  id: "f",
  template_key: "english_words",
  config: {
    schema: {
      fields: [
        { key: "foreign", active: true, required: true },
        { key: "native", active: true, required: true },
        { key: "example", active: true },
      ],
    },
    metadata_schema: { fields: [] },
  },
};
const make = () => parseDraft("apple - яблоко\nbook - книга", "text", folder);
test("several errors count affected cards; filters do not mutate; excluded cards inactive", () => {
  const d = make();
  d.cards[0].values = { foreign: "", native: "" };
  revalidate(d, folder);
  const before = JSON.stringify(d),
    di = diagnosticsFor(d, folder),
    s = summarize(d, di);
  assert.equal(di.length, 2);
  assert.equal(s.issueCards.length, 1);
  assert.equal(s.readyCards.length, 1);
  assert.equal(filteredCards(d, di, "issues")[0].id, d.cards[0].id);
  assert.equal(JSON.stringify(d), before);
  d.cards[0].excluded = true;
  assert.equal(summarize(d, diagnosticsFor(d, folder)).issueCards.length, 0);
});
test("resolved fragments and corrected duplicates disappear, preserving source", () => {
  const d = parseDraft("apple - яблоко - эпл\napple - яблоко", "text", folder);
  assert.ok(
    diagnosticsFor(d, folder).some((x) => x.code === "unsupported_fragment"),
  );
  d.issues
    .filter((x) => x.code === "unsupported_fragment")
    .forEach((x) => (x.resolved = "excluded"));
  d.cards[1].values.foreign = "pear";
  revalidate(d, folder);
  assert.equal(diagnosticsFor(d, folder).filter((x) => x.blocking).length, 0);
  assert.match(d.source, /эпл/);
});
test("server mapping uses client_id and submitted subset; skipped repeats are info", () => {
  const d = make(),
    body = command(d, [d.cards[1]]);
  const result = {
    valid: false,
    created: 0,
    skipped: 0,
    failed: 1,
    items: [],
    issues: [
      {
        client_id: d.cards[1].id,
        path: "items[0].values.native",
        code: "required",
        severity: "error",
        message: "Translation is required",
      },
    ],
  };
  let di = diagnosticsFor(d, folder, { result, command: body });
  assert.equal(di[0].cardId, d.cards[1].id);
  assert.equal(di[0].field, "values.native");
  assert.equal(summarize(d, di).readyCards.length, 1);
  delete result.issues[0].client_id;
  assert.equal(
    diagnosticsFor(d, folder, { result, command: body })[0].cardId,
    d.cards[1].id,
  );
  result.issues = [];
  result.items = [
    { client_id: d.cards[1].id, status: "skipped", duplicate_kind: "existing" },
  ];
  di = diagnosticsFor(d, folder, { result, command: body });
  assert.equal(di[0].severity, "info");
  assert.equal(summarize(d, di).issueCards.length, 0);
  assert.equal(isCurrentPreview(1, 2), false);
});
test("history has five real undo steps, redo and stable source/ranges", () => {
  let d = make(),
    h = createHistory();
  const source = d.source,
    id = d.cards[0].id,
    range = d.cards[0].range;
  for (let i = 1; i <= 7; i++) {
    let n = structuredClone(d);
    n.cards[0].values.native = String(i);
    h = recordEdit(h, d, n);
    d = n;
  }
  assert.equal(h.past.length, 5);
  for (let i = 6; i >= 2; i--) {
    const r = undoEdit(h, d);
    h = r.history;
    d = r.draft;
    assert.equal(d.cards[0].values.native, String(i));
  }
  assert.equal(d.source, source);
  assert.equal(d.cards[0].id, id);
  assert.deepEqual(d.cards[0].range, range);
  let r = redoEdit(h, d);
  h = r.history;
  d = r.draft;
  assert.equal(d.cards[0].values.native, "3");
  const n = structuredClone(d);
  n.cards[0].excluded = true;
  h = recordEdit(h, d, n);
  assert.equal(h.future.length, 0);
  const base = make(),
    changed = structuredClone(base);
  changed.cards[0].excluded = true;
  const out = undoEdit(recordEdit(createHistory(), base, changed), changed);
  assert.equal(out.draft.automatic, changed.automatic);
});
test("typing groups by field and idle interval", () => {
  let d = make(),
    h = createHistory();
  for (let i = 0; i < 5; i++) {
    const n = structuredClone(d);
    n.cards[0].values.native = String(i);
    h = recordEdit(h, d, n, { group: "native", now: i * 100 });
    d = n;
  }
  assert.equal(h.past.length, 1);
  assert.equal(undoEdit(h, d).draft.cards[0].values.native, "яблоко");
  const n = structuredClone(d);
  n.cards[0].values.native = "last";
  h = recordEdit(h, d, n, { group: "native", now: 2000 });
  assert.equal(h.past.length, 2);
});
