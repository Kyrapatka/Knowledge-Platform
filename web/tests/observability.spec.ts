import { test, expect } from "@playwright/test";

test("observability: real login, normal training and mock interview publish to ClickHouse", async ({
  page,
  request,
}) => {
  test.skip(
    process.env.OBSERVABILITY_E2E !== "true",
    "Requires the local Compose analytics stack",
  );
  test.setTimeout(180000);
  const nickname = `obs${Date.now().toString(36)}`;
  const password = "ObservabilityTest2026!";
  const registered = await request.post("/api/v1/auth/register", {
    data: { nickname, password },
  });
  expect(registered.ok()).toBeTruthy();
  const auth = await registered.json();
  const headers = { Authorization: `Bearer ${auth.access_token}` };
  const folderResponse = await request.post("/api/v1/folders", {
    headers,
    data: { title: "Observability practice", template_key: "english_words" },
  });
  expect(folderResponse.status()).toBe(201);
  const folder = await folderResponse.json();
  for (const [foreign, native] of [
    ["observable", "visible"],
    ["reliable", "dependable"],
  ]) {
    expect(
      (
        await request.post(`/api/v1/folders/${folder.id}/materials`, {
          headers,
          data: {
            values: { foreign, native },
            metadata: { topic: "Observability" },
            difficulty: "easy",
          },
        })
      ).status(),
    ).toBe(201);
  }

  await page.goto("/");
  await page.getByLabel("Username", { exact: true }).fill(nickname);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "My library." }),
  ).toBeVisible();
  await page
    .getByRole("link", { name: "Observability practice", exact: true })
    .click();
  await page.getByRole("button", { name: "Train folder", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Start training", exact: true })
    .click();
  let answered = 0;
  while (answered < 12) {
    await expect(page.locator(".saved-status")).toHaveText("Progress saved");
    if (await page.locator(".all-done").isVisible()) break;
    await page
      .getByRole("button", { name: "Flip to answer", exact: true })
      .click();
    const saved = page.waitForResponse(
      (r) => r.url().endsWith("/actions") && r.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Correct", exact: true }).click();
    expect((await saved).ok()).toBeTruthy();
    await expect(page.getByRole("button", { name: "Correct", exact: true }))
      .toBeEnabled()
      .catch(async () => {
        await expect(page.locator(".all-done")).toBeVisible();
      });
    answered++;
  }
  expect(answered).toBeGreaterThanOrEqual(3);
  await expect(page.locator(".all-done")).toBeVisible();
  await page.goto("/interview");
  await page
    .getByRole("button", { name: "Import question bank", exact: true })
    .click();
  const modal = page.getByRole("dialog");
  await expect(modal.locator(".interview-seed-domains label")).toHaveCount(15);
  for (const label of await modal
    .locator(".interview-seed-domains label")
    .all()) {
    await label
      .getByRole("checkbox")
      .setChecked((await label.locator("span").innerText()).startsWith("Go\n"));
  }
  await modal.getByRole("button", { name: "Import selected subjects" }).click();
  await expect(modal).not.toBeVisible({ timeout: 60000 });
  await page.getByRole("checkbox", { name: /Bank testing mode/ }).check();
  await page
    .locator(".interview-source")
    .filter({ hasText: "Interview / Go" })
    .getByRole("checkbox")
    .check();
  await page
    .getByRole("combobox", { name: "Interview mode", exact: true })
    .selectOption("deep");
  await page
    .getByRole("combobox", { name: "Depth level", exact: true })
    .selectOption("2");
  await page
    .getByRole("button", { name: "Start interview", exact: true })
    .click();
  for (const result of ["Correct", "Wrong", "Correct"]) {
    await expect(page.locator(".interview-question")).toBeVisible();
    const saved = page.waitForResponse(
      (r) => r.url().endsWith("/actions") && r.request().method() === "POST",
    );
    await page.getByRole("button", { name: result, exact: true }).click();
    expect((await saved).ok()).toBeTruthy();
  }
  await page
    .getByRole("button", { name: "End interview", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "A little more prepared." }),
  ).toBeVisible();
  // Observe only this real test user's events; do not insert synthetic analytics.
  const chURL = `http://127.0.0.1:${process.env.KP_CLICKHOUSE_PORT || "8123"}/`;
  const reader = Buffer.from(
    `grafana_reader:${process.env.KP_CLICKHOUSE_READER_PASSWORD || "local-reader-only"}`,
  ).toString("base64");
  const required = [
    "user_registered",
    "user_logged_in",
    "folder_created",
    "material_created",
    "training_started",
    "training_answered",
    "training_completed",
    "interview_started",
    "interview_question_answered",
    "interview_completed",
  ];
  await expect
    .poll(
      async () => {
        const response = await request.post(chURL, {
          headers: { Authorization: `Basic ${reader}` },
          data: `SELECT groupUniqArray(event_name) AS names FROM knowledge_analytics.events_unique WHERE user_id='${auth.user.id}' FORMAT JSONEachRow`,
        });
        expect(response.ok()).toBeTruthy();
        const names = JSON.parse(await response.text()).names;
        return required.filter((name) => !names.includes(name));
      },
      { timeout: 20000 },
    )
    .toEqual([]);
  const checks = await request.post(chURL, {
    headers: { Authorization: `Basic ${reader}` },
    data: `SELECT countIf(event_name='training_answered' AND mode='mock') AS mixed, countIf(event_name='interview_question_answered' AND mode='mock' AND interview_depth IS NOT NULL) AS mock_answers, countIf(event_name='training_started' AND interview_mode='deep') AS deep_starts FROM knowledge_analytics.events_unique WHERE user_id='${auth.user.id}' FORMAT JSONEachRow`,
  });
  expect(JSON.parse(await checks.text())).toEqual({
    mixed: 0,
    mock_answers: 3,
    deep_starts: 1,
  });
});

// Browser rendering complements datasource/API query validation.
test("observability: provisioned Grafana dashboards render real data", async ({
  page,
}, testInfo) => {
  test.skip(process.env.OBSERVABILITY_E2E !== "true", "Requires local Grafana");
  test.setTimeout(120000);
  const queryErrors: string[] = [];
  page.on("response", async (response) => {
    if (!response.url().includes("/api/ds/query")) return;
    const body = await response.json().catch(() => ({}));
    for (const result of Object.values(body.results || {}) as {
      error?: string;
    }[]) {
      if (result.error) queryErrors.push(result.error);
    }
  });
  const grafana = `http://127.0.0.1:${process.env.KP_GRAFANA_PORT || "3000"}`;
  await page.goto(`${grafana}/login`);
  await page
    .locator('input[name="user"]')
    .fill(process.env.KP_GRAFANA_USER || "admin");
  await page
    .locator('input[name="password"]')
    .fill(process.env.KP_GRAFANA_PASSWORD || "local-grafana-only");
  await page.getByRole("button", { name: "Log in", exact: true }).click();
  await expect(page).not.toHaveURL(/\/login$/);
  for (const [uid, title] of [
    ["knowledge-system", "Knowledge Platform - System"],
    ["knowledge-learning", "Knowledge Platform - Learning Analytics"],
  ]) {
    await page.goto(`${grafana}/d/${uid}?from=now-1h&to=now`);
    await expect(page.getByText(title, { exact: true }).first()).toBeVisible();
    await expect(
      page
        .getByText(
          uid === "knowledge-system"
            ? "Requests / second"
            : "Training Sessions Started",
          { exact: true },
        )
        .first(),
    ).toBeVisible();
    await expect(
      page
        .locator('[data-testid="data-testid Panel status message"]')
        .filter({ hasText: /error/i }),
    ).toHaveCount(0);
    // Wait for actual query rendering, not just panel titles or loading placeholders.
    await expect(
      page.getByRole("button", { name: "Cancel", exact: true }),
    ).not.toBeVisible({ timeout: 45000 });
    const overview = page.getByRole("region", {
      name:
        uid === "knowledge-system"
          ? "In-flight requests"
          : "Training Sessions Started",
      exact: true,
    });
    await expect(overview).toContainText(/\d+/, { timeout: 45000 });
    if (uid === "knowledge-learning") {
      for (const name of [
        "Training starts and completions over time",
        "Correct / wrong rate by stage, difficulty, topic and template",
        "Topics with highest rehab entry rate",
        "Cram / long-term: answers, rehab, sessions and learning",
        "Mock answers and actual follow-up depth",
        "Materials most often entering rehab",
        "Version / group effectiveness",
        "User return retention D1 / D7 / D30",
      ]) {
        await page
          .getByRole("region", { name, exact: true })
          .scrollIntoViewIfNeeded();
        await page.waitForLoadState("networkidle");
        expect(queryErrors).toEqual([]);
      }
      // Exercise real selected-value interpolation, not only the All sentinel.
      await page.goto(`${grafana}/d/${uid}?from=now-1h&to=now&var-mode=mock`);
      await expect(
        page.getByRole("region", {
          name: "Training Sessions Started",
          exact: true,
        }),
      ).toContainText("0", { timeout: 45000 });
      const mock = page.getByRole("region", {
        name: "Mock answers and actual follow-up depth",
        exact: true,
      });
      await mock.scrollIntoViewIfNeeded();
      await page.waitForLoadState("networkidle");
      await expect(mock).toContainText("correct_rate");
      expect(queryErrors).toEqual([]);
      await page.goto(`${grafana}/d/${uid}?from=now-1h&to=now`);
      await expect(overview).toContainText(/\d+/, { timeout: 45000 });
      await overview.scrollIntoViewIfNeeded();
    }
    expect(queryErrors).toEqual([]);
    await expect(
      page.getByRole("button", { name: "Panel status", exact: true }),
    ).toHaveCount(0);
    await page.screenshot({
      path: testInfo.outputPath(`${uid}.png`),
      fullPage: true,
    });
  }
});
