import { test, expect, type Page } from "@playwright/test";
import { initialConfig } from "../src/fields";

const folders = Array.from({ length: 24 }, (_, i) => ({
  id: `folder-${i}`,
  title: `Collection ${i}`,
  description: "Interview questions for responsive regression checks.",
  template_key: "interview_questions",
  config: initialConfig("interview_questions"),
  config_version: 1,
  training_config_version: 1,
  training_config: {
    default_algorithm_key: "interview_long_term",
    pool_size: 7,
  },
  topics: [],
  material_count: 30,
  due_count: 30,
  learning_count: 0,
  completed_count: 0,
  selected_plan: null,
}));
const answers = [
  "A goroutine is a lightweight thread.",
  "An index lets a database locate matching rows without scanning every row. It improves reads, while writes must also update the index.",
  "A transaction groups operations into a single unit. Isolation controls what concurrent transactions observe. ".repeat(
    35,
  ),
  "Индекс помогает находить строки без полного сканирования таблицы. При изменении данных индекс также обновляется. ".repeat(
    8,
  ),
  "Use context cancellation to stop concurrent work and release resources. Always handle errors at the appropriate boundary. ".repeat(
    8,
  ),
  "Горутина использует context.Context для cancellation; defer освобождает resources, а channel передаёт результат. ".repeat(
    8,
  ),
  "VeryLongUnbrokenTechnicalIdentifier".repeat(14),
  'Вызов `repository.FindByContextAndTransactionIdentifier` возвращает result.\n\n```go\nresult, err := repository.FindByContextAndTransactionIdentifier(ctx, transactionIdentifier)\nif err != nil { return fmt.Errorf("find transaction: %w", err) }\n```',
];

const questionAt = (i: number) =>
  i === 2
    ? `Question 2: ${"Explain how transaction boundaries interact with concurrent requests. ".repeat(10)}`
    : `Question ${i}`;

async function fixture(page: Page) {
  const state = { index: 0, actions: [] as string[], edits: 0 };
  await page.addInitScript(() =>
    localStorage.setItem(
      "knowledge:training:preview",
      JSON.stringify({
        sources: [{ folder_id: "folder-0" }],
        session_ids: ["session"],
      }),
    ),
  );
  const current = () => ({
    sessions: [{ session: { id: "session" } }],
    summary: {
      correct: state.actions.filter((a) => a === "correct").length,
      wrong: state.actions.filter((a) => a === "wrong").length,
      materials_reviewed: state.index,
    },
    undo_actions: state.actions.length ? ["last-event"] : [],
    current: {
      session_id: "session",
      algorithm_key: "interview_long_term",
      presentation: {
        id: `card-${state.index}`,
        material_id: "material",
        folder_id: "folder-0",
        kind: "stage",
        stage: 1,
        progress_version: state.index + 1,
        question: [{ key: "question", value: questionAt(state.index) }],
        // Both legacy answer-only and bank short/detailed presentations.
        answer: [
          {
            key: state.index % 2 ? "short_answer" : "answer",
            value: answers[state.index % answers.length],
          },
          ...(state.index % 2
            ? [
                {
                  key: "answer",
                  label: "Answer",
                  value: "Detailed explanation. Подробный ответ.",
                },
              ]
            : []),
        ],
        required_correct: 3,
        consecutive_correct: 0,
      },
    },
  });
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
        folders,
        totals: {
          folder_count: folders.length,
          material_count: 720,
          due_count: 720,
          learning_count: 0,
          completed_count: 0,
        },
      };
    else if (path.endsWith("/profile"))
      body = {
        frequency: 5,
        frequency_confidence: 0.5,
        interview_difficulty: 2,
        specificity: 2,
        root_weight: 5,
        followup_weight: 5,
        level_min: 1,
        level_max: 5,
        interview_profiles: [],
        status: "draft",
        profile_version: 0,
        domain: "",
        concepts: [],
      };
    else if (path.endsWith("/actions")) {
      const action = route.request().postDataJSON();
      expect(action.presentation_id).toBe(`card-${state.index}`);
      state.actions.push(action.action);
      state.index++;
      body = {};
    } else if (path.endsWith("/combined/undo")) {
      state.index--;
      state.actions.pop();
      body = current();
    } else if (path.endsWith("/combined/current")) body = current();
    else if (path.endsWith("/materials/material")) {
      if (route.request().method() === "PATCH") state.edits++;
      body = {
        id: "material",
        folder_id: "folder-0",
        difficulty: "medium",
        values: { question: "Question", answer: answers[0] },
        metadata: {},
        progress: null,
      };
    } else if (path.endsWith("/materials")) {
      // Let the loading layout paint before the full collection arrives.
      await new Promise((resolve) => setTimeout(resolve, 180));
      body = {
        items: Array.from({ length: 30 }, (_, i) => ({
          id: `material-${i}`,
          folder_id: path.split("/").at(-2),
          difficulty: "medium",
          values: { question: `Material ${i}`, answer: answers[0] },
          metadata: {},
          progress: null,
        })),
        total: 30,
        limit: 30,
        offset: 0,
        topics: [],
        plans: [],
        selected_plan: null,
      };
    } else
      return route.fulfill({
        status: 404,
        json: { error: `Unexpected route: ${path}` },
      });
    await route.fulfill({ json: body });
  });
  return state;
}

for (const width of [320, 375, 390, 430, 768, 1440, 1920]) {
  test(`collection entry starts at top after a deep Library click (${width}px)`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
    await fixture(page);
    await page.goto("/");
    for (const [iteration, index] of [20, 22, 21].entries()) {
      const open = page
        .locator(".folder-card")
        .filter({
          has: page.getByRole("link", {
            name: `Collection ${index}`,
            exact: true,
          }),
        })
        .getByRole("link", { name: "Open collection" });
      await open.scrollIntoViewIfNeeded();
      expect(await page.evaluate(() => scrollY)).toBeGreaterThan(1000);
      if (iteration === 1) {
        await open.focus();
        await page.keyboard.press("Enter");
      } else await open.click();
      await expect(page).toHaveURL(new RegExp(`/folders/folder-${index}$`));
      await expect(page.locator(".material-row")).toHaveCount(30);
      await expect.poll(() => page.evaluate(() => scrollY)).toBe(0);
      await expect(
        page.getByRole("heading", { name: `Collection ${index}`, exact: true }),
      ).toBeInViewport();
      await page.locator(".material-row").last().scrollIntoViewIfNeeded();
      expect(await page.evaluate(() => scrollY)).toBeGreaterThan(500);
      if (iteration === 0) await page.goBack();
      else if (iteration === 1)
        await page.getByRole("link", { name: "Back to library" }).click();
      else
        await page
          .getByRole("navigation", { name: "Main navigation" })
          .getByRole("link", { name: "My library" })
          .click();
      await expect(page).toHaveURL(/\/$/);
      await expect(page.locator(".folder-card")).toHaveCount(24);
    }
    const nav = page.getByRole("navigation", { name: "Main navigation" });
    await expect(nav).toBeInViewport();
    await nav.getByRole("link", { name: "Training", exact: true }).click();
    await expect(page).toHaveURL(/\/training$/);
  });

  test(`recall answers fit and controls work (${width}px)`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
    const state = await fixture(page);
    await page.goto("/train");
    for (let i = 0; i < answers.length; i++) {
      await expect(page.locator(".lead-question")).toHaveText(questionAt(i));
      await page
        .getByRole("button", { name: "Flip to answer", exact: true })
        .click();
      const face = page.getByRole("button", {
        name: "Flip to question",
        exact: true,
      });
      await expect(face).toBeVisible();
      await page.evaluate(() => document.fonts.ready);
      await page.locator(".recall-card").evaluate(async (el) => {
        await Promise.all(
          el.getAnimations({ subtree: true }).map((a) => a.finished),
        );
      });
      const typography = await face
        .locator(".recall-lead .markdown")
        .evaluate((el) => ({
          font: parseFloat(getComputedStyle(el).fontSize),
          line: parseFloat(getComputedStyle(el).lineHeight),
        }));
      if (width < 600) {
        expect(typography.font).toBeGreaterThanOrEqual(15);
        expect(typography.font).toBeLessThanOrEqual(16);
        expect(typography.line / typography.font).toBeGreaterThanOrEqual(1.4);
        expect(typography.line / typography.font).toBeLessThanOrEqual(1.65);
      } else {
        expect(typography.font).toBeGreaterThanOrEqual(16);
        expect(typography.font).toBeLessThanOrEqual(18);
      }
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      expect(
        await face.evaluate((el) => el.scrollWidth <= el.clientWidth + 1),
      ).toBe(true);
      if (width < 600) {
        for (const code of await face.locator("pre").all())
          expect(
            await code.evaluate((el) => el.scrollWidth <= el.clientWidth + 1),
          ).toBe(true);
      }
      // The last paragraph remains reachable inside the existing scrollable face.
      await face.evaluate((el) => {
        el.scrollTop = el.scrollHeight;
      });
      const last = await face
        .locator(".markdown")
        .last()
        .evaluate((el) => el.getBoundingClientRect().bottom);
      const bounds = (await face.boundingBox())!;
      expect(last).toBeLessThanOrEqual(bounds.y + bounds.height + 1);
      const meta = (await page.locator(".recall-meta").boundingBox())!;
      const wrong = (await page
        .getByRole("button", { name: "Wrong", exact: true })
        .boundingBox())!;
      const correct = (await page
        .getByRole("button", { name: "Correct", exact: true })
        .boundingBox())!;
      expect(meta.y).toBeGreaterThanOrEqual(bounds.y + bounds.height);
      expect(wrong.y).toBeGreaterThanOrEqual(meta.y + meta.height);
      expect(wrong.x + wrong.width).toBeLessThan(correct.x);
      await expect(
        page.getByRole("button", { name: "Edit current material" }),
      ).toBeEnabled();
      if (i === 2 || i === 7) {
        await face.evaluate((el) => {
          el.scrollTop = 0;
        });
        await page.screenshot({
          path: testInfo.outputPath(`answer-${i}-${width}.png`),
          fullPage: true,
        });
      }
      await face.focus();
      await page.keyboard.press("Enter");
      await expect(page.locator(".flip-scene")).not.toHaveClass(/is-flipped/);
      await page
        .getByRole("button", { name: i % 2 ? "Wrong" : "Correct", exact: true })
        .click();
      await expect(page.locator(".lead-question")).toHaveText(
        questionAt(i + 1),
      );
      await expect(page.locator(".saved-status")).toHaveText("Progress saved");
      await expect
        .poll(() => page.locator(".flip-front").evaluate((el) => el.scrollTop))
        .toBe(0);
    }
    await page.getByRole("button", { name: "Undo last answer" }).click();
    await expect(page.locator(".lead-question")).toHaveText("Question 7");
    await expect(page.locator(".flip-scene")).not.toHaveClass(/is-flipped/);
    await page.reload();
    await expect(page.locator(".lead-question")).toHaveText("Question 7");
    await expect(page.locator(".saved-status")).toHaveText("Progress saved");
    await page.getByRole("button", { name: "Edit current material" }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Save material", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    expect(state.edits).toBe(1);
    await expect(page.locator(".lead-question")).toHaveText("Question 7");
    expect(state.actions).toEqual([
      "correct",
      "wrong",
      "correct",
      "wrong",
      "correct",
      "wrong",
      "correct",
    ]);
  });
}
