import type { Folder } from "../types";

export const colors = [
  "default",
  "gray",
  "blue",
  "cyan",
  "green",
  "yellow",
  "orange",
  "red",
  "purple",
  "pink",
] as const;
export const icons = [
  "default",
  "folder",
  "code",
  "brain",
  "book",
  "language",
  "database",
  "math",
  "graduation",
  "rocket",
] as const;
export const sorts = [
  "recent",
  "created",
  "title-asc",
  "title-desc",
  "count-desc",
  "count-asc",
  "due",
] as const;
export type FolderAppearance = {
  categoryId?: string | null;
  color?: (typeof colors)[number];
  icon?: (typeof icons)[number];
  favorite?: boolean;
  lastUsedAt?: number;
};
export type Category = {
  id: string;
  name: string;
  collapsed: boolean;
  icon?: FolderAppearance["icon"];
  color?: FolderAppearance["color"];
};
export type LibraryPreferences = {
  version: 1;
  favoritesFirst: boolean;
  categories: Category[];
  folders: Record<string, FolderAppearance>;
  defaultCopyFolderId: string | null;
  recentCopyTargets: string[];
  sort: (typeof sorts)[number];
};
export const defaultPreferences = (): LibraryPreferences => ({
  version: 1,
  favoritesFirst: true,
  categories: [],
  folders: {},
  defaultCopyFolderId: null,
  recentCopyTargets: [],
  sort: "recent",
});
const record = (v: unknown): v is Record<string, unknown> =>
  !!v && typeof v === "object" && !Array.isArray(v);
const validId = (v: unknown): v is string =>
  typeof v === "string" &&
  v.length > 0 &&
  v.length <= 128 &&
  !["__proto__", "constructor", "prototype"].includes(v);
const member = <T extends string>(keys: readonly T[], v: unknown): v is T =>
  typeof v === "string" && keys.includes(v as T);
export function parsePreferences(raw: string | null): LibraryPreferences {
  try {
    const value: unknown = JSON.parse(raw || "null");
    if (!record(value) || value.version !== 1) return defaultPreferences();
    const next = defaultPreferences();
    next.favoritesFirst = value.favoritesFirst !== false;
    if (Array.isArray(value.categories)) {
      const seen = new Set<string>();
      next.categories = value.categories
        .filter(record)
        .flatMap((c) => {
          if (
            !validId(c.id) ||
            seen.has(c.id) ||
            typeof c.name !== "string" ||
            !c.name.trim()
          )
            return [];
          seen.add(c.id);
          return [
            {
              id: c.id,
              name: c.name.trim().slice(0, 64),
              collapsed: c.collapsed === true,
              ...(member(icons, c.icon) ? { icon: c.icon } : {}),
              ...(member(colors, c.color) ? { color: c.color } : {}),
            },
          ];
        })
        .slice(0, 100);
    }
    const categoryIds = new Set(next.categories.map((c) => c.id));
    if (record(value.folders))
      for (const [id, p] of Object.entries(value.folders).slice(0, 5000)) {
        if (!validId(id) || !record(p)) continue;
        next.folders[id] = {
          categoryId:
            typeof p.categoryId === "string" && categoryIds.has(p.categoryId)
              ? p.categoryId
              : null,
          color: member(colors, p.color) ? p.color : "default",
          icon: member(icons, p.icon) ? p.icon : "default",
          favorite: p.favorite === true,
          lastUsedAt:
            typeof p.lastUsedAt === "number" &&
            Number.isFinite(p.lastUsedAt) &&
            p.lastUsedAt > 0
              ? p.lastUsedAt
              : undefined,
        };
      }
    next.defaultCopyFolderId = validId(value.defaultCopyFolderId)
      ? value.defaultCopyFolderId
      : null;
    next.recentCopyTargets = Array.isArray(value.recentCopyTargets)
      ? [...new Set(value.recentCopyTargets.filter(validId))].slice(0, 5)
      : [];
    if (member(sorts, value.sort)) next.sort = value.sort;
    return next;
  } catch {
    return defaultPreferences();
  }
}
type StorageLike = Pick<Storage, "getItem" | "setItem">;
const storageKey = (user: string) => `knowledge:library-preferences:${user}`;
export const libraryPreferencesStorage = {
  load(user: string, storage?: StorageLike): LibraryPreferences {
    try {
      return parsePreferences(
        (storage ?? window.localStorage).getItem(storageKey(user)),
      );
    } catch {
      return defaultPreferences();
    }
  },
  save(
    user: string,
    value: LibraryPreferences,
    storage?: StorageLike,
  ): boolean {
    try {
      (storage ?? window.localStorage).setItem(
        storageKey(user),
        JSON.stringify(value),
      );
      return true;
    } catch {
      return false;
    }
  },
};
export function prunePreferences(
  p: LibraryPreferences,
  folders: Pick<Folder, "id" | "template_key">[],
): LibraryPreferences {
  const alive = new Set(folders.map((f) => f.id));
  const targets = new Set(
    folders
      .filter((f) => f.template_key === "interview_questions")
      .map((f) => f.id),
  );
  const next = {
    ...p,
    folders: Object.fromEntries(
      Object.entries(p.folders).filter(([id]) => alive.has(id)),
    ),
    defaultCopyFolderId:
      p.defaultCopyFolderId && targets.has(p.defaultCopyFolderId)
        ? p.defaultCopyFolderId
        : null,
    recentCopyTargets: p.recentCopyTargets.filter((id) => targets.has(id)),
  };
  return JSON.stringify(next) === JSON.stringify(p) ? p : next;
}
export type PreferenceAction =
  | { type: "folder"; id: string; patch: FolderAppearance }
  | { type: "folders"; ids: string[]; patch: FolderAppearance }
  | { type: "category"; category: Category }
  | { type: "remove-category"; id: string }
  | { type: "favorites-first"; enabled: boolean }
  | { type: "sort"; sort: LibraryPreferences["sort"] }
  | { type: "used"; ids: string[]; at: number }
  | { type: "copied"; id: string; makeDefault: boolean }
  | { type: "clear-default" };
export function updatePreferences(
  p: LibraryPreferences,
  action: PreferenceAction,
): LibraryPreferences {
  switch (action.type) {
    case "folders":
      return {
        ...p,
        folders: {
          ...p.folders,
          ...Object.fromEntries(
            action.ids.map((id) => [id, { ...p.folders[id], ...action.patch }]),
          ),
        },
      };
    case "folder":
      return {
        ...p,
        folders: {
          ...p.folders,
          [action.id]: { ...p.folders[action.id], ...action.patch },
        },
      };
    case "category":
      return {
        ...p,
        categories: p.categories.some((c) => c.id === action.category.id)
          ? p.categories.map((c) =>
              c.id === action.category.id ? action.category : c,
            )
          : [...p.categories, action.category],
      };
    case "remove-category":
      return {
        ...p,
        categories: p.categories.filter((c) => c.id !== action.id),
        folders: Object.fromEntries(
          Object.entries(p.folders).map(([id, f]) => [
            id,
            f.categoryId === action.id ? { ...f, categoryId: null } : f,
          ]),
        ),
      };
    case "favorites-first":
      return { ...p, favoritesFirst: action.enabled };
    case "sort":
      return { ...p, sort: action.sort };
    case "used":
      return {
        ...p,
        folders: {
          ...p.folders,
          ...Object.fromEntries(
            action.ids.map((id) => [
              id,
              { ...p.folders[id], lastUsedAt: action.at },
            ]),
          ),
        },
      };
    case "copied":
      return {
        ...p,
        recentCopyTargets: [
          action.id,
          ...p.recentCopyTargets.filter((id) => id !== action.id),
        ].slice(0, 5),
        defaultCopyFolderId: action.makeDefault
          ? action.id
          : p.defaultCopyFolderId,
      };
    case "clear-default":
      return { ...p, defaultCopyFolderId: null };
  }
}
export function selectFolders(
  folders: Folder[],
  p: LibraryPreferences,
  query: string,
  template: string,
): Folder[] {
  const needle = query.trim().toLocaleLowerCase();
  const categories = new Map(p.categories.map((c) => [c.id, c.name]));
  const text = (f: Folder) =>
    [
      f.title,
      f.description,
      categories.get(p.folders[f.id]?.categoryId || "") || "",
    ]
      .join(" ")
      .toLocaleLowerCase();
  const timestamp = (f: Folder) =>
    p.folders[f.id]?.lastUsedAt || Date.parse(f.created_at) || 0;
  return folders
    .filter(
      (f) =>
        (template === "all" || f.template_key === template) &&
        text(f).includes(needle),
    )
    .sort((a, b) => {
      const favoriteOrder =
        Number(!!p.folders[b.id]?.favorite) -
        Number(!!p.folders[a.id]?.favorite);
      if (p.favoritesFirst && favoriteOrder) return favoriteOrder;
      let order = 0;
      switch (p.sort) {
        case "recent":
          order = timestamp(b) - timestamp(a);
          break;
        case "created":
          order =
            (Date.parse(b.created_at) || 0) - (Date.parse(a.created_at) || 0);
          break;
        case "title-asc":
          order = a.title.localeCompare(b.title);
          break;
        case "title-desc":
          order = b.title.localeCompare(a.title);
          break;
        case "count-desc":
          order = b.material_count - a.material_count;
          break;
        case "count-asc":
          order = a.material_count - b.material_count;
          break;
        case "due":
          order = b.due_count - a.due_count;
          break;
      }
      return order || a.id.localeCompare(b.id);
    });
}
