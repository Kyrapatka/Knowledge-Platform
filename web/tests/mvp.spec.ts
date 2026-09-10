import { test, expect, type Page } from "@playwright/test";

async function register(page: Page) {
  await page.goto("/");
  await page
    .getByRole("button", { name: "Create an account", exact: true })
    .click();
  const nickname =
    `qa${Date.now().toString(36)}${Math.random().toString(36).slice(2, 5)}`.slice(
      0,
      16,
    );
  await page.getByLabel("Username", { exact: true }).fill(nickname);
  await page.getByLabel("Password", { exact: true }).fill("TestKnowledge2026!");
  await page
    .getByRole("button", { name: "Create account", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "My library." }),
  ).toBeVisible();
  return nickname;
}
async function createFolder(page: Page, name: string, type?: string) {
  await page
    .getByRole("button", { name: "Create folder", exact: true })
    .click();
  const dialog = page.getByRole("dialog");
  if (type)
    await dialog.getByRole("button", { name: new RegExp(type) }).click();
  await dialog.getByLabel("Folder name").fill(name);
  await dialog
    .getByLabel("Description")
    .fill("A collection for thoughtful, everyday practice.");
  await dialog
    .getByRole("button", { name: "Create folder", exact: true })
    .click();
  await expect(dialog).not.toBeVisible();
  await page.getByRole("link", { name, exact: true }).click();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
}
async function material(
  page: Page,
  values: Record<string, string>,
  topic: string,
) {
  await page.getByRole("button", { name: "Add material", exact: true }).click();
  const dialog = page.getByRole("dialog");
  for (const [label, value] of Object.entries(values))
    await dialog.getByLabel(label, { exact: true }).fill(value);
  await dialog.getByLabel("Topic", { exact: true }).fill(topic);
  await dialog
    .getByRole("combobox", { name: "Difficulty", exact: true })
    .selectOption("easy");
  await dialog
    .getByRole("button", { name: "Save material", exact: true })
    .click();
  await expect(dialog).not.toBeVisible();
}

test("desktop: real auth, CRUD, topic filtering, recall, persistence and statistics", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await register(page);
  await createFolder(page, "Everyday English");
  await material(
    page,
    { Foreign: "serendipity", Native: "A fortunate discovery by chance" },
    "Everyday",
  );
  await material(
    page,
    { Foreign: "resilient", Native: "Able to recover after difficulty" },
    "Work",
  );
  await page
    .getByRole("combobox", { name: "Filter by topic" })
    .selectOption("Everyday");
  await expect(page.getByRole("button", { name: /serendipity/ })).toBeVisible();
  await expect(page.getByRole("button", { name: /resilient/ })).toHaveCount(0);
  await page.getByRole("button", { name: "Train topic", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Start training", exact: true })
    .click();
  await expect(page.locator(".lead-question")).toContainText("serendipity");
  await page.getByRole("button", { name: "Edit current material" }).click();
  await page
    .getByRole("dialog")
    .getByRole("textbox", { name: "Foreign", exact: true })
    .fill("serendipity (updated)");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Save material", exact: true })
    .click();
  await expect(page.locator(".lead-question")).toHaveText("serendipity");
  await page.reload();
  await expect(page.locator(".lead-question")).toHaveText("serendipity");
  await page.getByRole("button", { name: "Show answer", exact: true }).click();
  await expect(page.locator(".answer-fields")).toContainText(
    "A fortunate discovery by chance",
  );
  await page.getByRole("button", { name: /Correct/ }).click();
  for (let i = 0; i < 2; i++) {
    await expect(
      page.getByRole("button", { name: "Show answer", exact: true }),
    ).toBeEnabled();
    await expect(page.locator(".lead-question")).toHaveText(
      "serendipity (updated)",
    );
    await page
      .getByRole("button", { name: "Show answer", exact: true })
      .click();
    await page.getByRole("button", { name: /Correct/ }).click();
  }
  await expect(
    page.getByRole("heading", { name: "All done for now." }),
  ).toBeVisible();
  await page
    .locator(".all-done")
    .getByRole("link", { name: "Back to library" })
    .click();
  await page.getByRole("link", { name: "Statistics", exact: true }).click();
  await expect(
    page.locator(".stats-metrics .metric").first().locator("strong"),
  ).toHaveText("3");
  await page.screenshot({
    path: "test-results/statistics-desktop.png",
    fullPage: true,
  });
  await page.getByRole("link", { name: /My library/ }).click();
  await createFolder(page, "Backend interviews", "Interview prep");
  await material(
    page,
    {
      Question: "What is an index?",
      Answer: "A data structure that speeds up retrieval.",
    },
    "SQL",
  );
  await page.getByRole("link", { name: "Back to library" }).click();
  await page.getByRole("checkbox", { name: "Select Everyday English" }).check();
  await page
    .getByRole("checkbox", { name: "Select Backend interviews" })
    .check();
  await page.screenshot({
    path: "test-results/library-desktop.png",
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Train together", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Start training", exact: true })
    .click();
  const questions: string[] = [];
  for (let i = 0; i < 4; i++) {
    await expect(
      page.getByRole("button", { name: "Show answer", exact: true }),
    ).toBeEnabled();
    questions.push(await page.locator(".lead-question").innerText());
    await page
      .getByRole("button", { name: "Show answer", exact: true })
      .click();
    await page.screenshot({
      path: `test-results/training-desktop-${i}.png`,
      fullPage: true,
    });
    await page.getByRole("button", { name: /Correct/ }).click();
  }
  expect(questions.join(" ")).toContain("resilient");
  expect(questions.join(" ")).toContain("What is an index?");
  await expect(
    page.getByRole("heading", { name: "All done for now." }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});

test("mobile: formula exercise, no layout overflow, login survives reload", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.setViewportSize({ width: 390, height: 844 });
  await register(page);
  await createFolder(page, "Physics essentials", "Formulas");
  await material(
    page,
    { Name: "Average speed", Formula: "v = \\frac{d}{t}" },
    "Motion",
  );
  await page.getByRole("button", { name: /Average speed/ }).click();
  await page.getByRole("button", { name: "Add exercise", exact: true }).click();
  await page
    .getByLabel("Problem", { exact: true })
    .fill("A car travels 100 km in 2 hours. Find its average speed.");
  await page.getByLabel("Answer", { exact: true }).fill("50 km/h");
  await page
    .getByLabel("Solution", { exact: true })
    .fill("Distance divided by time: 100 / 2 = 50 km/h.");
  await page
    .getByRole("button", { name: "Save exercise", exact: true })
    .click();
  await expect(page.locator(".exercise-item")).toContainText("A car travels");
  await page.getByRole("button", { name: "Close dialog" }).click();
  await page.screenshot({
    path: "test-results/folder-mobile.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "Train folder", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Start training", exact: true })
    .click();
  await expect(page.locator(".question-fields")).toContainText("A car travels");
  await page.getByRole("button", { name: "Show answer", exact: true }).click();
  await expect(page.locator(".answer-fields")).toContainText("50 km/h");
  const noOverflow = await page.evaluate(
    () => document.documentElement.scrollWidth <= window.innerWidth,
  );
  expect(noOverflow).toBeTruthy();
  await page.screenshot({
    path: "test-results/training-mobile.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: /Correct/ }).click();
  await expect(
    page.getByRole("heading", { name: "All done for now." }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "All done for now." }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: /Sign in/ })).toHaveCount(0);
  expect(errors).toEqual([]);
});

test("settings: change tracks, reuse plans and keep the displayed progress context", async ({
  page,
}) => {
  await register(page);
  await createFolder(page, "Interview practice", "Interview prep");
  for (const [algorithm, days] of [
    ["interview_long_term", "150"],
    ["interview_cram", "5"],
    ["interview_long_term", "120"],
  ]) {
    await page.getByRole("button", { name: "Settings", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog
      .getByRole("combobox", { name: "Learning algorithm" })
      .selectOption(algorithm);
    await dialog
      .getByRole("spinbutton", { name: "Learning horizon · days" })
      .fill(days);
    await dialog
      .getByRole("button", { name: "Save settings", exact: true })
      .click();
    await expect(dialog).not.toBeVisible();
    await expect(
      page
        .getByRole("combobox", { name: "Progress plan" })
        .locator("option:checked"),
    ).toContainText(algorithm === "interview_cram" ? "CRAM" : "Long-term");
  }
  await expect(
    page.getByRole("combobox", { name: "Progress plan" }).locator("option"),
  ).toHaveCount(2);
  await page.getByRole("button", { name: "Open profile" }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Sign out", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
});
