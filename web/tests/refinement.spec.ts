import { test, expect } from "@playwright/test";
import { initialConfig } from "../src/fields";

test("refined training: direct answers, masked examples, tooltip and Moscow time", async ({
  page,
}) => {
  let answers = 0;
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.addInitScript(() =>
    localStorage.setItem(
      "knowledge:training:preview",
      JSON.stringify({
        sources: [{ folder_id: "folder" }],
        session_ids: ["session"],
      }),
    ),
  );
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    let body: unknown;
    if (path.endsWith("/auth/browser/refresh"))
      body = {
        access_token: "preview",
        user: { id: "preview", nickname: "Preview", status: "active" },
      };
    else if (path.endsWith("/library"))
      body = {
        folders: [],
        totals: {
          folder_count: 0,
          material_count: 1,
          due_count: 1,
          learning_count: 1,
          completed_count: 0,
        },
      };
    else if (path.endsWith("/actions")) {
      answers++;
      body = {};
    } else if (path.endsWith("/combined/current"))
      body = {
        sessions: [{ session: { id: "session" } }],
        summary: { correct: answers, wrong: 0, materials_reviewed: 1 },
        current:
          answers < 2
            ? {
                session_id: "session",
                algorithm_key: "english_basic",
                presentation: {
                  id: `card-${answers}`,
                  material_id: "material",
                  folder_id: "folder",
                  kind: "stage",
                  stage: 2,
                  progress_version: answers + 1,
                  direction: answers ? "native" : "foreign",
                  question: [
                    {
                      key: answers ? "native" : "foreign",
                      value: answers ? "случайная удача" : "serendipity",
                    },
                  ],
                  answer: [
                    {
                      key: "native",
                      label: "Translation",
                      value: "случайная удача",
                    },
                  ],
                  example: "It was serendipity. SERENDIPITY happens.",
                  foreign_word: "serendipity",
                  required_correct: 3,
                  consecutive_correct: answers,
                },
              }
            : null,
        next_review_at: new Date(Date.now() + 3600000).toISOString(),
      };
    else if (path.endsWith("/statistics")) {
      expect(new URL(route.request().url()).searchParams.get("timezone")).toBe(
        "Europe/Moscow",
      );
      const activity = {
        answers: 12,
        correct: 9,
        wrong: 3,
        materials_reviewed: 5,
        stage_promotions: 2,
        active_days: 1,
      };
      body = {
        totals: activity,
        daily: Array.from({ length: 7 }, (_, i) => ({
          date: `2026-09-${String(i + 4).padStart(2, "0")}`,
          ...activity,
          answers: i === 6 ? 12 : 0,
          correct: i === 6 ? 9 : 0,
          wrong: i === 6 ? 3 : 0,
        })),
        timezone: "Europe/Moscow",
      };
    } else
      return route.fulfill({
        status: 404,
        json: { error: "Unexpected preview route" },
      });
    return route.fulfill({ json: body });
  });
  await page.goto("/train");
  await expect(
    page.getByRole("button", { name: "Correct", exact: true }),
  ).toBeEnabled();
  await expect(page.locator(".answer-fields")).toHaveAttribute(
    "aria-hidden",
    "true",
  );
  const frontBounds = await page.locator(".recall-card").boundingBox();
  await page.getByRole("button", { name: "Flip to answer" }).click();
  await expect(
    page.getByRole("button", { name: "Flip to question" }),
  ).toBeVisible();
  expect((await page.locator(".recall-card").boundingBox())?.height).toBe(
    frontBounds?.height,
  );
  await page.screenshot({
    path: "test-results/flipped-answer.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "Flip to question" }).click();
  await page.getByRole("button", { name: "Show example", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Reveal hidden word" }),
  ).toHaveCount(2);
  await expect(page.locator(".example-content")).not.toContainText(
    /serendipity/i,
  );
  await page
    .getByRole("button", { name: "Reveal hidden word" })
    .first()
    .click();
  await expect(page.locator(".example-content")).toContainText("serendipity");
  await page.screenshot({
    path: "test-results/refined-training-desktop.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "Correct", exact: true }).click();
  await expect(page.locator(".lead-question")).toContainText("случайная удача");
  await expect(
    page.getByRole("button", { name: "Show example", exact: true }),
  ).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({
    path: "test-results/refined-training-mobile.png",
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.getByRole("button", { name: "Correct", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Review early" }),
  ).toBeVisible();
  await expect(page.locator(".next-review-note")).toContainText("MSK");
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/statistics");
  await page.locator(".bar-column").last().hover();
  expect(
    (await page.locator(".chart-bar").last().boundingBox())!.width,
  ).toBeLessThanOrEqual(28);
  const panel = (await page.locator(".chart-panel").boundingBox())!;
  const details = (await page.getByRole("tooltip").boundingBox())!;
  expect(details.y).toBeGreaterThan(panel.y);
  await expect(page.getByRole("tooltip")).toContainText("Wrong");
  await expect(page.getByRole("tooltip")).toContainText("12");
  await page.screenshot({
    path: "test-results/refined-statistics.png",
    fullPage: true,
  });
  await page.getByRole("combobox", { name: "Statistics period" }).click();
  await page.screenshot({
    path: "test-results/refined-select.png",
    fullPage: true,
  });
  await page.keyboard.press("Escape");
  expect(errors).toEqual([]);
});

test("folder fields control the material editor", async ({ page }) => {
  let created = false;
  const folder = {
    id: "folder",
    title: "Custom vocabulary",
    description: "",
    template_key: "english_words",
    config: initialConfig("english_words"),
    config_version: 1,
    training_config: { default_algorithm_key: "english_basic", pool_size: 7 },
    training_config_version: 1,
    topics: [],
    material_count: 0,
    due_count: 0,
    learning_count: 0,
    completed_count: 0,
    selected_plan: null,
    created_at: new Date().toISOString(),
  };
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    let body: unknown;
    if (path.endsWith("/auth/browser/refresh"))
      body = {
        access_token: "preview",
        user: { id: "preview", nickname: "Preview", status: "active" },
      };
    else if (path.endsWith("/library"))
      body = {
        folders: created ? [folder] : [],
        totals: {
          folder_count: created ? 1 : 0,
          material_count: 0,
          due_count: 0,
          learning_count: 0,
          completed_count: 0,
        },
      };
    else if (path.endsWith("/folders")) {
      created = true;
      body = folder;
    } else if (path.endsWith("/workshop")) {
      folder.config = route.request().postDataJSON().config;
      folder.config_version++;
      body = folder;
    } else if (path.endsWith("/materials"))
      body = {
        items: [],
        total: 0,
        limit: 50,
        offset: 0,
        topics: [],
        selected_plan: null,
        plans: [],
      };
    else return route.fulfill({ status: 404, json: {} });
    return route.fulfill({ json: body });
  });
  await page.goto("/");
  await page
    .getByRole("button", { name: "Create folder", exact: true })
    .click();
  await page.getByLabel("Folder name", { exact: true }).fill(folder.title);
  await page
    .getByRole("checkbox", { name: "Enable Transcription", exact: true })
    .uncheck();
  await page
    .getByRole("checkbox", { name: "Enable Topic", exact: true })
    .uncheck();
  await page.getByRole("button", { name: "Add content field" }).click();
  await page
    .getByRole("textbox", { name: /^Label for custom_/ })
    .fill("Memory cue");
  await page.screenshot({
    path: "test-results/refined-folder-fields.png",
    fullPage: true,
  });
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Create folder", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("link", { name: folder.title, exact: true }).click();
  await page.getByRole("button", { name: "Add material", exact: true }).click();
  await expect(page.getByLabel("Memory cue", { exact: false })).toBeVisible();
  await expect(page.getByLabel("Transcription", { exact: false })).toHaveCount(
    0,
  );
  await expect(page.getByLabel("Topic", { exact: true })).toHaveCount(0);
  await expect(page.getByLabel("Foreign", { exact: true })).toBeVisible();
});
