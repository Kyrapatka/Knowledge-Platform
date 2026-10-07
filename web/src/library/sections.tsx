import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { MoreHorizontal, Search } from "lucide-react";
import { Empty, ErrorBox, Spinner } from "../ui";
import type { Category } from "./preferences";
import { selectFolders } from "./preferences";
import { CategoryDialog } from "./category-dialog";
import { FolderIcon } from "./appearance";
import { FolderBulkActions } from "./folder-bulk-actions";
import { FolderCard } from "./folder-card";
import { useLibrary } from "./context";
import { useLibraryPreferences } from "./preferences-context";
import { useLibraryScroll } from "./use-library-scroll";
export function SectionList({ query }: { query: string }) {
  const { data } = useLibrary();
  const { preferences } = useLibraryPreferences();
  const [editing, setEditing] = useState<Category | null>(null);
  const needle = query.trim().toLocaleLowerCase();
  const groups = [
    ...preferences.categories,
    { id: "uncategorized", name: "Uncategorized", collapsed: false },
  ];
  return (
    <>
      <div className="section-grid">
        {groups
          .filter((section) =>
            section.name.toLocaleLowerCase().includes(needle),
          )
          .map((section) => {
            const count = (data?.folders || []).filter(
              (f) =>
                (preferences.folders[f.id]?.categoryId || "uncategorized") ===
                section.id,
            ).length;
            return (
              <article
                className={`section-card folder-color-${section.color || "default"}`}
                key={section.id}
              >
                <Link
                  className="section-open"
                  to={`/sections/${encodeURIComponent(section.id)}`}
                >
                  <span className="folder-icon">
                    <FolderIcon
                      kind="section"
                      appearance={{ icon: section.icon || "folder" }}
                    />
                  </span>
                  <span className="section-card-text">
                    <strong title={section.name}>{section.name}</strong>
                    <small>{count} folders</small>
                  </span>
                </Link>
                {section.id !== "uncategorized" && (
                  <button
                    className="icon-button"
                    aria-label={`Edit section ${section.name}`}
                    onClick={() => setEditing(section)}
                  >
                    <MoreHorizontal size={20} />
                  </button>
                )}
              </article>
            );
          })}
      </div>
      {editing && (
        <CategoryDialog category={editing} onClose={() => setEditing(null)} />
      )}
    </>
  );
}
export function SectionPage() {
  const { sectionID = "" } = useParams();
  const { data, loading, error, reload } = useLibrary();
  const { preferences } = useLibraryPreferences();
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const [editing, setEditing] = useState(false);
  const section =
    sectionID === "uncategorized"
      ? { id: sectionID, name: "Uncategorized", collapsed: false }
      : preferences.categories.find((c) => c.id === sectionID);
  const members = (data?.folders || []).filter(
    (f) =>
      (preferences.folders[f.id]?.categoryId || "uncategorized") === sectionID,
  );
  const folders = selectFolders(members, preferences, query, "all");
  useLibraryScroll(!loading && !!data);
  if (loading) return <Spinner />;
  if (error) return <ErrorBox error={error} retry={reload} />;
  if (!section)
    return (
      <Empty
        title="Section not found"
        action={<Link to="/?view=sections">Back to sections</Link>}
      >
        Its folders remain in your library.
      </Empty>
    );
  const toggle = (id: string) =>
    setSelected((ids) =>
      ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id],
    );
  return (
    <>
      <Link className="back-link" to="/?view=sections">
        ← Sections
      </Link>
      <header className="page-heading section-page-heading">
        <div>
          <span className="eyebrow">Library / Sections</span>
          <h1>{section.name}</h1>
          <p>{members.length} folders</p>
        </div>
        {sectionID !== "uncategorized" && (
          <button className="button" onClick={() => setEditing(true)}>
            Edit section
          </button>
        )}
      </header>
      <div className="library-toolbar">
        <label className="search-input">
          <Search size={16} />
          <input
            aria-label="Search in section"
            placeholder="Search in section"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
      </div>
      <FolderBulkActions
        selected={selected}
        setSelected={setSelected}
        shown={folders}
      />
      <div className="folder-grid">
        {folders.map((folder) => (
          <FolderCard
            key={folder.id}
            folder={folder}
            selected={selected.includes(folder.id)}
            toggle={() => toggle(folder.id)}
          />
        ))}
      </div>
      {!folders.length && (
        <Empty title="No folders here">
          Assign folders using Customize or choose another search.
        </Empty>
      )}
      {editing && (
        <CategoryDialog category={section} onClose={() => setEditing(false)} />
      )}
    </>
  );
}
