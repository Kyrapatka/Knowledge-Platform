import { test, expect, type Page } from "@playwright/test";
import { defaultGraphConfig } from "../src/interview-types";
import { initialConfig } from "../src/fields";
import { defaultPreferences } from "../src/library/preferences";
import type { MockPreset } from "../src/mock-setup";
async function fixture(page: Page) {
  const folders = ["Go Core", "Concurrency", "PostgreSQL", "Pet Project"].map(
    (title, i) => ({
      id: `f${i}`,
      title,
      template_key: "interview_questions",
      description: "",
      config: initialConfig("interview_questions"),
      config_version: 1,
      training_config: {
        default_algorithm_key: "interview_long_term",
        pool_size: 5,
        refill_threshold: 3,
      },
      training_config_version: 1,
      material_count: 10,
      due_count: 0,
      learning_count: 0,
      completed_count: 0,
      topics: [],
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    }),
  );
  let presets: MockPreset[] = [];
  const starts: any[] = [];
  const bulk: any[] = [];
  let materials = Array.from({ length: 3 }, (_, i) => ({
    id: `m${i}`,
    folder_id: "f0",
    values: { question: `Question ${i}`, answer: `Answer ${i}` },
    metadata: {},
    difficulty: "medium",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    progress: null,
  }));
  let active: any = null;
  let sequence = 0;
  await page.addInitScript(
    (prefs) => {
      if (!localStorage.getItem("knowledge:library-preferences:preset-tests"))
        localStorage.setItem(
          "knowledge:library-preferences:preset-tests",
          JSON.stringify(prefs),
        );
    },
    {
      ...defaultPreferences(),
      categories: [
        { id: "go", name: "Go", collapsed: false },
        { id: "sql", name: "SQL", collapsed: false },
      ],
      folders: {
        f0: { categoryId: "go" },
        f1: { categoryId: "go" },
        f2: { categoryId: "sql" },
      },
    },
  );
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const method = route.request().method();
    const body = route.request().postDataJSON();
    if (path.endsWith("/auth/browser/refresh"))
      return route.fulfill({
        json: {
          access_token: "fixture",
          user: {
            id: "preset-tests",
            nickname: "Preset tests",
            status: "active",
          },
        },
      });
    if (path.endsWith("/library"))
      return route.fulfill({
        json: {
          folders,
          totals: {
            folder_count: 4,
            material_count: 40,
            due_count: 0,
            learning_count: 0,
            completed_count: 0,
          },
        },
      });
    if (path.endsWith("/interview-graph/config"))
      return route.fulfill({ json: defaultGraphConfig });
    if (path.endsWith("/mock-presets")) {
      if (method === "GET") return route.fulfill({ json: presets });
      const p = {
        ...body,
        id: `p${++sequence}`,
        version: 1,
        created_at: `2026-01-0${sequence}T00:00:00Z`,
        updated_at: "2026-01-01T00:00:00Z",
        last_used_at: null,
        unavailable_folder_ids: [],
      };
      presets.push(p);
      return route.fulfill({ status: 201, json: p });
    }
    if (path.includes("/mock-presets/")) {
      const id = path.split("/")[5];
      const p = presets.find((p) => p.id === id)!;
      if (path.endsWith("/duplicate")) {
        const copy = {
          ...p,
          id: `p${++sequence}`,
          name: p.name + " (copy)",
          last_used_at: null,
        };
        presets.push(copy);
        return route.fulfill({ json: copy });
      }
      if (method === "DELETE") {
        presets = presets.filter((p) => p.id !== id);
        return route.fulfill({ json: { deleted: true } });
      }
      if (method === "PUT") {
        Object.assign(p, body, { version: p.version + 1 });
        return route.fulfill({ json: p });
      }
      return route.fulfill({ json: p });
    }
    if (path.endsWith("/mock-interviews/preview"))
      return route.fulfill({
        json: {
          mode: body.config.interview_mode,
          depth_level: body.config.depth_level,
          strategy: { target_roots: 2, target_branch: 3 },
          topics: [
            { key: "go", label: "Go", available: 5, weight: 1, roots: 2 },
          ],
          slots: ["go"],
        },
      });
    if (path.endsWith("/mock-interviews") && method === "POST") {
      starts.push(body);
      const ids = body.preset_ids || [];
      for (const p of presets)
        if (ids.includes(p.id)) p.last_used_at = new Date().toISOString();
      active = {
        session: { id: `s${starts.length}`, status: "completed" },
        summary: { correct: 0, wrong: 0 },
        current: null,
        graph: {
          state: {
            practice_only: true,
            config: body.config,
            questions_asked: 0,
            current_root: 0,
            current_depth: 0,
            stop_reason: "no_remaining_questions",
          },
        },
      };
      return route.fulfill({ json: active });
    }
    if (
      path.endsWith("/mock-interviews/active") ||
      path.includes("/training/sessions/")
    )
      return active
        ? route.fulfill({ json: active })
        : route.fulfill({ status: 404, json: { error: "not_found" } });
    if (path.includes("/library/folders/") && path.endsWith("/materials"))
      return route.fulfill({
        json: {
          items: materials,
          total: materials.length,
          limit: 30,
          offset: 0,
          topics: [],
          plans: [],
          selected_plan: null,
        },
      });
    if (path.endsWith("/materials/bulk")) {
      bulk.push(body);
      if (body.action === "delete")
        materials = materials.filter((m) => !body.material_ids.includes(m.id));
      return route.fulfill({
        json: {
          affected: body.material_ids.length,
          material_ids: body.material_ids,
          folder_id: body.target_folder_id || "f0",
        },
      });
    }
    if (path.endsWith("/training/plans") && method === "GET")
      return route.fulfill({ json: [] });
    if (path.endsWith("/training/plans") && method === "POST")
      return route.fulfill({
        status: 201,
        json: {
          id: "plan",
          status: "active",
          algorithm_key: body.algorithm_key,
          version: 1,
          source_folder_ids: body.source_folder_ids,
          config: body,
        },
      });
    if (path.endsWith("/training-config")) {
      const f = folders.find((f) => path.includes(f.id))!;
      Object.assign(f, {
        training_config: body.training_config,
        training_config_version: body.version + 1,
      });
      return route.fulfill({
        json: {
          training_config: f.training_config,
          version: f.training_config_version,
        },
      });
    }
    return route.fulfill({ status: 404, json: { error: "fixture_missing" } });
  });
  return {
    folders,
    starts,
    bulk,
    presets: () => presets,
    clearSession: () => {
      active = null;
    },
  };
}
async function create(
  page: Page,
  name: string,
  ids: string[],
  settings = false,
) {
  await page
    .getByRole("button", { name: "+ Create preset", exact: true })
    .click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name", { exact: true }).fill(name);
  for (const title of ids)
    await dialog.getByRole("checkbox", { name: title, exact: false }).check();
  if (settings) {
    await dialog.getByLabel("Save Mock default settings").check();
    await dialog.getByLabel("Questions", { exact: true }).fill("20");
  }
  await dialog
    .getByRole("button", { name: "Save preset", exact: true })
    .click();
  await expect(dialog).toHaveCount(0);
}
test("presets persist, session overrides stay temporary, save/reset and multiple presets", async ({
  page,
}) => {
  const f = await fixture(page);
  await page.goto("/interview");
  await create(page, "Go Theory", ["Go Core", "Concurrency"], true);
  await create(page, "Database", ["Concurrency", "PostgreSQL"]);
  await page.getByRole("checkbox", { name: "Go Theory", exact: true }).check();
  await expect(page.getByLabel("Questions", { exact: true })).toHaveValue("20");
  await page.getByLabel("Questions", { exact: true }).fill("30");
  await page
    .getByRole("combobox", { name: "Interview mode", exact: true })
    .selectOption("deep");
  await page.getByLabel("Depth level").selectOption("2");
  await expect(
    page.getByText("Unsaved session changes", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Start Mock", exact: true }).click();
  await expect.poll(() => f.starts.length).toBe(1);
  expect(f.starts[0].preset_ids).toEqual(["p1"]);
  expect(f.starts[0].config.question_limit).toBe(30);
  expect(f.presets()[0].settings?.question_limit).toBe(20);
  f.clearSession();
  await page.evaluate(() =>
    localStorage.removeItem("knowledge:interview:preset-tests"),
  );
  await page.reload();
  await page.getByRole("checkbox", { name: "Go Theory", exact: true }).check();
  await expect(page.getByLabel("Questions", { exact: true })).toHaveValue("20");
  await page.getByLabel("Questions", { exact: true }).fill("30");
  await page.getByRole("button", { name: "Save changes to preset" }).click();
  await expect.poll(() => f.presets()[0].settings?.question_limit).toBe(30);
  await page.reload();
  await page.getByRole("checkbox", { name: "Go Theory", exact: true }).check();
  await expect(page.getByLabel("Questions", { exact: true })).toHaveValue("30");
  await page.getByLabel("Questions", { exact: true }).fill("40");
  await page.getByRole("button", { name: "Reset to preset defaults" }).click();
  await expect(page.getByLabel("Questions", { exact: true })).toHaveValue("30");
  await page.getByRole("checkbox", { name: "Database", exact: true }).check();
  await expect(page.getByLabel("Questions", { exact: true })).toHaveValue("24");
  await expect(
    page.getByText("3 folders selected", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Save changes to preset" }),
  ).toHaveCount(0);
  await expect(
    page.getByText(/Saved preset settings are not combined/),
  ).toBeVisible();
  await page.getByRole("button", { name: "Start Mock", exact: true }).click();
  expect(f.starts[1].preset_ids).toEqual(["p1", "p2"]);
  expect(f.starts[1].config.question_limit).toBe(24);
});
test("selector indeterminate, search, edit, duplicate, rename and delete", async ({
  page,
}) => {
  const f = await fixture(page);
  await page.goto("/interview");
  await create(page, "Go Theory", ["Go Core"]);
  await page.getByLabel("Actions for Go Theory").selectOption("edit");
  const dialog = page.getByRole("dialog");
  await expect(
    dialog.getByRole("checkbox", { name: "Select section Go", exact: true }),
  ).toHaveJSProperty("indeterminate", true);
  await dialog
    .getByRole("checkbox", { name: "Select section Go", exact: true })
    .check();
  await expect(
    dialog.getByRole("checkbox", { name: /Concurrency/ }),
  ).toBeChecked();
  await dialog.getByLabel("Search folders").fill("Pet");
  await expect(
    dialog.getByRole("checkbox", { name: /Pet Project/ }),
  ).toBeVisible();
  await dialog.getByRole("checkbox", { name: /Pet Project/ }).check();
  await dialog
    .getByRole("button", { name: "Save preset", exact: true })
    .click();
  await expect
    .poll(() => f.presets()[0].folder_ids)
    .toEqual(["f0", "f1", "f3"]);
  await page.getByLabel("Actions for Go Theory").selectOption("duplicate");
  await expect(
    page.getByRole("checkbox", { name: "Go Theory (copy)", exact: true }),
  ).toBeVisible();
  await page.getByLabel("Actions for Go Theory (copy)").selectOption("rename");
  await dialog.getByLabel("Name", { exact: true }).fill("Practice");
  await dialog
    .getByRole("button", { name: "Save preset", exact: true })
    .click();
  page.once("dialog", (d) => d.accept());
  await page.getByLabel("Actions for Practice").selectOption("delete");
  await expect(
    page.getByRole("checkbox", { name: "Practice", exact: true }),
  ).toHaveCount(0);
});
for (const width of [360, 390, 430])
  test(`mobile selector and manual Mock ${width}`, async ({ page }) => {
    const f = await fixture(page);
    await page.setViewportSize({ width, height: 850 });
    await page.goto("/interview");
    await page
      .getByRole("button", { name: "Choose folders manually", exact: true })
      .click();
    await expect(page.getByRole("checkbox", { name: /Go Core/ })).toBeHidden();
    await page
      .getByRole("button", { name: "Go · 0 selected →", exact: true })
      .click();
    await page.getByRole("checkbox", { name: /Go Core/ }).check();
    await page.getByRole("button", { name: "← Sections", exact: true }).click();
    await page
      .getByRole("button", { name: "Без секции · 0 selected →", exact: true })
      .click();
    await page.getByRole("checkbox", { name: /Pet Project/ }).check();
    await page.getByRole("button", { name: "Continue →", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Session Settings", exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.getByRole("button", { name: "Start Mock", exact: true }).click();
    await expect.poll(() => f.starts.length).toBe(1);
    expect(f.starts[0].sources).toEqual([
      { folder_id: "f0" },
      { folder_id: "f3" },
    ]);
    expect(f.presets()).toEqual([]);
  });
test("pool settings preserve threshold on invalid resize and persist after reload", async ({
  page,
}) => {
  const f = await fixture(page);
  await page.goto("/");
  await page
    .getByRole("link", { name: /Go Core/ })
    .first()
    .click();
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Pool size", { exact: true }).fill("1");
  await expect(
    dialog.getByLabel("Refill threshold", { exact: true }),
  ).toHaveValue("3");
  await expect(
    dialog.getByRole("button", { name: "Save settings" }),
  ).toBeDisabled();
  await dialog.getByLabel("Pool size", { exact: true }).fill("12");
  await dialog.getByLabel("Refill threshold", { exact: true }).fill("4");
  await dialog.getByRole("button", { name: "Save settings" }).click();
  await expect(dialog).toHaveCount(0);
  expect(f.folders[0].training_config).toMatchObject({
    pool_size: 12,
    refill_threshold: 4,
  });
  await page.reload();
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await expect(dialog.getByLabel("Pool size", { exact: true })).toHaveValue(
    "12",
  );
  await expect(
    dialog.getByLabel("Refill threshold", { exact: true }),
  ).toHaveValue("4");
});

test("bulk material Mock preserves exact IDs in Deep and never selects folder presets", async ({
  page,
}) => {
  const f = await fixture(page);
  await page.goto("/folders/f0");
  await page
    .getByRole("checkbox", { name: "Select Question 0", exact: true })
    .check();
  await page
    .getByRole("checkbox", { name: "Select Question 2", exact: true })
    .check();
  await page
    .getByRole("region", { name: "Material bulk actions" })
    .getByRole("button", { name: "Start Mock", exact: true })
    .click();
  await expect(
    page.getByText("Only the 2 selected questions will participate."),
  ).toBeVisible();
  await page
    .getByRole("combobox", { name: "Interview mode", exact: true })
    .selectOption("deep");
  await page
    .getByRole("combobox", { name: "Depth level", exact: true })
    .selectOption("3");
  await page.getByRole("button", { name: "Start Mock", exact: true }).click();
  await expect.poll(() => f.starts.length).toBe(1);
  expect(f.starts[0].sources).toEqual([
    { folder_id: "f0", material_ids: ["m0", "m2"] },
  ]);
  expect(f.starts[0].preset_ids).toBeUndefined();
  expect(f.starts[0].config.interview_mode).toBe("deep");
});
test("bulk copy metadata and delete keep using the batch endpoint", async ({
  page,
}) => {
  const f = await fixture(page);
  await page.goto("/folders/f0");
  const select = async () => {
    await page
      .getByRole("checkbox", { name: "Select Question 0", exact: true })
      .check();
    await page
      .getByRole("checkbox", { name: "Select Question 2", exact: true })
      .check();
  };
  const toolbar = page.getByRole("region", { name: "Material bulk actions" });
  const dialog = page.getByRole("dialog");
  await select();
  await toolbar.getByRole("button", { name: "Copy", exact: true }).click();
  await dialog.getByLabel("Bulk copy target").selectOption("f1");
  await dialog
    .getByRole("button", { name: "Copy materials", exact: true })
    .click();
  await expect.poll(() => f.bulk.length).toBe(1);
  expect(f.bulk[0]).toMatchObject({
    action: "copy",
    material_ids: ["m0", "m2"],
    target_folder_id: "f1",
  });
  await expect(dialog).toHaveCount(0);
  await select();
  await toolbar
    .getByRole("button", { name: "Edit metadata", exact: true })
    .click();
  await dialog
    .getByRole("checkbox", { name: "Change Difficulty", exact: true })
    .check();
  await dialog
    .getByLabel("Bulk Difficulty", { exact: true })
    .selectOption("hard");
  await dialog
    .getByRole("button", { name: "Apply metadata", exact: true })
    .click();
  await expect.poll(() => f.bulk.length).toBe(2);
  expect(f.bulk[1]).toMatchObject({
    action: "metadata",
    difficulty: "hard",
    material_ids: ["m0", "m2"],
  });
  expect(f.bulk[1].values).toBeUndefined();
  await expect(dialog).toHaveCount(0);
  await select();
  await toolbar.getByRole("button", { name: "Delete", exact: true }).click();
  await dialog
    .getByRole("button", { name: "Confirm delete materials", exact: true })
    .click();
  await expect.poll(() => f.bulk.length).toBe(3);
  expect(f.bulk[2]).toMatchObject({
    action: "delete",
    material_ids: ["m0", "m2"],
  });
  await expect(
    page.getByRole("checkbox", { name: "Select Question 0", exact: true }),
  ).toHaveCount(0);
});
test("uncertain Mock start retries immutable command and explicit multi settings survive", async ({
  page,
}) => {
  const f = await fixture(page);
  await page.goto("/interview");
  await create(page, "A", ["Go Core"], true);
  await create(page, "B", ["Concurrency"]);
  await page.getByRole("checkbox", { name: "A", exact: true }).check();
  await page.getByLabel("Questions", { exact: true }).fill("30");
  await page.getByRole("checkbox", { name: "B", exact: true }).check();
  await expect(page.getByLabel("Questions", { exact: true })).toHaveValue("30");
  const commands: any[] = [];
  await page.route("**/api/v1/training/mock-interviews", async (route) => {
    commands.push(route.request().postDataJSON());
    if (commands.length === 1)
      return route.fulfill({
        status: 503,
        json: { error: "temporary_unavailable" },
      });
    return route.fallback();
  });
  await page.getByRole("button", { name: "Start Mock", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Retry start", exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel("Questions", { exact: true })).toBeDisabled();
  await expect(
    page.getByRole("checkbox", { name: "A", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Retry start", exact: true }).click();
  await expect.poll(() => f.starts.length).toBe(1);
  expect(commands[1]).toEqual(commands[0]);
  expect(f.presets()[0].settings?.question_limit).toBe(20);
});
test("empty and stale presets cannot start", async ({ page }) => {
  const f = await fixture(page);
  f.presets().push({
    id: "stale",
    name: "Removed sources",
    folder_ids: [],
    unavailable_folder_ids: ["gone"],
    settings: null,
    version: 1,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    last_used_at: null,
  });
  await page.goto("/interview");
  await expect(page.getByText("1 unavailable folders ignored.")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Start", exact: true }),
  ).toBeDisabled();
  await page
    .getByRole("checkbox", { name: "Removed sources", exact: true })
    .check();
  await expect(
    page.getByRole("button", { name: "Start Mock", exact: true }),
  ).toBeDisabled();
  expect(f.starts).toEqual([]);
});
