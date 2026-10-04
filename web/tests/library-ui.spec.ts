import { test, expect, type Page } from "@playwright/test";
import { initialConfig } from "../src/fields";
import { defaultPreferences } from "../src/library/preferences";
const key = "knowledge:library-preferences:library-ui";
const description =
  "Goal:\nExplain the project.\n\nMaterials:\nhttps://example.com/guide\nhttp://example.com/practice\n<script>window.libraryXss=true</script>\n<img src=x onerror=window.libraryXss=true>\n[bad](javascript:alert(1))";
function fixtures(count = 3) {
  return Array.from({ length: count }, (_, i) => ({
    id: `f${i}`,
    title: `Collection ${String(i).padStart(2, "0")}`,
    description,
    template_key: "interview_questions",
    config: initialConfig("interview_questions"),
    config_version: 1,
    training_config: {
      default_algorithm_key: "interview_long_term",
      pool_size: 5,
    },
    training_config_version: 1,
    material_count: i + 1,
    due_count: 0,
    learning_count: 0,
    completed_count: 0,
    topics: [],
    selected_plan: null,
    created_at: `2026-01-${String((i % 28) + 1).padStart(2, "0")}T00:00:00Z`,
    updated_at: "2026-01-01T00:00:00Z",
  }));
}
const material = {
  id: "question",
  folder_id: "f0",
  values: {
    question: "What is atomicity?",
    answer: "All changes commit together.",
    short_answer: "All or nothing.",
  },
  metadata: { topic: "Transactions" },
  difficulty: "hard",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  progress: null,
};
async function mockLibrary(page: Page, count = 3, expired = false) {
  const folders = fixtures(count);
  let refreshes = 0,
    libraryCalls = 0;
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/auth/browser/refresh")) {
      refreshes++;
      return route.fulfill({
        json: {
          access_token: "fixture",
          user: { id: "library-ui", nickname: "Library UI", status: "active" },
        },
      });
    }
    if (path.endsWith("/library")) {
      libraryCalls++;
      if (expired && libraryCalls === 1)
        return route.fulfill({ status: 401, json: { error: "unauthorized" } });
      return route.fulfill({
        json: {
          folders,
          totals: {
            folder_count: folders.length,
            material_count: count,
            due_count: 0,
            learning_count: 0,
            completed_count: 0,
          },
        },
      });
    }
    if (path.endsWith("/materials/question"))
      return route.fulfill({ json: material });
    if (path.endsWith("/materials"))
      return route.fulfill({
        json: {
          items: [material],
          total: 1,
          limit: 30,
          offset: 0,
          topics: [],
          plans: [],
          selected_plan: null,
        },
      });
    return route.fulfill({
      status: 404,
      json: { error: "fixture_route_missing" },
    });
  });
  return { folders, refreshes: () => refreshes };
}
async function preferences(page: Page) {
  return page.evaluate((key) => JSON.parse(localStorage.getItem(key)!), key);
}
async function noOverflow(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
}
for (const width of [360, 390, 430, 1280]) {
  test(`Library organization and dialogs at ${width}px`, async ({
    page,
  }, info) => {
    await page.setViewportSize({ width, height: 900 });
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await mockLibrary(page, 50);
    await page.goto("/");
    await expect(page.locator(".folder-card")).toHaveCount(50);
    await noOverflow(page);
    await page
      .getByRole("button", { name: "New category", exact: true })
      .click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Category name").fill("Interview practice");
    await dialog
      .getByRole("button", { name: "Save category", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Customize Collection 00", exact: true })
      .click();
    await dialog
      .getByLabel("Folder category")
      .selectOption({ label: "Interview practice" });
    await dialog
      .getByRole("button", { name: "Color blue", exact: true })
      .click();
    await dialog
      .getByRole("button", { name: "Icon brain", exact: true })
      .click();
    await dialog.getByLabel("Favorite folder").check();
    await noOverflow(page);
    await page.screenshot({ path: info.outputPath(`customize-${width}.png`) });
    await dialog.getByRole("button", { name: "Save appearance" }).click();
    await page.reload();
    const card = page.locator(".folder-card").filter({
      has: page.getByRole("link", { name: "Collection 00", exact: true }),
    });
    await expect(card).toHaveClass(/folder-color-blue/);
    await expect(
      card.getByRole("button", {
        name: "Unfavorite Collection 00",
        exact: true,
      }),
    ).toHaveAttribute("aria-pressed", "true");
    expect((await preferences(page)).folders.f0.icon).toBe("brain");
    await page.getByLabel("Filter folders").fill("INTERVIEW PRACTICE");
    await expect(page.locator(".folder-card")).toHaveCount(1);
    const category = page.getByRole("button", { name: /^Interview practice/ });
    await category.click();
    await expect(category).toHaveAttribute("aria-expanded", "false");
    await page.reload();
    await expect(category).toHaveAttribute("aria-expanded", "false");
    await category.click();
    await page
      .getByRole("button", { name: "Edit category Interview practice" })
      .click();
    await dialog.getByLabel("Category name").fill("Renamed");
    await dialog.getByRole("button", { name: "Save category" }).click();
    await page.getByRole("button", { name: "Edit category Renamed" }).click();
    await dialog
      .getByRole("button", { name: "Delete category", exact: true })
      .click();
    await dialog
      .getByRole("button", { name: "Confirm delete category" })
      .click();
    await expect(page.locator(".folder-card")).toHaveCount(50);
    await page
      .getByRole("button", { name: "Unfavorite Collection 00", exact: true })
      .click();
    expect((await preferences(page)).folders.f0.favorite).toBe(false);
    await page
      .getByRole("link", { name: "Collection 00", exact: true })
      .click();
    const desc = page.locator(".folder-page-heading .folder-description-text");
    await expect(
      desc.getByRole("link", {
        name: "https://example.com/guide",
        exact: true,
      }),
    ).toHaveAttribute("target", "_blank");
    await expect(
      desc.getByRole("link", {
        name: "http://example.com/practice",
        exact: true,
      }),
    ).toHaveAttribute("rel", "noopener noreferrer");
    await expect(desc).toContainText("Goal:\nExplain the project.");
    await expect(desc.locator("p").first()).toHaveCSS(
      "white-space",
      "pre-wrap",
    );
    expect(await page.evaluate(() => "libraryXss" in window)).toBe(false);
    await expect(desc.locator('a[href^="javascript:"],script,img')).toHaveCount(
      0,
    );
    await page.getByRole("button", { name: /What is atomicity/ }).click();
    await dialog
      .getByRole("button", { name: "Copy question", exact: true })
      .click();
    await dialog.getByLabel("Search copy folders").fill("collection 01");
    await dialog.locator(".copy-target").click();
    await noOverflow(page);
    await expect(
      dialog.getByRole("button", { name: "Copy question", exact: true }),
    ).toBeInViewport();
    await page.keyboard.press("Escape");
    await expect(dialog).not.toBeVisible();
    await expect(
      page.getByRole("button", { name: /What is atomicity/ }),
    ).toBeFocused();
    await page.screenshot({ path: info.outputPath(`folder-${width}.png`) });
    expect(errors).toEqual([]);
  });
}

test("Library recently used, folder navigation, Back scroll and auth refresh retry", async ({
  page,
}) => {
  const fixture = await mockLibrary(page, 24, true);
  await page.goto("/");
  await expect(page.locator(".folder-card")).toHaveCount(24);
  expect(fixture.refreshes()).toBe(2);
  expect((await preferences(page)).folders).toEqual({});
  await page.getByLabel("Favorites first").uncheck();
  await page
    .getByRole("link", { name: "Collection 00", exact: true })
    .scrollIntoViewIfNeeded();
  const before = await page.evaluate(() => scrollY);
  expect(before).toBeGreaterThan(100);
  await page.getByRole("link", { name: "Collection 00", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Collection 00", exact: true }),
  ).toBeVisible();
  expect(await page.evaluate(() => scrollY)).toBe(0);
  await page.goBack();
  await expect(page.locator(".folder-card").first()).toContainText(
    "Collection 00",
  );
  await expect.poll(() => page.evaluate(() => scrollY)).toBeGreaterThan(100);
  await page.getByRole("link", { name: "Collection 01", exact: true }).click();
  expect(await page.evaluate(() => scrollY)).toBe(0);
  await page.goBack();
  await expect(page.locator(".folder-card").first()).toContainText(
    "Collection 01",
  );
});

test("Copy dialog default, one-off target, conflict, explicit duplicate and mutation guard", async ({
  page,
}) => {
  await mockLibrary(page);
  await page.addInitScript(
    ({ key, p }) => localStorage.setItem(key, JSON.stringify(p)),
    { key, p: { ...defaultPreferences(), defaultCopyFolderId: "f1" } },
  );
  const bodies: unknown[] = [];
  await page.route("**/interview/questions/question/copy", async (route) => {
    const body = route.request().postDataJSON();
    bodies.push(body);
    if (!body.allow_duplicate)
      return route.fulfill({
        status: 409,
        json: {
          error: "duplicate_question",
          existing_material_id: "question",
          folder_id: body.target_folder_id,
        },
      });
    await new Promise((resolve) => setTimeout(resolve, 200));
    return route.fulfill({
      status: 201,
      json: { material_id: "copy", folder_id: body.target_folder_id },
    });
  });
  await page.goto("/folders/f0");
  const dialog = page.getByRole("dialog");
  async function open() {
    await page.getByRole("button", { name: /What is atomicity/ }).click();
    await dialog
      .getByRole("button", { name: "Copy question", exact: true })
      .click();
  }
  await open();
  await expect(dialog.getByRole("status")).toHaveText(
    "Selected: Collection 01",
  );
  expect(bodies).toHaveLength(0);
  await dialog
    .locator(".copy-target")
    .filter({ hasText: "Collection 02" })
    .click();
  await dialog
    .getByRole("button", { name: "Copy question", exact: true })
    .click();
  await expect(dialog.getByRole("alert")).toContainText("already exists");
  await dialog.getByRole("button", { name: "Create another copy" }).dblclick();
  await expect(dialog).not.toBeVisible();
  expect(bodies).toEqual([
    { target_folder_id: "f2", allow_duplicate: false },
    { target_folder_id: "f2", allow_duplicate: true },
  ]);
  expect((await preferences(page)).defaultCopyFolderId).toBe("f1");
  await open();
  await dialog
    .getByRole("button", { name: "Copy question", exact: true })
    .click();
  await dialog.getByRole("button", { name: "Open existing" }).click();
  await expect(page).toHaveURL(/folders\/f1\?material=question/);
  await expect(
    dialog.getByRole("heading", { name: "Material details", exact: true }),
  ).toBeVisible();
});

test("Corrupt preferences recover; copy error re-enables submit", async ({
  page,
}) => {
  await mockLibrary(page);
  await page.addInitScript((key) => localStorage.setItem(key, "{broken"), key);
  await page.goto("/");
  await expect(page.locator(".folder-card")).toHaveCount(3);
  expect((await preferences(page)).defaultCopyFolderId).toBeNull();
  await page.route("**/interview/questions/question/copy", (route) =>
    route.fulfill({ status: 500, json: { error: "internal_error" } }),
  );
  await page.getByRole("link", { name: "Collection 00", exact: true }).click();
  await page.getByRole("button", { name: /What is atomicity/ }).click();
  const dialog = page.getByRole("dialog");
  await dialog
    .getByRole("button", { name: "Copy question", exact: true })
    .click();
  await dialog.locator(".copy-target").first().click();
  await dialog
    .getByRole("button", { name: "Copy question", exact: true })
    .click();
  await expect(dialog.getByRole("alert")).toContainText("Something went wrong");
  await expect(
    dialog.getByRole("button", { name: "Copy question", exact: true }),
  ).toBeEnabled();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(dialog).not.toBeVisible();
});

test("Deleted default target is pruned before opening Copy", async ({
  page,
}) => {
  await mockLibrary(page);
  await page.addInitScript(
    ({ key, p }) => localStorage.setItem(key, JSON.stringify(p)),
    {
      key,
      p: {
        ...defaultPreferences(),
        defaultCopyFolderId: "deleted",
        recentCopyTargets: ["deleted", "f1"],
      },
    },
  );
  await page.goto("/folders/f0");
  await page.getByRole("button", { name: /What is atomicity/ }).click();
  const dialog = page.getByRole("dialog");
  await dialog
    .getByRole("button", { name: "Copy question", exact: true })
    .click();
  await expect(dialog.getByRole("status")).toHaveText(
    "Choose a target folder.",
  );
  expect((await preferences(page)).defaultCopyFolderId).toBeNull();
  expect((await preferences(page)).recentCopyTargets).toEqual(["f1"]);
});
