import test from "node:test";
import assert from "node:assert/strict";

// Exercise the public storage API with asynchronous, serial transactions. The
// browser suite verifies the same recovery flow against real IndexedDB.
let databaseId = 0;
async function storage(initial = []) {
  const rows = new Map(initial.map((row) => [row.key, structuredClone(row)]));
  let tail = Promise.resolve();
  const database = {
    transaction(_name, mode = "readonly") {
      const operations = [];
      let aborted = false;
      const tx = { error: null };
      tx.abort = () => {
        aborted = true;
      };
      const request = (operation) => {
        const result = {};
        operations.push(() => {
          result.result = operation();
          result.onsuccess?.();
        });
        return result;
      };
      let snapshot;
      tx.objectStore = () => ({
        get: (key) => request(() => structuredClone(snapshot.get(key))),
        getAll: () => request(() => structuredClone([...snapshot.values()])),
        put: (row) =>
          request(() => snapshot.set(row.key, structuredClone(row))),
        delete: (key) => request(() => snapshot.delete(key)),
      });
      tail = tail.then(() => {
        snapshot = new Map(rows);
        while (operations.length && !aborted) operations.shift()();
        if (aborted) {
          tx.onabort?.();
        } else {
          if (mode === "readwrite") {
            rows.clear();
            for (const [key, row] of snapshot) rows.set(key, row);
          }
          tx.oncomplete?.();
        }
      });
      return tx;
    },
  };
  globalThis.indexedDB = {
    open() {
      const request = {};
      queueMicrotask(() => {
        request.result = database;
        request.onsuccess?.();
      });
      return request;
    },
  };
  const api = await import(
    `../../.cache/quick-import/features/quick-import/model/storage.js?case=${++databaseId}`
  );
  return { ...api, rows };
}
const command = (id = "command-1") => ({
  command_id: id,
  duplicate_policy: "skip",
  items: [
    {
      client_id: "card-1",
      values: { foreign: "run", native: "бежать" },
      metadata: {},
      difficulty: "medium",
    },
  ],
});
function saved(key = "folder-1", pending = undefined) {
  return {
    key,
    updatedAt: Date.now(),
    input: "run — бежать",
    kind: "text",
    draft: {
      version: 1,
      draftId: `draft-${key}`,
      folderId: key,
      source: "run — бежать",
      kind: "text",
      cards: [],
      issues: [],
      automatic: { cards: [], issues: [] },
      history: [],
      updatedAt: Date.now(),
      ...(pending ? { pending } : {}),
    },
  };
}

test("pending commands remain recoverable after the ordinary seven-day TTL", async () => {
  const pending = saved("pending", command());
  const ordinary = saved("ordinary");
  pending.updatedAt = ordinary.updatedAt = Date.now() - 8 * 86400000;
  const api = await storage([pending, ordinary]);
  assert.equal(
    (await api.loadDraft("pending"))?.draft.pending.command_id,
    "command-1",
  );
  assert.equal(await api.loadDraft("ordinary"), null);
});

test("saving newer drafts never evicts an unresolved pending command", async () => {
  const api = await storage();
  await api.saveDraft(saved("pending", command()), null);
  for (let i = 0; i < 7; i++) await api.saveDraft(saved(`ordinary-${i}`), null);
  assert.equal(
    (await api.loadDraft("pending"))?.draft.pending.command_id,
    "command-1",
  );
  assert.ok([...api.rows.values()].filter((row) => "draft" in row).length <= 5);
});

test("legacy version-one drafts acquire a revision without losing their source", async () => {
  const legacy = saved();
  const api = await storage([legacy]);
  const revision = await api.saveDraft(
    { ...legacy, input: "edited buffer" },
    0,
  );
  assert.equal(revision, 1);
  assert.equal(
    (await api.loadDraft(legacy.key)).draft.source,
    legacy.draft.source,
  );
});

test("stale tabs cannot overwrite a newer draft even with a newer wall clock", async () => {
  const api = await storage();
  const first = saved();
  const revision = await api.saveDraft(first, null);
  const edited = { ...first, input: "newer edit" };
  const nextRevision = await api.saveDraft(edited, revision);
  await assert.rejects(
    api.saveDraft(
      { ...first, input: "stale edit", updatedAt: Date.now() + 50000 },
      revision,
    ),
    /another tab|changed elsewhere/i,
  );
  assert.equal(nextRevision, revision + 1);
  assert.equal((await api.loadDraft(first.key)).input, "newer edit");
});

test("two competing absent-record saves cannot silently replace each other", async () => {
  const api = await storage();
  const outcomes = await Promise.allSettled([
    api.saveDraft(saved(), null),
    api.saveDraft({ ...saved(), input: "other tab" }, null),
  ]);
  assert.equal(outcomes.filter((r) => r.status === "fulfilled").length, 1);
  assert.equal(outcomes.filter((r) => r.status === "rejected").length, 1);
});

test("an unconfirmed command cannot be cleared or replaced by an autosave", async () => {
  const api = await storage();
  const pending = saved("folder-1", command());
  const revision = await api.saveDraft(pending, null);
  await assert.rejects(
    api.saveDraft(saved(), revision),
    /unconfirmed|pending/i,
  );
  await assert.rejects(
    api.saveDraft(saved("folder-1", command("different-command")), revision),
    /unconfirmed|pending/i,
  );
  const changedBody = saved("folder-1", command());
  changedBody.draft.pending.items[0].values.foreign = "changed";
  await assert.rejects(
    api.saveDraft(changedBody, revision),
    /unconfirmed|pending/i,
  );
  assert.deepEqual(
    (await api.loadDraft(pending.key)).draft.pending,
    pending.draft.pending,
  );
});

test("only the confirmed command may be cleared after a definitive response", async () => {
  const api = await storage();
  const revision = await api.saveDraft(saved("folder-1", command()), null);
  await assert.rejects(
    api.saveDraft(saved(), revision, "wrong-command"),
    /unconfirmed|pending/i,
  );
  const clearedRevision = await api.saveDraft(saved(), revision, "command-1");
  assert.equal(clearedRevision, revision + 1);
  assert.equal((await api.loadDraft("folder-1")).draft.pending, undefined);
});

test("discard and stale completion cannot delete an unconfirmed command", async () => {
  const api = await storage();
  const revision = await api.saveDraft(saved("folder-1", command()), null);
  await assert.rejects(
    api.deleteDraft("folder-1", revision),
    /unconfirmed|pending/i,
  );
  await assert.rejects(
    api.deleteDraft("folder-1", revision, "different-command"),
    /unconfirmed|pending/i,
  );
  await assert.rejects(
    api.deleteDraft("folder-1", revision - 1, "command-1"),
    /another tab|changed elsewhere/i,
  );
  assert.ok(await api.loadDraft("folder-1"));
  await api.deleteDraft("folder-1", revision, "command-1");
  assert.equal(await api.loadDraft("folder-1"), null);
});

test("source recovery distinguishes missing legacy input from an intentionally empty buffer", async () => {
  const api = await storage();
  const legacy = saved();
  delete legacy.input;
  assert.equal(api.savedInput(legacy), legacy.draft.source);
  assert.equal(api.savedInput({ ...legacy, input: "" }), "");
  assert.equal(api.savedInput({ ...legacy, input: "edited" }), "edited");
});

test("storage refuses another draft when all five recovery slots contain pending commands", async () => {
  const api = await storage();
  for (let i = 0; i < 5; i++)
    await api.saveDraft(saved(`pending-${i}`, command(`command-${i}`)), null);
  await assert.rejects(
    api.saveDraft(saved("new-folder"), null),
    /full of unconfirmed imports/i,
  );
  for (let i = 0; i < 5; i++)
    assert.equal(
      (await api.loadDraft(`pending-${i}`)).draft.pending.command_id,
      `command-${i}`,
    );
  assert.equal(await api.loadDraft("new-folder"), null);
});

test("a deleted and recreated key never reuses a stale tab's revision", async () => {
  const api = await storage();
  const staleRevision = await api.saveDraft(saved(), null);
  await api.deleteDraft("folder-1", staleRevision);
  const recreatedRevision = await api.saveDraft(
    { ...saved(), input: "recreated" },
    null,
  );
  assert.notEqual(recreatedRevision, staleRevision);
  await assert.rejects(
    api.saveDraft(saved(), staleRevision),
    /another tab|changed elsewhere/i,
  );
  await assert.rejects(
    api.deleteDraft("folder-1", staleRevision),
    /another tab|changed elsewhere/i,
  );
  assert.equal((await api.loadDraft("folder-1")).input, "recreated");
});

test("an expired ordinary record can be replaced after load returns no draft", async () => {
  const old = {
    ...saved(),
    revision: 10,
    updatedAt: Date.now() - 8 * 86400000,
  };
  const pending = { ...saved("pending", command()), updatedAt: old.updatedAt };
  const api = await storage([old, pending]);
  assert.equal(await api.loadDraft(old.key), null);
  const revision = await api.saveDraft(saved(), null);
  assert.ok(revision > old.revision);
  await assert.rejects(
    api.saveDraft(old, old.revision),
    /another tab|changed elsewhere/i,
  );
  assert.ok(await api.loadDraft("pending"));
});

test("retry saves retain exactly the original command payload and ID", async () => {
  const api = await storage();
  const pending = saved("pending", command());
  const firstRevision = await api.saveDraft(pending, null);
  const nextRevision = await api.saveDraft(
    { ...pending, updatedAt: Date.now() + 1 },
    firstRevision,
  );
  assert.ok(nextRevision > firstRevision);
  assert.deepEqual(
    (await api.loadDraft("pending")).draft.pending,
    pending.draft.pending,
  );
});

test("byte pressure fails safely instead of evicting pending command recovery", async () => {
  const OriginalEncoder = globalThis.TextEncoder;
  // Fixed measured fixture sizes exercise the 80 MiB quota without allocating
  // hundreds of MiB while the transaction fixture clones its in-memory rows.
  globalThis.TextEncoder = class {
    encode(value) {
      return {
        length: value.includes("quota-39-mib")
          ? 39 * 1024 * 1024
          : value.includes("quota-3-mib")
            ? 3 * 1024 * 1024
            : new OriginalEncoder().encode(value).length,
      };
    }
  };
  try {
    const first = {
      ...saved("pending-1", command("first")),
      input: "quota-39-mib",
    };
    const second = {
      ...saved("pending-2", command("second")),
      input: "quota-39-mib",
    };
    const api = await storage([first, second]);
    await assert.rejects(
      api.saveDraft({ ...saved("new"), input: "quota-3-mib" }, null),
      /full of unconfirmed imports/i,
    );
    assert.equal(
      (await api.loadDraft("pending-1")).draft.pending.command_id,
      "first",
    );
    assert.equal(
      (await api.loadDraft("pending-2")).draft.pending.command_id,
      "second",
    );
    assert.equal(await api.loadDraft("new"), null);
  } finally {
    globalThis.TextEncoder = OriginalEncoder;
  }
});
