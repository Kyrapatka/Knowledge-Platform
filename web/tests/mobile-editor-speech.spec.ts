import { test, expect, type Page } from "@playwright/test";
import { initialConfig } from "../src/fields";

async function mockLibrary(page: Page) {
  const folder = {
    id: "folder",
    title: "Everyday words",
    description: "",
    template_key: "english_words",
    config: initialConfig("english_words"),
    config_version: 1,
    training_config_version: 1,
    training_config: { default_algorithm_key: "english_basic", pool_size: 7 },
    topics: [],
    material_count: 1,
    due_count: 0,
    learning_count: 0,
    completed_count: 0,
    selected_plan: null,
  };
  const material = {
    id: "material",
    folder_id: "folder",
    difficulty: "medium",
    values: {
      foreign: "serendipity",
      native: "случайная удача",
      example: "It was serendipity.",
      transcription: "/ˌser.ənˈdɪp.ə.ti/",
    },
    metadata: { topic: "Daily life" },
    progress: null,
  };
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    let body: unknown;
    if (path.endsWith("/auth/browser/refresh"))
      body = {
        access_token: "preview",
        user: { id: "preview", nickname: "Preview", status: "active" },
      };
    else if (path.endsWith("/library"))
      body = {
        folders: [folder],
        totals: {
          folder_count: 1,
          material_count: 1,
          due_count: 0,
          learning_count: 0,
          completed_count: 0,
        },
      };
    else if (path.endsWith("/materials/material")) body = material;
    else if (path.endsWith("/materials"))
      body = {
        items: [material],
        total: 1,
        limit: 50,
        offset: 0,
        topics: [],
        selected_plan: null,
        plans: [],
      };
    else
      return route.fulfill({
        status: 404,
        json: { error: "Unexpected preview route" },
      });
    return route.fulfill({ json: body });
  });
}

async function openDetails(page: Page) {
  await page.goto("/folders/folder");
  await page.locator(".material-row").click();
  await expect(
    page.getByRole("dialog", { name: "Material details" }),
  ).toBeVisible();
}

test("pronunciation needs a click, uses English and can stop or report voice errors", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const calls: { text: string; lang: string }[] = [];
    let active: SpeechSynthesisUtterance | undefined;
    Object.defineProperty(window, "speechSynthesis", {
      value: {
        getVoices: () => [],
        cancel: () => {
          active = undefined;
        },
        speak: (utterance: SpeechSynthesisUtterance) => {
          calls.push({ text: utterance.text, lang: utterance.lang });
          active = utterance;
        },
      },
      configurable: true,
    });
    Object.assign(window, {
      pronunciationCalls: calls,
      failPronunciation: () =>
        active?.onerror?.({
          error: "voice-unavailable",
        } as SpeechSynthesisErrorEvent),
    });
  });
  await mockLibrary(page);
  await openDetails(page);
  expect(
    await page.evaluate(
      () =>
        (window as unknown as { pronunciationCalls: unknown[] })
          .pronunciationCalls,
    ),
  ).toEqual([]);
  await page.getByRole("button", { name: "Listen to pronunciation" }).click();
  expect(
    await page.evaluate(
      () =>
        (window as unknown as { pronunciationCalls: unknown[] })
          .pronunciationCalls,
    ),
  ).toEqual([{ text: "serendipity", lang: "en-US" }]);
  await page.getByRole("button", { name: "Stop pronunciation" }).click();
  await expect(
    page.getByRole("button", { name: "Listen to pronunciation" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Listen to pronunciation" }).click();
  await page.evaluate(() =>
    (
      window as unknown as { failPronunciation: () => void }
    ).failPronunciation(),
  );
  await expect(page.locator(".pronunciation-status")).toContainText(
    "Could not play pronunciation",
  );
});

test("unsupported speech gives a readable message without breaking the card", async ({
  page,
}) => {
  await page.addInitScript(() => {
    Object.defineProperty(window, "SpeechSynthesisUtterance", {
      value: undefined,
      configurable: true,
    });
  });
  await mockLibrary(page);
  await openDetails(page);
  await page.getByRole("button", { name: "Listen to pronunciation" }).click();
  await expect(page.locator(".pronunciation-status")).toContainText(
    "not available",
  );
  await expect(page.getByRole("button", { name: "Edit word" })).toBeVisible();
});

test("mobile editor keeps a focused field and Save visible above a shrinking visual viewport", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => {
    const viewport = new EventTarget();
    Object.assign(viewport, { height: 844, offsetTop: 0 });
    Object.defineProperty(window, "visualViewport", {
      value: viewport,
      configurable: true,
    });
    Object.assign(window, {
      resizeKeyboardViewport: (height: number, offsetTop: number) => {
        Object.assign(viewport, { height, offsetTop });
        viewport.dispatchEvent(new Event("resize"));
      },
    });
  });
  await mockLibrary(page);
  await openDetails(page);
  const edit = page.getByRole("button", { name: "Edit word" });
  await expect.poll(async () => (await edit.boundingBox())!.height).toBeGreaterThanOrEqual(44);
  await edit.click();
  const field = page.getByRole("textbox", { name: "Example optional", exact: true });
  await field.fill("The word stays visible while I type.");
  for (const height of [430, 330]) {
  await page.evaluate((height) =>
    (
      window as unknown as {
        resizeKeyboardViewport: (height: number, offset: number) => void;
      }
    ).resizeKeyboardViewport(height, 32), height,
  );
  await expect(page.locator("dialog")).toHaveClass(/keyboard-open/);
  await expect
    .poll(async () => {
      const input = (await field.boundingBox())!;
      const save = (await page
        .getByRole("button", { name: "Save material", exact: true })
        .boundingBox())!;
      return input.y >= save.y + save.height && input.y + input.height <= height + 32;
    })
    .toBe(true);
  }
  expect(await field.evaluate((el) => getComputedStyle(el).fontSize)).toBe(
    "16px",
  );
  await field.press("End");
  await field.pressSequentially(" More.");
  await expect(field).toHaveValue("The word stays visible while I type. More.");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({ path: "test-results/mobile-editor-keyboard.png" });
  await page.evaluate(() =>
    (
      window as unknown as {
        resizeKeyboardViewport: (height: number, offset: number) => void;
      }
    ).resizeKeyboardViewport(844, 0),
  );
  await expect(page.locator("dialog")).not.toHaveClass(/keyboard-open/);
  await expect(field).toHaveValue("The word stays visible while I type. More.");
});

test("training pronunciation does not reveal a native-side answer or flip with Space", async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("knowledge:training:preview", JSON.stringify({
      sources: [{ folder_id: "folder" }], session_ids: ["session"],
    }));
    const calls: string[] = [];
    Object.defineProperty(window, "speechSynthesis", { value: {
      getVoices: () => [], cancel: () => {},
      speak: (utterance: SpeechSynthesisUtterance) => calls.push(utterance.text),
    }, configurable: true });
    Object.assign(window, { pronunciationCalls: calls });
  });
  await mockLibrary(page);
  await page.route("**/api/v1/training/combined/current", async (route) => route.fulfill({ json: {
    sessions: [{ session: { id: "session" } }],
    summary: { correct: 0, wrong: 0, materials_reviewed: 0 },
    current: { session_id: "session", algorithm_key: "english_basic", presentation: {
      id: "card", material_id: "material", folder_id: "folder", kind: "stage", stage: 1,
      direction: "native", question: [{ key: "native", value: "случайная удача" }],
      answer: [{ key: "foreign", value: "serendipity" }], foreign_word: "serendipity",
      required_correct: 3, consecutive_correct: 0, progress_version: 1,
    } },
  } }));
  await page.goto("/train");
  await expect(page.getByRole("button", { name: "Flip to answer" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Listen to pronunciation" })).toHaveCount(0);
  await expect(page.locator(".answer-fields")).not.toContainText("serendipity");
  await page.getByRole("button", { name: "Flip to answer" }).click();
  const listen = page.getByRole("button", { name: "Listen to pronunciation" });
  await listen.focus();
  await page.keyboard.press("Space");
  await expect(page.getByRole("button", { name: "Stop pronunciation" })).toBeVisible();
  await expect(page.locator(".flip-scene")).toHaveClass(/is-flipped/);
  expect(await page.evaluate(() => (window as unknown as { pronunciationCalls: string[] }).pronunciationCalls)).toEqual(["serendipity"]);
  await page.getByRole("button", { name: "Flip to question" }).click();
  await expect(page.getByRole("button", { name: "Listen to pronunciation" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Stop pronunciation" })).toHaveCount(0);
});
