import { test, expect } from "@playwright/test";
import { fixture } from "./quick-import-fixture";

test("redesign offers working steps, filters, undo and a pinned action", async ({
  page,
}) => {
  await fixture(page);
  await page
    .getByLabel("Import source")
    .fill("subtle - тонкий - сатл\nbook - книга");
  await page
    .getByRole("button", { name: "Review import", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Undo", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Issues 1", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "1 Input", exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByRole("button", { name: "Revalidate", exact: true }),
  ).toHaveCount(0);
  await expect(page.getByText("Advanced", { exact: true })).toBeVisible();
  await expect(
    page.getByLabel("Card difficulty", { exact: true }),
  ).not.toBeVisible();
  await expect(page.getByTestId("import-footer")).toBeVisible();
});

async function review(page: import("@playwright/test").Page, source: string) {
  await page.getByLabel("Import source").fill(source);
  await page
    .getByRole("button", { name: "Review import", exact: true })
    .click();
  await expect(
    page.getByRole("group", { name: "Review filter" }),
  ).toBeVisible();
}
test("quick fix, grouped Undo/Redo, source and filters preserve reviewed edits", async ({
  page,
}) => {
  await fixture(page);
  const source = "subtle - тонкий - сатл\nbook - книга";
  await review(page, source);
  await page
    .getByTestId("import-fragment")
    .getByRole("button", { name: "Exclude fragment" })
    .click();
  await expect(
    page.getByRole("button", { name: "Issues 0", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Undo", exact: true }).click();
  await expect(page.getByTestId("import-fragment")).toBeVisible();
  await page.getByRole("button", { name: "Redo", exact: true }).click();
  await page.getByLabel("native", { exact: true }).fill("изящный");
  await page.getByRole("button", { name: "1 Input", exact: true }).click();
  await expect(page.getByLabel("Import source")).toHaveValue(source);
  await page.getByLabel("Import source").fill("changed - изменённый");
  await page.getByRole("button", { name: "2 Review", exact: true }).click();
  await expect(page.getByLabel("native", { exact: true })).toHaveValue(
    "изящный",
  );
  await page
    .getByRole("button", { name: "Go to source", exact: true })
    .first()
    .click();
  await expect(page.getByLabel("Import source")).toHaveValue(source);
  await page.getByRole("button", { name: "Return to edited source" }).click();
  await expect(page.getByLabel("Import source")).toHaveValue(
    "changed - изменённый",
  );
  await page.getByRole("button", { name: "Back to review" }).click();
  await page.getByRole("button", { name: "Issues 0", exact: true }).click();
  await expect(
    page.getByText("No cards need attention.", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "All 2", exact: true }).click();
  await expect(page.getByLabel("native", { exact: true })).toHaveValue(
    "изящный",
  );
});
test("server error maps to field; editing invalidates an in-flight preview", async ({
  page,
}) => {
  await fixture(page);
  await review(page, "apple - яблоко\nbook - книга");
  let release: (() => void) | undefined;
  let started = false;
  await page.route("**/materials/import/validate", async (route) => {
    const body = route.request().postDataJSON();
    started = true;
    await new Promise<void>((r) => {
      release = r;
    });
    await route.fulfill({
      json: {
        valid: false,
        created: 0,
        skipped: 0,
        failed: 1,
        items: [],
        issues: [
          {
            client_id: body.items[0].client_id,
            path: "items[0].values.native",
            code: "field_invalid",
            severity: "error",
            message: "Translation rejected",
          },
        ],
      },
    });
  });
  await page
    .getByRole("button", { name: "Import 2 cards", exact: true })
    .click();
  await expect.poll(() => started).toBe(true);
  await page.getByLabel("native", { exact: true }).fill("фрукт");
  release!();
  await expect(
    page.getByRole("button", { name: "Import 2 cards", exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByText("Translation rejected", { exact: true }),
  ).toHaveCount(0);
  await page.unroute("**/materials/import/validate");
  await page.route("**/materials/import/validate", async (route) => {
    const body = route.request().postDataJSON();
    await route.fulfill({
      json: {
        valid: false,
        created: 0,
        skipped: 0,
        failed: 1,
        items: [],
        issues: [
          {
            client_id: body.items[1].client_id,
            path: "items[1].values.native",
            code: "field_invalid",
            severity: "error",
            message: "Translation rejected",
          },
        ],
      },
    });
  });
  await page
    .getByRole("button", { name: "Import 2 cards", exact: true })
    .click();
  await page.getByRole("button", { name: "Issues 1", exact: true }).click();
  await expect(page.getByLabel("native", { exact: true })).toHaveAttribute(
    "aria-invalid",
    "true",
  );
  await expect(
    page.getByText("Translation rejected", { exact: true }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Server validation" }),
  ).toHaveCount(0);
});
for (const width of [320, 360, 390, 430, 768])
  test(`mobile full import and pinned action at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 844 });
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    const f = await fixture(page);
    await review(
      page,
      Array.from({ length: 14 }, (_, i) => `word${i} - слово${i}`).join("\n"),
    );
    await page
      .locator(".dialog-content")
      .evaluate((e) => (e.scrollTop = e.scrollHeight));
    const footer = await page.getByTestId("import-footer").boundingBox();
    expect(footer!.y + footer!.height).toBeLessThanOrEqual(844);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page
      .getByRole("button", { name: "Import 14 cards", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Confirm import", exact: true })
      .click();
    await expect(
      page.getByText("14 cards added", { exact: true }),
    ).toBeVisible();
    expect(f.commands).toHaveLength(1);
    expect(errors).toEqual([]);
  });
test("500 cards: problem navigation crosses pages and keeps mobile list selection", async ({
  page,
}) => {
  await page.setViewportSize({ width: 360, height: 800 });
  await fixture(page);
  await review(
    page,
    Array.from(
      { length: 500 },
      (_, i) => `word${i} - слово${i}${i === 0 || i === 499 ? " - сатл" : ""}`,
    ).join("\n"),
  );
  await page.getByRole("button", { name: "Next problem", exact: true }).click();
  await expect(page.getByLabel("foreign", { exact: true })).toHaveValue(
    "word0",
  );
  await page.getByRole("button", { name: "Next problem", exact: true }).click();
  await expect(page.getByLabel("foreign", { exact: true })).toHaveValue(
    "word499",
  );
  await page
    .getByTestId("import-fragment")
    .getByRole("button", { name: "Exclude fragment" })
    .click();
  await expect(
    page.getByRole("button", { name: "Issues 1", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Previous problem", exact: true })
    .click();
  await expect(page.getByLabel("foreign", { exact: true })).toHaveValue(
    "word0",
  );
  await page.setViewportSize({ width: 844, height: 390 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await expect(page.getByTestId("import-footer")).toBeVisible();
});

test("legacy Folder Import still validates and submits a file through its original endpoints", async ({
  page,
}) => {
  await fixture(page);
  await page.getByRole("button", { name: "Close dialog", exact: true }).click();
  await page.goto("/");
  const requests: string[] = [];
  await page.route("**/folders/import**", async (route) => {
    requests.push(new URL(route.request().url()).pathname);
    expect(route.request().headers()["content-type"]).toContain(
      "multipart/form-data",
    );
    await route.fulfill({
      json: route.request().url().endsWith("/validate")
        ? {
            valid: true,
            format_version: 1,
            template: "english_words",
            folder_name: "Legacy words",
            items_count: 1,
            warnings: [],
          }
        : {
            folder_id: "f2",
            folder_name: "Legacy words",
            template: "english_words",
            items_created: 1,
            warnings: [],
          },
    });
  });
  await page
    .getByRole("button", { name: "Import folder", exact: true })
    .click();
  await page
    .getByLabel("Choose JSON file")
    .setInputFiles({
      name: "words.json",
      mimeType: "application/json",
      buffer: Buffer.from(
        JSON.stringify({
          format_version: 1,
          folder: { name: "Legacy words", template: "english_words" },
          items: [{ word: "apple", translation: "яблоко" }],
        }),
      ),
    });
  await expect(page.getByLabel("Import preview")).toContainText("Legacy words");
  await page
    .getByRole("button", { name: "Import folder", exact: true })
    .last()
    .click();
  await expect(
    page.getByRole("dialog", { name: "Import folder", exact: true }),
  ).toHaveCount(0);
  expect(requests).toEqual([
    "/api/v1/folders/import/validate",
    "/api/v1/folders/import",
  ]);
});
