import { expect, test } from "@playwright/test";
test("local Swagger loads its contract under CSP and calls the real liveness endpoint", async ({
  page,
  baseURL,
}) => {
  const foreignRequests: string[] = [];
  const cspErrors: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).origin !== new URL(baseURL!).origin)
      foreignRequests.push(request.url());
  });
  page.on("console", (message) => {
    if (/Content Security Policy|Refused to/i.test(message.text()))
      cspErrors.push(message.text());
  });
  const spec = await page.request.get("/openapi.yaml");
  expect(spec.status()).toBe(200);
  expect(spec.headers()["content-type"]).toContain("yaml");
  expect(await spec.text()).toContain("bearerAuth:");
  await page.goto("/swagger/");
  await expect(
    page.getByRole("heading", { name: /Knowledge Platform API/ }),
  ).toBeVisible();
  const operation = page.locator("#operations-Operations-live");
  await operation.locator(".opblock-summary").click();
  await operation.getByRole("button", { name: "Try it out" }).click();
  const response = page.waitForResponse(
    (r) => new URL(r.url()).pathname === "/live",
  );
  await operation.getByRole("button", { name: "Execute", exact: true }).click();
  expect((await response).status()).toBe(200);
  await expect(
    operation.locator(".responses-inner .response-col_status").first(),
  ).toContainText("200");
  expect(foreignRequests).toEqual([]);
  expect(cspErrors).toEqual([]);
});
