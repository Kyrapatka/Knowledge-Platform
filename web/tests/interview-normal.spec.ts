import { test, expect } from "@playwright/test";

test("Bank interview folder supports normal recall and independent mock", async ({ page }) => {
  test.setTimeout(180000);
  await page.goto("/");
  await page.getByRole("button", { name: "Create an account", exact: true }).click();
  await page.getByLabel("Username", { exact: true }).fill(`normal${Date.now().toString(36)}`);
  await page.getByLabel("Password", { exact: true }).fill("TestKnowledge2026!");
  await page.getByRole("button", { name: "Create account", exact: true }).click();
  await expect(page.getByRole("heading", { name: "My library." })).toBeVisible();
  await page.getByRole("link", { name: "Mock interview", exact: true }).click();
  await page.getByRole("button", { name: "Import question bank", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Import selected subjects" }).click();
  await expect(dialog).not.toBeVisible();

  await page.goto("/training");
  await page.getByRole("button", { name: "Start training", exact: true }).click();
  const goSource = dialog.getByRole("checkbox", { name: "Train with Interview / Go", exact: true });
  await expect(goSource).toBeVisible();
  for (const source of await dialog.getByRole("checkbox").all()) await source.uncheck();
  await goSource.check();
  const started = page.waitForResponse(r => r.url().endsWith("/training/combined") && r.request().method() === "POST");
  await dialog.getByRole("button", { name: "Start training", exact: true }).click();
  const response = await started;
  expect(response.ok()).toBeTruthy();
  const view = await response.json();
  expect(view.current).not.toBeNull();
  const card = view.current.presentation;
  expect(card.interview_graph).toBeFalsy();
  expect(card.answer.map((f: { key: string }) => f.key)).toContain("short_answer");
  expect(card.answer.map((f: { key: string }) => f.key)).toContain("answer");
  await expect(page.getByRole("button", { name: "Flip to question", exact: true })).not.toBeVisible();
  await page.getByRole("button", { name: "Flip to answer", exact: true }).click();
  const back = page.getByRole("button", { name: "Flip to question", exact: true });
  await expect(back).toContainText(card.answer.find((f: { key: string }) => f.key === "short_answer").value);
  await expect(back).toContainText("Detailed answer");
  const answered = page.waitForResponse(r => r.url().endsWith("/actions") && r.request().method() === "POST");
  await page.getByRole("button", { name: "Correct", exact: true }).click();
  expect((await answered).ok()).toBeTruthy();

  // Opening setup from an existing plan must not copy that plan to other folders.
  await page.goto(`/folders/${card.folder_id}`);
  await page.getByRole("button", { name: "Train folder", exact: true }).click();
  await dialog.getByRole("checkbox", { name: "Train with Interview / Go", exact: true }).uncheck();
  await dialog.getByRole("checkbox", { name: "Train with Interview / Algorithms", exact: true }).check();
  const switched = page.waitForResponse(r => r.url().endsWith("/training/combined") && r.request().method() === "POST");
  await dialog.getByRole("button", { name: "Start training", exact: true }).click();
  const switchedResponse = await switched;
  expect(switchedResponse.ok()).toBeTruthy();
  const switchedView = await switchedResponse.json();
  expect(switchedView.current).not.toBeNull();
  expect(switchedView.current.presentation.folder_id).not.toBe(card.folder_id);
  expect(switchedView.current.plan_id).not.toBe(view.current.plan_id);

  await page.goto("/interview");
  await page.getByRole("checkbox", { name: /Bank testing mode/ }).check();
  await page.locator(".interview-source").filter({ hasText: "Interview / Go" }).getByRole("checkbox").check();
  const mockStarted = page.waitForResponse(r => r.url().endsWith("/training/mock-interviews") && r.request().method() === "POST");
  await page.getByRole("button", { name: "Start interview", exact: true }).click();
  const mock = await (await mockStarted).json();
  expect(mock.current.interview_graph.review_credit).toBe(false);
  expect(mock.current.progress_version).toBe(0);
  await expect(page.locator(".interview-question")).toBeVisible();
});
