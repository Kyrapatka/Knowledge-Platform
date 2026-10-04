import { expect, test } from "@playwright/test";
import {
  defaultPreferences,
  libraryPreferencesStorage,
  parsePreferences,
  prunePreferences,
  selectFolders,
  updatePreferences,
} from "../src/library/preferences";
import type { Folder } from "../src/types";
const folder = (id: string, title: string, count = 0): Folder =>
  ({
    id,
    title,
    description: "",
    template_key: "interview_questions",
    created_at: "2026-01-01T00:00:00Z",
    material_count: count,
    due_count: 0,
  }) as Folder;
test("preferences defaults, invalid JSON, version and blocked storage", () => {
  for (const raw of [
    null,
    "{broken",
    "null",
    "[]",
    '{"version":2}',
    '{"version":1,"folders":null,"categories":[null,{}]}',
  ]) {
    expect(parsePreferences(raw)).toEqual(defaultPreferences());
  }
  const broken = {
    getItem: () => {
      throw new Error("blocked");
    },
    setItem: () => {
      throw new Error("full");
    },
  };
  expect(libraryPreferencesStorage.load("one", broken)).toEqual(
    defaultPreferences(),
  );
  expect(
    libraryPreferencesStorage.save("one", defaultPreferences(), broken),
  ).toBe(false);
});
test("preferences round trip is scoped to the signed-in user", () => {
  const values = new Map<string, string>();
  const storage = {
    getItem: (k: string) => values.get(k) ?? null,
    setItem: (k: string, v: string) => {
      values.set(k, v);
    },
  };
  const p = updatePreferences(defaultPreferences(), {
    type: "copied",
    id: "target",
    makeDefault: true,
  });
  expect(libraryPreferencesStorage.save("one", p, storage)).toBe(true);
  expect(libraryPreferencesStorage.load("one", storage)).toEqual(p);
  expect(libraryPreferencesStorage.load("two", storage)).toEqual(
    defaultPreferences(),
  );
});
test("category assignment/removal keeps folders and favorites; collapse persists", () => {
  let p = updatePreferences(defaultPreferences(), {
    type: "category",
    category: { id: "cat", name: "Interview", collapsed: true },
  });
  p = updatePreferences(p, {
    type: "folder",
    id: "one",
    patch: { categoryId: "cat", favorite: true, color: "blue", icon: "code" },
  });
  expect(parsePreferences(JSON.stringify(p)).categories[0].collapsed).toBe(
    true,
  );
  p = updatePreferences(p, {
    type: "category",
    category: { id: "cat", name: "Renamed", collapsed: false },
  });
  expect(p.categories).toEqual([
    { id: "cat", name: "Renamed", collapsed: false },
  ]);
  p = updatePreferences(p, { type: "remove-category", id: "cat" });
  expect(p.folders.one).toMatchObject({
    categoryId: null,
    favorite: true,
    color: "blue",
    icon: "code",
  });
  p = updatePreferences(p, {
    type: "folder",
    id: "one",
    patch: { favorite: false },
  });
  expect(p.folders.one.favorite).toBe(false);
});
test("deleted folders clear stale/default targets; one-off copy keeps the default", () => {
  let p = updatePreferences(defaultPreferences(), {
    type: "copied",
    id: "one",
    makeDefault: true,
  });
  p = updatePreferences(p, { type: "copied", id: "two", makeDefault: false });
  expect(p.defaultCopyFolderId).toBe("one");
  expect(p.recentCopyTargets).toEqual(["two", "one"]);
  p = updatePreferences(p, {
    type: "folder",
    id: "one",
    patch: { favorite: true },
  });
  p = prunePreferences(p, [folder("two", "Two")]);
  expect(p.defaultCopyFolderId).toBeNull();
  expect(p.recentCopyTargets).toEqual(["two"]);
  expect(p.folders.one).toBeUndefined();
});
test("folder sorting, category search and deterministic ties do not mutate API data", () => {
  const folders = [
    folder("a", "Zulu", 5),
    folder("b", "Alpha", 1),
    folder("c", "Beta", 9),
  ];
  let p = updatePreferences(defaultPreferences(), {
    type: "used",
    ids: ["b"],
    at: Date.parse("2026-10-04"),
  });
  expect(selectFolders(folders, p, "", "all").map((f) => f.id)).toEqual([
    "b",
    "a",
    "c",
  ]);
  for (const [sort, ids] of [
    ["title-asc", ["b", "c", "a"]],
    ["title-desc", ["a", "c", "b"]],
    ["count-desc", ["c", "a", "b"]],
    ["count-asc", ["b", "a", "c"]],
  ] as const) {
    expect(
      selectFolders(folders, { ...p, sort }, "", "all").map((f) => f.id),
    ).toEqual(ids);
  }
  p = updatePreferences(p, {
    type: "category",
    category: { id: "cat", name: "Coding", collapsed: false },
  });
  p = updatePreferences(p, {
    type: "folder",
    id: "c",
    patch: { categoryId: "cat" },
  });
  expect(selectFolders(folders, p, "CODING", "all").map((f) => f.id)).toEqual([
    "c",
  ]);
  expect(folders.map((f) => f.id)).toEqual(["a", "b", "c"]);
});

test("Favorites-first combines with sorting and can be disabled", () => {
  const folders = [folder("a", "Alpha", 1), folder("b", "Beta", 2)];
  let p = updatePreferences(defaultPreferences(), {
    type: "sort",
    sort: "title-asc",
  });
  p = updatePreferences(p, {
    type: "folder",
    id: "b",
    patch: { favorite: true },
  });
  expect(selectFolders(folders, p, "", "all").map((f) => f.id)).toEqual([
    "b",
    "a",
  ]);
  p = updatePreferences(p, { type: "favorites-first", enabled: false });
  expect(selectFolders(folders, p, "", "all").map((f) => f.id)).toEqual([
    "a",
    "b",
  ]);
});
