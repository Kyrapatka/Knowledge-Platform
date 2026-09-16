import { test, expect } from "@playwright/test";

for (const [domain, count] of [["Algorithms", 8], ["Go", 130]] as const) {
  test(`real ${domain}: delete folder and reimport restores library count`, async ({ page }) => {
    test.setTimeout(120000);
    const errors: string[] = [];
    page.on("pageerror", e => errors.push(e.message));
    await page.goto("/");
    await page.getByRole("button", { name: "Create an account", exact: true }).click();
    await page.getByLabel("Username", { exact: true }).fill(`seed${Date.now().toString(36)}`);
    await page.getByLabel("Password", { exact: true }).fill("TestKnowledge2026!");
    await page.getByRole("button", { name: "Create account", exact: true }).click();
    await expect(page.getByRole("heading", { name: "My library." })).toBeVisible();

    async function importBank(expectedCreated: number, expectedSkipped: number) {
      await page.goto("/interview");
      await page.getByRole("button", { name: "Import question bank", exact: true }).click();
      const modal = page.getByRole("dialog");
      await expect(modal.locator(".interview-seed-domains label")).toHaveCount(15);
      for (const label of await modal.locator(".interview-seed-domains label").all()) {
        const checkbox = label.getByRole("checkbox");
        if ((await label.locator("span").innerText()).startsWith(`${domain}\n`)) await checkbox.check();
        else await checkbox.uncheck();
      }
      const response = page.waitForResponse(r => r.url().endsWith("/interview/seed/import") && r.request().method() === "POST");
      const refreshedLibrary = page.waitForResponse(r => r.url().endsWith("/library") && r.request().method() === "GET");
      await modal.getByRole("button", { name: "Import selected subjects" }).click();
      const result = await response;
      expect(result.status()).toBe(200);
      const body = await result.json();
      expect(body.created).toBe(expectedCreated);
      expect(body.updated).toBe(0);
      expect(body.skipped).toBe(expectedSkipped);
      const library = await (await refreshedLibrary).json();
      const folders = library.folders.filter((f: { title: string }) => f.title === `Interview / ${domain}`);
      expect(folders).toHaveLength(1);
      expect(folders[0].material_count).toBe(count);
      await expect(modal).not.toBeVisible();
      await page.goto(`/folders/${folders[0].id}`);
      await expect(page.getByRole("heading", { name: `Interview / ${domain}`, exact: true })).toBeVisible();
      await expect(page.locator(".material-row, .material-card").first()).toBeVisible();
      return folders[0].id as string;
    }
    const original = await importBank(count, 0);
    expect(await importBank(0, count)).toBe(original);
    await page.getByRole("button", { name: "Delete folder", exact: true }).click();
    const modal = page.getByRole("dialog");
    await expect(modal).toContainText(`${count} materials`);
    const deleted = page.waitForResponse(r => r.request().method() === "DELETE");
    await modal.getByRole("button", { name: "Delete folder", exact: true }).click();
    expect((await deleted).status()).toBe(204);
    await expect(page.getByRole("heading", { name: "My library." })).toBeVisible();
    await expect(page.getByRole("link", { name: `Interview / ${domain}`, exact: true })).toHaveCount(0);
    const restored = await importBank(count, 0);
    expect(restored).not.toBe(original);
    await page.reload();
    await expect(page.getByRole("heading", { name: `Interview / ${domain}`, exact: true })).toBeVisible();
    await page.screenshot({ path: `test-results/reimport-${domain.toLowerCase()}.png`, fullPage: true });
    expect(errors).toEqual([]);
  });
}
