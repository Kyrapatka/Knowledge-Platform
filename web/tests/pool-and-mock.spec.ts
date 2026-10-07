import {
  test,
  expect,
  type Page,
  type APIRequestContext,
} from "@playwright/test";
import { createClientUUID } from "../src/uuid";
async function account(page: Page, request: APIRequestContext) {
  const nickname = `pool${createClientUUID().replaceAll("-", "").slice(0, 10)}`,
    password = "PoolRegression2026!";
  const response = await request.post("/api/v1/auth/register", {
    data: { nickname, password },
  });
  expect(response.status()).toBe(201);
  const auth = await response.json();
  const headers = { Authorization: `Bearer ${auth.access_token}` };
  async function login() {
    await page.goto("/");
    await page.getByLabel("Username", { exact: true }).fill(nickname);
    await page.getByLabel("Password", { exact: true }).fill(password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "My library." }),
    ).toBeVisible();
  }
  return { headers, login };
}
for (const [template, target, threshold] of [
  ["interview_questions", 5, 3],
  ["english_words", 8, 5],
] as const) {
  test(`real ${template} pool refills only at ${threshold} of ${target}`, async ({
    page,
    request,
  }) => {
    test.setTimeout(180000);
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    const { headers, login } = await account(page, request);
    const created = await request.post("/api/v1/folders", {
      headers,
      data: { title: "Pool threshold", template_key: template },
    });
    expect(created.status()).toBe(201);
    const folder = await created.json();
    for (let i = 0; i < target + 5; i++) {
      const values =
        template === "english_words"
          ? { foreign: `word ${i}`, native: `translation ${i}` }
          : { question: `Question ${i}?`, answer: `Reference answer ${i}` };
      const response = await request.post(
        `/api/v1/folders/${folder.id}/materials`,
        { headers, data: { values, difficulty: "easy" } },
      );
      expect(response.status()).toBe(201);
    }
    await login();
    await page
      .getByRole("link", { name: "Pool threshold", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Train folder", exact: true })
      .click();
    const started = page.waitForResponse(
      (r) =>
        r.url().endsWith("/training/combined") &&
        r.request().method() === "POST",
    );
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Start training", exact: true })
      .click();
    const initial = await (await started).json();
    expect(initial.sessions[0].pool_size).toBe(target);
    expect(initial.sessions[0].plan.config.refill_threshold).toBe(threshold);
    const sizes: number[] = [];
    let departures = 0;
    for (let n = 0; n < target * 4 && departures < target - threshold; n++) {
      const result = page.waitForResponse(
        (r) => r.url().endsWith("/actions") && r.request().method() === "POST",
      );
      await page.getByRole("button", { name: "Correct", exact: true }).click();
      const response = await result;
      expect(response.status()).toBe(200);
      const action = await response.json();
      if (action.event.stage_after > action.event.stage_before) {
        departures++;
        sizes.push(action.session.pool_size);
      } else expect(action.session.pool_size).toBe(target);
      await expect(
        page.getByRole("button", { name: "Correct", exact: true }),
      ).toBeEnabled();
    }
    expect(sizes).toEqual(template === "english_words" ? [7, 6, 8] : [4, 5]);
    expect(errors).toEqual([]);
  });
}
for (const mode of ["real", "deep"] as const) {
  test(`real ${mode} interview continues through fallback roots and finishes eligible bank`, async ({
    page,
    request,
  }) => {
    test.setTimeout(120000);
    const { headers, login } = await account(page, request);
    const created = await request.post("/api/v1/folders", {
      headers,
      data: { title: "Fallback bank", template_key: "interview_questions" },
    });
    expect(created.status()).toBe(201);
    const folder = await created.json();
    for (let i = 0; i < 7; i++) {
      const response = await request.post(
        `/api/v1/folders/${folder.id}/materials`,
        {
          headers,
          data: {
            values: {
              question: `Unconnected topic ${i}?`,
              answer: `Reference ${i}`,
            },
            difficulty: "easy",
          },
        },
      );
      expect(response.status()).toBe(201);
      const material = await response.json();
      const path = `/api/v1/folders/${folder.id}/interview/questions/${material.id}/profile`;
      const profile = await (await request.get(path, { headers })).json();
      const saved = await request.put(path, {
        headers,
        data: {
          ...profile,
          expected_version: 0,
          status: "ready",
          domain: "go",
          root_weight: i === 0 ? 10 : 0,
          concepts: [
            { slug: `concept_${i}`, role: "primary", weight: 1, ordinal: 0 },
            { slug: `concept_${i}`, role: "tested", weight: 1, ordinal: 0 },
          ],
        },
      });
      expect(saved.status()).toBe(200);
    }
    await login();
    await page
      .getByRole("link", { name: "Mock interview", exact: true })
      .click();
    await page
      .locator(".interview-source")
      .filter({ hasText: "Fallback bank" })
      .getByRole("checkbox")
      .check();
    await page
      .getByRole("combobox", { name: "Interview mode", exact: true })
      .selectOption(mode);
    if (mode === "deep")
      await page
        .getByRole("combobox", { name: "Depth level", exact: true })
        .selectOption("3");
    await page
      .getByRole("spinbutton", { name: "Questions", exact: true })
      .fill("20");
    await expect(
      page.getByRole("button", { name: "Start interview", exact: true }),
    ).toBeEnabled();
    const started = page.waitForResponse(
      (r) =>
        r.url().endsWith("/training/mock-interviews") &&
        r.request().method() === "POST",
    );
    await page
      .getByRole("button", { name: "Start interview", exact: true })
      .click();
    let view = await (await started).json();
    const seen = new Set<string>();
    let fallback = false;
    for (let i = 0; i < 7; i++) {
      expect(view.current).toBeTruthy();
      expect(seen.has(view.current.material_id)).toBe(false);
      seen.add(view.current.material_id);
      if (view.graph.selection.selection_reason === "fallback_root")
        fallback = true;
      const next = page.waitForResponse(
        (r) => r.url().endsWith("/actions") && r.request().method() === "POST",
      );
      await page.getByRole("button", { name: "Correct", exact: true }).click();
      const response = await next;
      expect(response.status()).toBe(200);
      view = (await response.json()).session;
      if (i < 6) {
        expect(view.session.status).toBe("active");
        await expect(
          page.getByRole("button", { name: "Correct", exact: true }),
        ).toBeEnabled();
      }
    }
    expect(fallback).toBe(true);
    expect(view.session.status).toBe("completed");
    expect(view.graph.state.stop_reason).toBe("no_remaining_questions");
    expect(seen.size).toBe(7);
    await expect(
      page.getByRole("heading", { name: "A little more prepared." }),
    ).toBeVisible();
    const materials = await (
      await request.get(`/api/v1/library/folders/${folder.id}/materials`, {
        headers,
      })
    ).json();
    for (const material of materials.items)
      expect(material.progress).toBeFalsy();
  });
}
