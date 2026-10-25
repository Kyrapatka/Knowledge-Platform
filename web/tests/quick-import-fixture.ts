import { expect, type Page } from "@playwright/test";
import { initialConfig } from "../src/fields";
export async function fixture(
  page: Page,
  template = "english_words",
  uncertain = false,
) {
  const folder = {
    id: "f1",
    title: "Import destination",
    template_key: template,
    description: "",
    config: initialConfig(template),
    config_version: 1,
    training_config: {
      default_algorithm_key: "english_basic",
      pool_size: 8,
      refill_threshold: 5,
    },
    training_config_version: 1,
    material_count: 1,
    due_count: 0,
    learning_count: 0,
    completed_count: 0,
    topics: [],
    selected_plan: null,
    created_at: "2026-10-25T00:00:00Z",
    updated_at: "2026-10-25T00:00:00Z",
  };
  const old = {
    id: "old",
    folder_id: "f1",
    values:
      template === "english_words"
        ? { foreign: "existing", native: "старое" }
        : { question: "Existing?", short_answer: "Yes" },
    metadata: {},
    difficulty: "medium",
    created_at: folder.created_at,
    updated_at: folder.updated_at,
  };
  const materials: any[] = [old];
  const commands: any[] = [];
  const receipts = new Map<string, any>();
  let folderWrites = 0;
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname,
      method = route.request().method();
    if (path.endsWith("/auth/browser/refresh"))
      return route.fulfill({
        json: {
          access_token: "fixture",
          user: {
            id: "quick-import-tests",
            nickname: "Import test",
            status: "active",
          },
        },
      });
    if (path.endsWith("/library"))
      return route.fulfill({
        json: {
          folders: [{ ...folder, material_count: materials.length }],
          totals: {
            folder_count: 1,
            material_count: materials.length,
            due_count: 0,
            learning_count: 0,
            completed_count: 0,
          },
        },
      });
    if (path.endsWith("/library/folders/f1/materials"))
      return route.fulfill({
        json: {
          items: materials,
          total: materials.length,
          limit: 30,
          offset: 0,
          topics: [],
          selected_plan: null,
          plans: [],
        },
      });
    if (path.includes("/materials/import") && method === "POST") {
      const body = route.request().postDataJSON();
      const key = template === "english_words" ? "foreign" : "question";
      const normalize = (s: string) =>
        s.trim().toLowerCase().replace(/\s+/g, " ");
      const seen = new Set(materials.map((m) => normalize(m.values[key])));
      const items = body.items.map((i: any) => {
        const duplicate = seen.has(normalize(i.values[key]));
        seen.add(normalize(i.values[key]));
        return {
          client_id: i.client_id,
          status:
            duplicate && i.duplicate_policy !== "allow" ? "skipped" : "create",
          duplicate_kind: duplicate ? "existing" : undefined,
          duplicate_of: duplicate ? "old" : undefined,
        };
      });
      const out = {
        valid: true,
        created: items.filter((i: any) => i.status === "create").length,
        skipped: items.filter((i: any) => i.status === "skipped").length,
        failed: 0,
        issues: [],
        items,
      };
      if (path.endsWith("/validate")) return route.fulfill({ json: out });
      commands.push(body);
      if (receipts.has(body.command_id))
        return route.fulfill({ json: receipts.get(body.command_id) });
      for (const i of body.items)
        if (
          items.find((r: any) => r.client_id === i.client_id).status ===
          "create"
        )
          materials.push({ ...old, id: `new-${materials.length}`, ...i });
      receipts.set(body.command_id, out);
      if (uncertain && commands.length === 1) return route.abort("failed");
      return route.fulfill({ json: out });
    }
    if (path.endsWith("/folders/import") && method === "POST") folderWrites++;
    if (path.endsWith("/training/sessions/active"))
      return route.fulfill({ status: 204 });
    return route.fulfill({ json: [] });
  });
  await page.goto("/folders/f1");
  await page.getByRole("button", { name: "Import", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Review import" }),
  ).toBeVisible();
  return {
    folder,
    commands,
    materials,
    get folderWrites() {
      return folderWrites;
    },
  };
}
