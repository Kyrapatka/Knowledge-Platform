import { readFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";
import { ModuleKind, ScriptTarget, transpileModule } from "typescript";
import type { SavedDraft } from "../src/features/quick-import/model/storage";

declare global {
  interface Window {
    recoveryStorage: typeof import("../src/features/quick-import/model/storage");
  }
}
const source = readFileSync(
  new URL("../src/features/quick-import/model/storage.ts", import.meta.url),
  "utf8",
);
const javascript = transpileModule(source, {
  compilerOptions: { module: ModuleKind.ESNext, target: ScriptTarget.ES2022 },
}).outputText;
const moduleURL = `data:text/javascript;base64,${Buffer.from(javascript).toString("base64")}`;

async function openStorage(page: Page) {
  // The empty same-origin page isolates real IndexedDB from application boot,
  // login, API mocks and server availability; two tabs still share one database.
  await page.route("**/__quick-import-storage-test", (route) =>
    route.fulfill({
      contentType: "text/html",
      body: "<!doctype html><title>Recovery storage test</title>",
    }),
  );
  await page.goto("/__quick-import-storage-test");
  await page.evaluate(async (url) => {
    window.recoveryStorage = await import(/* @vite-ignore */ url);
  }, moduleURL);
}
function saved(key: string, pending = false): SavedDraft {
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
      ...(pending
        ? {
            pending: {
              command_id: "immutable-command",
              duplicate_policy: "skip",
              items: [],
            },
          }
        : {}),
    },
  };
}

test("real IndexedDB serializes competing tabs and rejects stale revisions", async ({
  page,
  context,
}) => {
  const other = await context.newPage();
  await openStorage(page);
  await openStorage(other);
  const first = saved("shared-folder");
  const second = { ...first, input: "other tab" };
  const results = await Promise.all([
    page.evaluate(async (row) => {
      try {
        return { revision: await window.recoveryStorage.saveDraft(row, null) };
      } catch (error) {
        return { error: (error as Error).name };
      }
    }, first),
    other.evaluate(async (row) => {
      try {
        return { revision: await window.recoveryStorage.saveDraft(row, null) };
      } catch (error) {
        return { error: (error as Error).name };
      }
    }, second),
  ]);
  expect(
    results.filter((result) => result.revision !== undefined),
  ).toHaveLength(1);
  expect(
    results.filter((result) => result.error === "DraftConflictError"),
  ).toHaveLength(1);
  const loaded = await other.evaluate(() =>
    window.recoveryStorage.loadDraft("shared-folder"),
  );
  expect(loaded?.revision).toBeGreaterThan(0);
  const revision = await page.evaluate(
    (row) =>
      window.recoveryStorage.saveDraft(
        { ...row, input: "latest edit" },
        row.revision,
      ),
    loaded!,
  );
  expect(revision).toBeGreaterThan(loaded!.revision!);
  const stale = await other.evaluate(async (row) => {
    try {
      await window.recoveryStorage.saveDraft(
        { ...row, input: "stale" },
        row.revision,
      );
    } catch (error) {
      return (error as Error).name;
    }
  }, loaded!);
  expect(stale).toBe("DraftConflictError");
  expect(
    (
      await page.evaluate(() =>
        window.recoveryStorage.loadDraft("shared-folder"),
      )
    )?.input,
  ).toBe("latest edit");
});

test("real IndexedDB preserves old pending commands through eviction and guarded deletion", async ({
  page,
  context,
}) => {
  await openStorage(page);
  const pending = {
    ...saved("pending-folder", true),
    updatedAt: Date.now() - 8 * 86400000,
  };
  await page.evaluate(
    (row) => window.recoveryStorage.saveDraft(row, null),
    pending,
  );
  for (let i = 0; i < 7; i++)
    await page.evaluate(
      (row) => window.recoveryStorage.saveDraft(row, null),
      saved(`folder-${i}`),
    );
  const other = await context.newPage();
  await openStorage(other);
  const restored = await other.evaluate(() =>
    window.recoveryStorage.loadDraft("pending-folder"),
  );
  expect(restored?.draft?.pending).toEqual(pending.draft?.pending);
  const errors = await other.evaluate(async (row) => {
    const failures: string[] = [];
    try {
      await window.recoveryStorage.saveDraft(
        { ...row, draft: null },
        row.revision,
      );
    } catch (error) {
      failures.push((error as Error).name);
    }
    try {
      await window.recoveryStorage.deleteDraft(row.key, row.revision);
    } catch (error) {
      failures.push((error as Error).name);
    }
    return failures;
  }, restored!);
  expect(errors).toEqual(["PendingCommandError", "PendingCommandError"]);
  await page.evaluate(
    (row) =>
      window.recoveryStorage.deleteDraft(
        row.key,
        row.revision,
        row.draft!.pending!.command_id,
      ),
    restored!,
  );
  expect(
    await page.evaluate(() =>
      window.recoveryStorage.loadDraft("pending-folder"),
    ),
  ).toBeNull();
});
