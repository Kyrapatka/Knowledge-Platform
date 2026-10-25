import { useRef, useEffect, useState } from "react";
import type { Folder } from "../types";
import { useLibraryPreferences } from "./preferences-context";
function SectionCheck({
  checked,
  partial,
  onChange,
  label,
}: {
  checked: boolean;
  partial: boolean;
  onChange: () => void;
  label: string;
}) {
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (ref.current) ref.current.indeterminate = partial;
  }, [partial]);
  return (
    <input
      ref={ref}
      type="checkbox"
      aria-label={label}
      checked={checked}
      onChange={onChange}
    />
  );
}
export function SectionFolderSelector({
  folders,
  value,
  onChange,
  disabled = false,
}: {
  folders: Folder[];
  value: string[];
  onChange: (ids: string[]) => void;
  disabled?: boolean;
}) {
  const { preferences } = useLibraryPreferences();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState<string | null>(null);
  const [collapsed, setCollapsed] = useState<string[]>([]);
  const groups = [
    ...preferences.categories.map((c) => ({ id: c.id, name: c.name })),
    { id: "unsectioned", name: "Без секции" },
  ]
    .map((group) => ({
      ...group,
      folders: folders.filter((f) => {
        const id = preferences.folders[f.id]?.categoryId;
        return group.id === "unsectioned"
          ? !preferences.categories.some((c) => c.id === id)
          : id === group.id;
      }),
    }))
    .filter((g) => g.folders.length);
  const toggle = (ids: string[]) =>
    onChange(
      ids.every((id) => value.includes(id))
        ? value.filter((id) => !ids.includes(id))
        : [...new Set([...value, ...ids])],
    );
  const search = query.trim().toLocaleLowerCase();
  return (
    <fieldset
      className={`section-folder-selector ${active ? "drilled" : ""}`}
      disabled={disabled}
    >
      <legend>Choose folders</legend>
      <label>
        Search folders
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </label>
      {active && (
        <button
          type="button"
          className="button selector-back"
          onClick={() => setActive(null)}
        >
          ← Sections
        </button>
      )}
      {groups
        .filter(
          (g) =>
            g.name.toLocaleLowerCase().includes(search) ||
            g.folders.some((f) => f.title.toLocaleLowerCase().includes(search)),
        )
        .map((g) => {
          const shown = g.folders.filter(
            (f) =>
              g.name.toLocaleLowerCase().includes(search) ||
              f.title.toLocaleLowerCase().includes(search),
          );
          const count = g.folders.filter((f) => value.includes(f.id)).length;
          const closed = collapsed.includes(g.id) && !search;
          return (
            <section
              key={g.id}
              className={`selector-section ${active === g.id ? "active" : ""}`}
            >
              <div className="selector-group-heading">
                <SectionCheck
                  label={`Select section ${g.name}`}
                  checked={count === g.folders.length}
                  partial={count > 0 && count < g.folders.length}
                  onChange={() => toggle(g.folders.map((f) => f.id))}
                />
                <button
                  type="button"
                  className="selector-desktop-group"
                  aria-expanded={!closed}
                  onClick={() =>
                    setCollapsed(
                      closed
                        ? collapsed.filter((id) => id !== g.id)
                        : [...collapsed, g.id],
                    )
                  }
                >
                  {closed ? "▸" : "▾"} {g.name} · {count} selected
                </button>
                <button
                  type="button"
                  className="selector-mobile-group"
                  onClick={() => setActive(g.id)}
                >
                  {g.name} · {count} selected →
                </button>
              </div>
              <div
                className={`selector-folder-list ${closed ? "collapsed" : ""}`}
              >
                {shown.map((f) => (
                  <label key={f.id}>
                    <input
                      type="checkbox"
                      checked={value.includes(f.id)}
                      onChange={() => toggle([f.id])}
                    />
                    <span>
                      {f.title}
                      <small>{f.material_count} questions</small>
                    </span>
                  </label>
                ))}
              </div>
            </section>
          );
        })}
      {!groups.length && <p>No interview folders yet.</p>}
    </fieldset>
  );
}
