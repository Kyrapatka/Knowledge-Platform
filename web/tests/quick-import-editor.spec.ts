import { expect, test } from "@playwright/test";
import { fixture } from "./quick-import-fixture";

test("problem navigation opens Advanced and focuses its server-invalid field", async ({
  page,
}) => {
  await fixture(page);
  await page.getByLabel("Import source").fill("apple - яблоко");
  await page
    .getByRole("button", { name: "Review import", exact: true })
    .click();
  await expect(
    page.getByLabel("Metadata topic", { exact: true }),
  ).not.toBeVisible();
  await page.route("**/materials/import/validate", async (route) => {
    const body = route.request().postDataJSON();
    await route.fulfill({
      json: {
        valid: false,
        created: 0,
        skipped: 0,
        failed: 1,
        items: [],
        issues: [
          {
            client_id: body.items[0].client_id,
            path: "items[0].metadata.topic",
            code: "invalid_topic",
            severity: "error",
            message: "Choose a more specific topic",
          },
        ],
      },
    });
  });
  await page
    .getByRole("button", { name: "Import 1 card", exact: true })
    .click();
  await page.getByRole("button", { name: "Next problem", exact: true }).click();
  const field = page.getByLabel("Metadata topic", { exact: true });
  await expect(field).toBeVisible();
  await expect(field).toBeFocused();
  await expect(field).toHaveAttribute("aria-invalid", "true");
  await expect(
    page.getByText("Choose a more specific topic", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("invalid_topic", { exact: true }),
  ).not.toBeVisible();
  await field.fill("Fruit");
  await expect(field).not.toHaveAttribute("aria-invalid", "true");
  await expect(
    page.getByRole("button", { name: "Issues 0", exact: true }),
  ).toBeVisible();
});

test("multiline concepts preserve typing and grouped history without sending empty concepts", async ({
  page,
}) => {
  const imported = await fixture(page, "interview_questions");
  await page
    .getByLabel("Import source")
    .fill(
      "Question: What is concurrency?\nShort answer: Independent work progressing together.",
    );
  await page
    .getByRole("button", { name: "Review import", exact: true })
    .click();
  await page.getByText("Advanced", { exact: true }).click();
  const concepts = page.getByLabel("Profile concepts", { exact: true });
  await concepts.fill("Concurrency\n");
  await concepts.pressSequentially("Channels");
  await expect(concepts).toHaveValue("Concurrency\nChannels");
  await page.getByRole("button", { name: "Undo", exact: true }).click();
  await expect(concepts).toHaveValue("");
  await page.getByRole("button", { name: "Redo", exact: true }).click();
  await expect(concepts).toHaveValue("Concurrency\nChannels");
  await page
    .getByRole("button", { name: "Import 1 card", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Confirm import", exact: true })
    .click();
  await expect(page.getByText("1 cards added", { exact: true })).toBeVisible();
  expect(imported.commands[0].items[0].profile.concepts).toEqual([
    "Concurrency",
    "Channels",
  ]);
});
