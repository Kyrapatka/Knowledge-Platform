import { expect, test } from "@playwright/test";
import { createClientUUID } from "../src/uuid";

const uuidV4 =
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

test("UUID prefers native randomUUID and preserves its receiver", () => {
  let calls = 0;
  const source = {
    randomUUID(): `${string}-${string}-${string}-${string}-${string}` {
      expect(this).toBe(source);
      calls++;
      return "12345678-1234-4567-89ab-123456789abc";
    },
    getRandomValues: (() => {
      throw new Error("fallback must not run");
    }) as Crypto["getRandomValues"],
  };
  expect(createClientUUID(source)).toBe("12345678-1234-4567-89ab-123456789abc");
  expect(calls).toBe(1);
});

test("UUID fallback sets version and variant bits and pads every byte", () => {
  for (const byte of [0, 0x40, 0x80, 0xff]) {
    const source = {
      getRandomValues: ((bytes: Uint8Array) => {
        expect(bytes).toHaveLength(16);
        bytes.fill(byte);
        return bytes;
      }) as Crypto["getRandomValues"],
    };
    const id = createClientUUID(source);
    expect(id).toMatch(uuidV4);
    expect(id[14]).toBe("4");
    expect(parseInt(id[19], 16) & 0xc).toBe(8);
    if (byte === 0) expect(id).toBe("00000000-0000-4000-8000-000000000000");
  }
});

test("UUID without randomUUID uses fresh cryptographic randomness each time", () => {
  const source = {
    getRandomValues: globalThis.crypto.getRandomValues.bind(globalThis.crypto),
  };
  const ids = Array.from({ length: 1000 }, () => createClientUUID(source));
  for (const id of ids) expect(id).toMatch(uuidV4);
  expect(new Set(ids).size).toBe(ids.length);
});

test("UUID refuses an environment with no secure random source", () => {
  expect(() => createClientUUID({})).toThrow(
    "Secure random generation is unavailable",
  );
});

for (const fallback of [false, true]) {
  test(`real training UUID: ${fallback ? "getRandomValues fallback" : "native"}, Correct/Wrong, retry and Undo`, async ({
    page,
    request,
  }) => {
    if (fallback)
      await page.addInitScript(() => {
        Object.defineProperty(globalThis.crypto, "randomUUID", {
          value: undefined,
          configurable: true,
        });
      });
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const nickname = `uuid${createClientUUID().replaceAll("-", "").slice(0, 10)}`;
    const credentials = { nickname, password: "UUIDRegression2026!" };
    const registered = await request.post("/api/v1/auth/register", {
      data: credentials,
    });
    expect(registered.status()).toBe(201);
    const { access_token: token } = await registered.json();
    const headers = { Authorization: `Bearer ${token}` };
    const folderResponse = await request.post("/api/v1/folders", {
      headers,
      data: { title: "UUID regression", template_key: "english_words" },
    });
    expect(folderResponse.status()).toBe(201);
    const folder = await folderResponse.json();
    const materialResponse = await request.post(
      `/api/v1/folders/${folder.id}/materials`,
      {
        headers,
        data: {
          values: { foreign: "hello", native: "привет" },
          difficulty: "easy",
        },
      },
    );
    expect(materialResponse.status()).toBe(201);
    await page.goto("/");
    await page.getByLabel("Username", { exact: true }).fill(nickname);
    await page
      .getByLabel("Password", { exact: true })
      .fill(credentials.password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "My library." }),
    ).toBeVisible();
    await page
      .getByRole("link", { name: "UUID regression", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Train folder", exact: true })
      .click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Start training", exact: true })
      .click();
    await expect(page.locator(".lead-question")).toBeVisible();
    expect(await page.evaluate(() => typeof crypto.randomUUID)).toBe(
      fallback || !(await page.evaluate(() => isSecureContext))
        ? "undefined"
        : "function",
    );
    const bodies: Record<string, unknown>[] = [];
    let loseResponse = true;
    await page.route("**/api/v1/training/sessions/*/actions", async (route) => {
      const body = route.request().postDataJSON();
      expect(body.command_id).toMatch(uuidV4);
      expect(body.presentation_id).toMatch(uuidV4);
      expect(typeof body.expected_version).toBe("number");
      bodies.push(body);
      if (loseResponse) {
        loseResponse = false;
        // Commit the real request, then simulate loss of its response.
        const committed = await route.fetch();
        expect(committed.status()).toBe(200);
        await route.abort("failed");
      } else await route.continue();
    });
    await page.getByRole("button", { name: "Correct", exact: true }).click();
    await expect(
      page.getByText("Your connection was interrupted.", { exact: false }),
    ).toBeVisible();
    const retried = page.waitForResponse(
      (r) => r.url().endsWith("/actions") && r.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Try again", exact: true }).click();
    expect((await retried).status()).toBe(200);
    expect(bodies[1]).toEqual(bodies[0]);
    await expect(
      page.getByRole("button", { name: "Wrong", exact: true }),
    ).toBeEnabled();
    const wrong = page.waitForResponse(
      (r) => r.url().endsWith("/actions") && r.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Wrong", exact: true }).click();
    expect((await wrong).status()).toBe(200);
    expect(bodies[2].command_id).not.toBe(bodies[0].command_id);
    const undo = page.waitForResponse((r) =>
      r.url().endsWith("/training/combined/undo"),
    );
    await page
      .getByRole("button", { name: "Undo last answer", exact: true })
      .click();
    const undone = await undo;
    expect(undone.status()).toBe(200);
    expect(undone.request().postDataJSON().command_id).toMatch(uuidV4);
    expect(errors).toEqual([]);
  });
}
