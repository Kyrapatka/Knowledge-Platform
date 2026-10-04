import { test, expect } from "@playwright/test";
import { createClientUUID } from "../src/uuid";

test("real Interview copy persists content/profile without progress; default, duplicates and deletion", async ({
  page,
  request,
}) => {
  test.setTimeout(120000);
  const nickname = `copy${createClientUUID().replaceAll("-", "").slice(0, 10)}`;
  const password = "CopyRegression2026!";
  const registered = await request.post("/api/v1/auth/register", {
    data: { nickname, password },
  });
  expect(registered.status()).toBe(201);
  const auth = await registered.json();
  const headers = { Authorization: `Bearer ${auth.access_token}` };
  async function folder(title: string) {
    const response = await request.post("/api/v1/folders", {
      headers,
      data: {
        title,
        template_key: "interview_questions",
        description:
          "Goals:\nExplain transactions.\nhttps://example.com/transactions",
      },
    });
    expect(response.status()).toBe(201);
    return response.json();
  }
  const source = await folder("Copy source"),
    target = await folder("Copy target"),
    oneOff = await folder("One-off target");
  const content = {
    values: {
      question: "What is atomicity?",
      short_answer: "All or nothing.",
      answer: "All operations in the transaction commit together.",
      source: "https://example.com/transactions",
    },
    metadata: { topic: "Transactions", category: "SQL" },
    difficulty: "hard",
  };
  // Use exactly the real template field keys; preserve nullable optional fields too.
  for (const key of Object.keys(content.values))
    if (
      !source.config.schema.fields.some((f: { key: string }) => f.key === key)
    )
      delete (content.values as Record<string, string>)[key];
  const created = await request.post(`/api/v1/folders/${source.id}/materials`, {
    headers,
    data: content,
  });
  expect(created.status()).toBe(201);
  const material = await created.json();
  const profilePath = `/api/v1/folders/${source.id}/interview/questions/${material.id}/profile`;
  const base = await (await request.get(profilePath, { headers })).json();
  const savedProfile = await request.put(profilePath, {
    headers,
    data: {
      ...base,
      expected_version: 0,
      domain: "sql",
      topic: "Transactions",
      subtopic: "Atomicity",
      frequency: 9,
      interview_difficulty: 4,
      concepts: [{ slug: "atomicity", role: "primary", weight: 1, ordinal: 0 }],
      status: "draft",
    },
  });
  expect(savedProfile.status()).toBe(200);
  const originalProfile = await savedProfile.json();
  const started = await request.post("/api/v1/training/combined", {
    headers,
    data: { sources: [{ folder_id: source.id }] },
  });
  expect(started.status()).toBe(200);
  const view = await started.json();
  expect(view.current).toBeTruthy();
  const answered = await request.post(
    `/api/v1/training/sessions/${view.current.session_id}/actions`,
    {
      headers,
      data: {
        command_id: createClientUUID(),
        presentation_id: view.current.presentation.id,
        expected_version: view.current.presentation.progress_version,
        action: "correct",
      },
    },
  );
  expect(answered.status()).toBe(200);
  async function materials(id: string) {
    const response = await request.get(
      `/api/v1/library/folders/${id}/materials`,
      { headers },
    );
    expect(response.status()).toBe(200);
    return response.json();
  }
  const before = (await materials(source.id)).items[0].progress;
  expect(before).toBeTruthy();
  await page.goto("/");
  await page.getByLabel("Username", { exact: true }).fill(nickname);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.getByRole("link", { name: "Copy source", exact: true }).click();
  const dialog = page.getByRole("dialog");
  const requests: unknown[] = [];
  page.on("request", (r) => {
    if (r.url().endsWith("/copy") && r.method() === "POST")
      requests.push(r.postDataJSON());
  });
  async function openCopy() {
    await page.getByRole("button", { name: /What is atomicity/ }).click();
    await dialog
      .getByRole("button", { name: "Copy question", exact: true })
      .click();
  }
  async function copySelected(label = "Copy question") {
    const result = page.waitForResponse(
      (r) => r.url().endsWith("/copy") && r.request().method() === "POST",
    );
    await dialog.getByRole("button", { name: label, exact: true }).click();
    return result;
  }
  await openCopy();
  await dialog.getByLabel("Search copy folders").fill("COPY TARGET");
  await dialog.locator(".copy-target").click();
  await dialog.getByLabel("Make this my default copy folder").check();
  expect((await copySelected()).status()).toBe(201);
  await expect(dialog).not.toBeVisible();
  const copies = await materials(target.id);
  expect(copies.items).toHaveLength(1);
  const clone = copies.items[0];
  expect(clone.id).not.toBe(material.id);
  expect(clone.progress).toBeFalsy();
  expect(clone.values).toEqual(material.values);
  expect(clone.metadata).toEqual(material.metadata);
  expect(clone.difficulty).toBe("hard");
  const clonedProfile = await (
    await request.get(
      `/api/v1/folders/${target.id}/interview/questions/${clone.id}/profile`,
      { headers },
    )
  ).json();
  for (const key of [
    "domain",
    "topic",
    "subtopic",
    "concepts",
    "frequency",
    "interview_difficulty",
    "interview_profiles",
  ])
    expect(clonedProfile[key]).toEqual(originalProfile[key]);
  expect((await materials(source.id)).items[0].progress).toEqual(before);
  expect(requests[0]).toEqual({
    target_folder_id: target.id,
    allow_duplicate: false,
  });
  await openCopy();
  await expect(dialog.getByRole("status")).toHaveText("Selected: Copy target");
  expect((await copySelected()).status()).toBe(409);
  await dialog
    .getByRole("button", { name: "Open existing", exact: true })
    .click();
  await expect(page).toHaveURL(
    new RegExp(`/folders/${target.id}\\?material=${clone.id}`),
  );
  await expect(
    dialog.getByRole("heading", { name: "Material details", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await page.goto(`/folders/${source.id}`);
  await openCopy();
  expect((await copySelected()).status()).toBe(409);
  expect((await copySelected("Create another copy")).status()).toBe(201);
  await expect(dialog).not.toBeVisible();
  expect((await materials(target.id)).items).toHaveLength(2);
  await openCopy();
  await dialog
    .locator(".copy-target")
    .filter({ hasText: "One-off target" })
    .last()
    .click();
  expect((await copySelected()).status()).toBe(201);
  await expect(dialog).not.toBeVisible();
  await openCopy();
  await expect(dialog.getByRole("status")).toHaveText("Selected: Copy target");
  await page.keyboard.press("Escape");
  const removed = await request.delete(`/api/v1/folders/${target.id}`, {
    headers,
  });
  expect(removed.ok()).toBeTruthy();
  await page.reload();
  await openCopy();
  await expect(dialog.getByRole("status")).toHaveText(
    "Choose a target folder.",
  );
  const prefs = await page.evaluate(
    (key) => JSON.parse(localStorage.getItem(key)!),
    `knowledge:library-preferences:${auth.user.id}`,
  );
  expect(prefs.defaultCopyFolderId).toBeNull();
  expect(prefs.recentCopyTargets).toEqual([oneOff.id]);
});
