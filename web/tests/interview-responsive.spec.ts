import { test, expect } from "@playwright/test";
import { defaultGraphConfig } from "../src/interview-types";

const longQuestion =
  "Explain how context cancellation interacts with database transactions, retries and concurrent updates. ".repeat(
    10,
  );
const detailed = [
  "A transaction groups related changes so they commit together.",
  "1. Validate the command.\n2. Lock the account and read current state.\n3. Persist changes and the receipt atomically.",
  '```go\nresult, err := repository.FindByContextAndTransactionIdentifier(ctx, transactionIdentifier)\nif err != nil { return fmt.Errorf("transaction: %w", err) }\n```',
  ...Array.from(
    { length: 24 },
    (_, i) =>
      `Paragraph ${i + 1}. ${"A retry must preserve the original command identity and must not apply changes twice. ".repeat(4)}`,
  ),
  "Final paragraph: preserve transaction boundaries.",
].join("\n\n");

for (const width of [320, 375, 390, 430, 768, 1440, 1920]) {
  test(`interview prose and Next after a long answer (${width}px)`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    let index = 0;
    const view = () => ({
      session: { id: "mock", status: "active" },
      summary: { correct: index, wrong: 0 },
      graph: {
        state: {
          practice_only: true,
          config: defaultGraphConfig,
          questions_asked: index + 1,
          current_root: 1,
          current_depth: 1,
        },
      },
      current: {
        id: `question-${index}`,
        material_id: `material-${index}`,
        progress_version: 0,
        question: [
          {
            key: "question",
            value: index === 0 ? longQuestion : "What makes a retry safe?",
          },
        ],
        answer: [
          {
            key: "short_answer",
            value:
              "Reuse the command ID and commit the receipt with its changes.",
          },
          { key: "answer", value: detailed },
        ],
        interview_graph: { review_credit: false },
      },
    });
    await page.route("**/api/v1/**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith("/auth/browser/refresh"))
        return route.fulfill({
          json: {
            access_token: "preview",
            user: { id: "prose", nickname: "Preview", status: "active" },
          },
        });
      if (path.endsWith("/library"))
        return route.fulfill({
          json: {
            folders: [],
            totals: {
              folder_count: 0,
              material_count: 0,
              due_count: 0,
              learning_count: 0,
              completed_count: 0,
            },
          },
        });
      if (path.endsWith("/actions")) {
        expect(route.request().postDataJSON().presentation_id).toBe(
          `question-${index}`,
        );
        index++;
        return route.fulfill({ json: view() });
      }
      if (path.endsWith("/mock-interviews/active"))
        return route.fulfill({ json: view() });
      return route.fulfill({ status: 404, json: { error: "not_found" } });
    });
    await page.goto("/interview");
    const question = page.locator(".interview-question");
    await expect(question).toContainText("Explain how context cancellation");
    await expect(question).toHaveCSS(
      "font-size",
      width <= 540 ? "19px" : "24px",
    );
    await page
      .getByRole("button", { name: "Reveal Answer", exact: true })
      .click();
    const short = page.locator(
      '.interview-reference [data-field="short_answer"] .markdown',
    );
    const full = page.locator(
      '.interview-reference [data-field="answer"] .markdown',
    );
    await expect(short).toHaveCSS("font-size", width <= 540 ? "16px" : "17px");
    await expect(full).toHaveCSS("font-size", width <= 540 ? "15px" : "16px");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    for (const el of await page
      .locator(
        ".interview-question, .interview-reference, .interview-reference pre",
      )
      .all()) {
      // Desktop code blocks may scroll locally; prose must always fit the viewport.
      const box = (await el.boundingBox())!;
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width).toBeLessThanOrEqual(width + 1);
    }
    if (width === 390 || width === 1440)
      await page.screenshot({
        path: testInfo.outputPath(`interview-${width}.png`),
        fullPage: true,
      });
    await full
      .getByText("Final paragraph: preserve transaction boundaries.", {
        exact: true,
      })
      .scrollIntoViewIfNeeded();
    expect(await page.evaluate(() => scrollY)).toBeGreaterThan(1000);
    await page.getByRole("button", { name: "Next Root", exact: false }).click();
    await expect(question).toHaveText("What makes a retry safe?");
    await expect
      .poll(() =>
        page
          .locator(".interview-conversation")
          .evaluate((el) => el.getBoundingClientRect().top),
      )
      .toBeGreaterThanOrEqual(95);
    await expect(question).toBeInViewport();
    // A short, visible next question should not trigger another scroll.
    await page.evaluate(() => {
      (window as unknown as { scrollCalls: number }).scrollCalls = 0;
      const original = Element.prototype.scrollIntoView;
      Element.prototype.scrollIntoView = function (...args) {
        (window as unknown as { scrollCalls: number }).scrollCalls++;
        return original.apply(this, args);
      };
    });
    await page.getByRole("button", { name: "Correct", exact: true }).click();
    await expect.poll(() => index).toBe(2);
    await expect(question).toBeInViewport();
    expect(
      await page.evaluate(
        () => (window as unknown as { scrollCalls: number }).scrollCalls,
      ),
    ).toBe(0);
  });
}
