import { test, expect, type Page } from "@playwright/test";
import { fixture } from "./quick-import-fixture";
async function parse(page: Page, text: string) {
  await page.getByLabel("Import source").fill(text);
  await page
    .getByRole("button", { name: "Review import", exact: true })
    .click();
  await expect(page.getByLabel("Review filter")).toBeVisible();
}
async function save(page: Page) {
  await page
    .getByRole("button", { name: /^Import \d+( ready)? cards?$/ })
    .click();
  await page
    .getByRole("button", { name: "Confirm import", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Import complete" }),
  ).toBeVisible();
}
test("text review preserves examples, unknown fragment, source navigation and refresh", async ({
  page,
}) => {
  const f = await fixture(page);
  await parse(
    page,
    "subtle - тонкий - сатл\nExample: A subtle change.\nExample: Subtle choices matter.",
  );
  await expect(
    page.getByRole("button", { name: /^Import \d+( ready)? cards?$/ }),
  ).toBeDisabled();
  const issue = page
    .getByTestId("import-fragment")
    .filter({ hasText: "unsupported_fragment" });
  await issue.getByRole("button", { name: "Go to source" }).click();
  expect(
    await page
      .getByLabel("Import source")
      .evaluate((e: HTMLTextAreaElement) =>
        e.value.slice(e.selectionStart, e.selectionEnd),
      ),
  ).toBe("сатл");
  await page.getByRole("button", { name: "Back to review" }).click();
  await issue.getByRole("button", { name: "Exclude fragment" }).click();
  await page.getByLabel("native", { exact: true }).fill("едва заметный");
  await page.locator(".dialog-content").evaluate((e) => {
    e.scrollTop = 0;
  });
  await page.screenshot({
    path: ".cache/quick-import-desktop.png",
    fullPage: true,
  });
  await save(page);
  expect(f.commands[0].items[0].values).toMatchObject({
    foreign: "subtle",
    native: "едва заметный",
    example: "A subtle change.\nSubtle choices matter.",
  });
  expect(f.commands[0].items[0].values.transcription).toBeUndefined();
  expect(f.folderWrites).toBe(0);
  await page
    .getByRole("button", { name: "Back to folder", exact: true })
    .click();
  await expect(
    page.getByRole("checkbox", { name: "Select subtle", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Import", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Review import" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Restore draft" })).toHaveCount(
    0,
  );
});
test("valid-only requires explicit excluded-fragment confirmation", async ({
  page,
}) => {
  const f = await fixture(page);
  await parse(page, "run - бегать\nbroken -");
  await expect(
    page.getByRole("button", { name: /^Import \d+( ready)? cards?$/ }),
  ).toBeEnabled();
  await page.getByRole("button", { name: /^Import \d+ ready cards?$/ }).click();
  await expect(page.getByLabel("Confirm import")).toContainText(
    "1 unresolved fragments excluded",
  );
  expect(f.commands).toHaveLength(0);
  await page
    .getByRole("button", { name: "Confirm import", exact: true })
    .click();
  await expect(
    page.getByText("Import complete", { exact: true }),
  ).toBeVisible();
  expect(f.commands[0].items).toHaveLength(1);
});
test("JSON aliases use same editor and reject duplicate keys and invalid encoding", async ({
  page,
}) => {
  const f = await fixture(page);
  await page.getByRole("button", { name: "Upload JSON", exact: true }).click();
  await page.locator("input[type=file]").setInputFiles({
    name: "bad.json",
    mimeType: "application/json",
    buffer: Buffer.from([0xff]),
  });
  await expect(page.getByRole("alert")).toBeVisible();
  await page.locator("input[type=file]").setInputFiles({
    name: "words.json",
    mimeType: "application/json",
    buffer: Buffer.from(
      JSON.stringify({
        format_version: 1,
        folder: { name: "Must not create", template: "english_words" },
        items: [{ word: "apple", translation: "яблоко", pronunciation: "эпл" }],
      }),
    ),
  });
  await page
    .getByRole("button", { name: "Review import", exact: true })
    .click();
  await expect(page.getByLabel("foreign", { exact: true })).toHaveValue(
    "apple",
  );
  await page
    .getByTestId("import-fragment")
    .getByRole("button", { name: "Exclude fragment" })
    .click();
  await save(page);
  expect(f.folderWrites).toBe(0);
});
test("uncertain commit freezes edits, restores and retries identical command", async ({
  page,
}) => {
  const f = await fixture(page, "english_words", true);
  await parse(page, "apple - яблоко");
  await page
    .getByRole("button", { name: /^Import \d+( ready)? cards?$/ })
    .click();
  await page
    .getByRole("button", { name: "Confirm import", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Retry same import" }),
  ).toBeEnabled();
  await expect(page.getByLabel("native", { exact: true })).toBeDisabled();
  expect(f.materials).toHaveLength(2);
  await page.reload();
  await page.getByRole("button", { name: "Import", exact: true }).click();
  await page.getByRole("button", { name: "Restore draft" }).click();
  await page.getByRole("button", { name: "Retry same import" }).click();
  await expect(
    page.getByText("Import complete", { exact: true }),
  ).toBeVisible();
  expect(f.commands).toHaveLength(2);
  expect(f.commands[1]).toEqual(f.commands[0]);
  expect(f.materials).toHaveLength(2);
});
test("edit and revalidate preserve manual values; reparse asks before replacing", async ({
  page,
}) => {
  await fixture(page);
  await parse(page, "run - бегать");
  await page.getByLabel("native", { exact: true }).fill("управлять");
  await expect(
    page.getByRole("button", { name: "Revalidate", exact: true }),
  ).toHaveCount(0);
  await expect(page.getByLabel("native", { exact: true })).toHaveValue(
    "управлять",
  );
  await page.getByRole("button", { name: "Edit source" }).click();
  await page.getByLabel("Import source").fill("run - бежать");
  page.once("dialog", (d) => d.dismiss());
  await page.getByRole("button", { name: "Reparse source" }).click();
  await page.getByRole("button", { name: "Back to review" }).click();
  await expect(page.getByLabel("native", { exact: true })).toHaveValue(
    "управлять",
  );
});
test("move, copy, merge and split preserve edits and explicit example order", async ({
  page,
}) => {
  await fixture(page);
  await parse(
    page,
    "apple - яблоко\nExample: Apple one.\nExample: Apple two.\nbook - книга",
  );
  await page.getByText("Advanced", { exact: true }).click();
  await page.getByLabel("Target card").selectOption({ label: "book" });
  await page.getByRole("button", { name: "Copy to target" }).first().click();
  await page.getByRole("button", { name: "Move to target" }).last().click();
  await page.getByRole("button", { name: "Merge into target" }).click();
  await expect(page.getByLabel("native", { exact: true })).toHaveValue(
    "книга\nяблоко",
  );
  await expect(page.getByLabel("Example 1", { exact: true })).toHaveValue(
    "Apple one.",
  );
  await page.getByText("Advanced", { exact: true }).click();
  await page.getByRole("button", { name: "Split card", exact: true }).click();
  await expect(page.getByLabel("foreign", { exact: true })).toHaveValue("");
  await page.getByLabel("foreign", { exact: true }).fill("pear");
  await page.getByLabel("native", { exact: true }).fill("груша");
  await expect(
    page.getByRole("button", { name: "Revalidate", exact: true }),
  ).toHaveCount(0);
  await expect(page.getByLabel("native", { exact: true })).toHaveValue("груша");
});
test("existing duplicates are shown and explicit allow overrides skip", async ({
  page,
}) => {
  const f = await fixture(page);
  await parse(page, "existing - существующий");
  await page
    .getByRole("button", { name: /^Import \d+( ready)? cards?$/ })
    .click();
  await expect(page.getByLabel("Confirm import")).toContainText(
    "0 will be created · 1 duplicates skipped",
  );
  await page.getByText("Advanced", { exact: true }).click();
  await page.getByLabel("Duplicate decision").selectOption("allow");
  await save(page);
  expect(f.commands[0].items[0].duplicate_policy).toBe("allow");
  expect(f.materials).toHaveLength(2);
});
test("Escape keeps local draft and reopening offers restoration", async ({
  page,
}) => {
  await fixture(page);
  await parse(page, "book - книга");
  await page.getByLabel("native", { exact: true }).fill("учебник");
  page.once("dialog", (d) => d.accept());
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: "Import", exact: true }).click();
  await page.getByRole("button", { name: "Restore draft" }).click();
  await expect(page.getByLabel("native", { exact: true })).toHaveValue(
    "учебник",
  );
});
test("interview text imports schema fields and preserves missing answers", async ({
  page,
}) => {
  const f = await fixture(page, "interview_questions");
  await parse(
    page,
    "Question: What is a goroutine?\nShort answer: A concurrent function.\nSources: https://go.dev\n\nQuestion: What is a channel?",
  );
  await expect(page.getByLabel("question", { exact: true })).toHaveValue(
    "What is a goroutine?",
  );
  await save(page);
  expect(f.commands[0].items[0].values).toMatchObject({
    question: "What is a goroutine?",
    short_answer: "A concurrent function.",
    sources: "https://go.dev",
  });
  expect(f.commands[0].items[1].values.short_answer).toBeUndefined();
});
test("mobile 500 cards paginate and navigate exactly to the last source fragment", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await fixture(page);
  const source = Array.from(
    { length: 500 },
    (_, i) => `entry${i} - запись${i}${i === 499 ? " - сатл" : ""}`,
  ).join("\n");
  const started = Date.now();
  await parse(page, source);
  expect(Date.now() - started).toBeLessThan(10000);
  await expect(page.locator(".qi-list li")).toHaveCount(25);
  await page.getByRole("button", { name: "Issues 1", exact: true }).click();
  await page
    .locator(".qi-list li button")
    .filter({ hasText: "entry499" })
    .click();
  await expect(page.getByLabel("foreign", { exact: true })).toBeVisible();
  await page.screenshot({
    path: ".cache/quick-import-mobile.png",
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page
    .getByTestId("import-fragment")
    .getByRole("button", { name: "Go to source" })
    .click();
  const selected = await page
    .getByLabel("Import source")
    .evaluate((e: HTMLTextAreaElement) => ({
      text: e.value.slice(e.selectionStart, e.selectionEnd),
      scroll: e.scrollTop,
    }));
  expect(selected.text).toBe("сатл");
  expect(selected.scroll).toBeGreaterThan(5000);
});
