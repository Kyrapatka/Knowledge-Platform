import {
  ArrowRight,
  BookOpen,
  Check,
  ChevronRight,
  Clock3,
  FolderOpen,
  Plus,
  Search,
  TrendingUp,
  Upload,
} from "lucide-react";
import { useLayoutEffect, useState } from "react";
import { useNavigate, useNavigationType } from "react-router-dom";
import { readStored } from "../api";
import { useAuth } from "../auth";
import { FolderEditor } from "../editors";
import { ImportFolder } from "../import-folder";
import { type SavedTraining } from "../training";
import { Empty, ErrorBox, Spinner } from "../ui";

import { useMemo } from "react";
import { CategoryDialog } from "./category-dialog";
import { useLibrary } from "./context";
import { FolderCard } from "./folder-card";
import {
  selectFolders,
  type Category,
  type LibraryPreferences,
} from "./preferences";
import { useLibraryPreferences } from "./preferences-context";
import { Metric } from "./presentation";
export function LibraryPage() {
  const navigate = useNavigate();
  const { data, loading, error, reload, launch, notify } = useLibrary();
  const { user } = useAuth();
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("all");
  const [selected, setSelected] = useState<string[]>([]);
  const [create, setCreate] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const { preferences, dispatch, storageFailed, libraryScroll } =
    useLibraryPreferences();
  const navigationType = useNavigationType();
  useLayoutEffect(() => {
    if (loading || !data) return;
    if (navigationType === "POP")
      window.scrollTo({ top: libraryScroll.current, behavior: "instant" });
    const remember = () => {
      libraryScroll.current = window.scrollY;
    };
    window.addEventListener("scroll", remember, { passive: true });
    return () => window.removeEventListener("scroll", remember);
  }, [loading, !!data, navigationType, libraryScroll]);
  const [categoryEditor, setCategoryEditor] = useState<Category | "new" | null>(
    null,
  );
  const [saved] = useState(() =>
    readStored<SavedTraining>(`knowledge:training:${user.id}`),
  );
  const folders = useMemo(
    () => selectFolders(data?.folders || [], preferences, query, filter),
    [data?.folders, preferences, query, filter],
  );
  const groups = useMemo(
    () => [
      ...preferences.categories.map((c) => ({
        category: c,
        folders: folders.filter(
          (f) => preferences.folders[f.id]?.categoryId === c.id,
        ),
      })),
      {
        category: null,
        folders: folders.filter((f) => !preferences.folders[f.id]?.categoryId),
      },
    ],
    [folders, preferences],
  );
  if (loading) return <Spinner />;
  if (error) return <ErrorBox error={error} retry={reload} />;
  if (!data) return null;
  function toggle(id: string) {
    setSelected((s) =>
      s.includes(id) ? s.filter((x) => x !== id) : [...s, id],
    );
  }
  return (
    <>
      <div className="page-heading">
        <div>
          <div className="eyebrow">A SPACE FOR WHAT YOU KNOW</div>
          <h1>
            My library<span className="heading-dot">.</span>
          </h1>
          <p>A collection of ideas. A little more yours, every day.</p>
        </div>
        <div className="page-heading-actions">
          <button className="button" onClick={() => setImportOpen(true)}>
            <Upload size={17} />
            Import folder
          </button>
          <button className="button primary" onClick={() => setCreate(true)}>
            <Plus size={17} />
            Create folder
          </button>
        </div>
      </div>
      <section className="library-overview">
        <div className="overview-copy">
          <span className="eyebrow">
            <span className="status-dot" /> KEEP YOUR MOMENTUM
          </span>
          <h2>
            {data.totals.due_count
              ? "A good day to make it stick."
              : data.totals.material_count
                ? "Your next little breakthrough."
                : "Big things start small."}
          </h2>
          <p>
            {data.totals.due_count
              ? `${data.totals.due_count} ${data.totals.due_count === 1 ? "material is" : "materials are"} ready for a fresh look.`
              : data.totals.material_count
                ? "Bring your knowledge back into focus. One card at a time."
                : "Add your first folder. Give your curiosity a place to grow."}
          </p>
          <button
            className="button primary"
            disabled={!data.folders.length}
            onClick={() =>
              saved?.sources?.length
                ? navigate("/train")
                : launch(
                    saved?.sources
                      ?.map((s) => s.folder_id)
                      .filter((id) => data.folders.some((f) => f.id === id)) ||
                      data.folders.map((f) => f.id),
                  )
            }
          >
            Start training
            <ArrowRight size={17} />
          </button>
        </div>
        <div className="overview-art" aria-hidden="true">
          <div className="mini-card mini-card-back" />
          <div className="mini-card mini-card-front">
            <span className="mini-star">✧</span>
            <span>KNOW A LITTLE MORE.</span>
            <div className="mini-line" />
            <div className="mini-line short" />
            <div className="mini-card-bottom">
              <span />
              <Check size={15} />
            </div>
          </div>
          <span className="art-spark spark-one">+</span>
          <span className="art-spark spark-two">✧</span>
        </div>
      </section>
      <div className="library-metrics">
        <Metric
          label="Total materials"
          value={data.totals.material_count}
          icon={<LayersIcon />}
        />
        <Metric
          label="Ready to review"
          value={data.totals.due_count}
          icon={<Clock3 size={17} />}
          accent
        />
        <Metric
          label="In progress"
          value={data.totals.learning_count}
          icon={<TrendingUp size={17} />}
        />
        <Metric
          label="Your folders"
          value={data.totals.folder_count}
          icon={<FolderOpen size={17} />}
        />
      </div>
      <div className="section-top">
        <div className="section-title">
          <h2>Your collections</h2>
          <span className="count-pill">{data.folders.length}</span>
        </div>
        <div className="search-input">
          <Search size={16} />
          <input
            aria-label="Filter folders"
            placeholder="Find a folder…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
      </div>
      <div className="collection-controls">
        <div className="tabs" aria-label="Folder types">
          {[
            ["all", "All folders"],
            ["english_words", "Languages"],
            ["interview_questions", "Interview prep"],
            ["formulas", "Formulas"],
          ].map(([key, label]) => (
            <button
              key={key}
              className={filter === key ? "active" : ""}
              onClick={() => setFilter(key)}
            >
              {label}
            </button>
          ))}
        </div>
        <span className="selection-hint">Select folders to train together</span>
      </div>
      <div className="library-toolbar">
        <label>
          Sort{" "}
          <select
            aria-label="Sort folders"
            value={preferences.sort}
            onChange={(e) =>
              dispatch({
                type: "sort",
                sort: e.target.value as LibraryPreferences["sort"],
              })
            }
          >
            <option value="recent">Recently used</option>
            <option value="created">Recently created</option>
            <option value="title-asc">Title A�Z</option>
            <option value="title-desc">Title Z�A</option>
            <option value="count-desc">Most materials</option>
            <option value="count-asc">Fewest materials</option>
            <option value="due">Most ready to review</option>
          </select>
        </label>
        <label className="check-label">
          <input
            type="checkbox"
            checked={preferences.favoritesFirst}
            onChange={(e) =>
              dispatch({ type: "favorites-first", enabled: e.target.checked })
            }
          />
          Favorites first
        </label>
        <button
          className="button small"
          onClick={() => setCategoryEditor("new")}
        >
          New category
        </button>
      </div>
      {storageFailed && (
        <ErrorBox error="Browser storage is unavailable. Your organization changes will last until this page is closed." />
      )}
      {groups.map(({ category, folders: grouped }) =>
        !category && !grouped.length ? null : (
          <section
            className="category-section"
            key={category?.id || "uncategorized"}
          >
            <div className="category-heading">
              {category ? (
                <button
                  className="text-button category-toggle"
                  aria-expanded={!category.collapsed}
                  onClick={() =>
                    dispatch({
                      type: "category",
                      category: { ...category, collapsed: !category.collapsed },
                    })
                  }
                >
                  <ChevronRight
                    className={category.collapsed ? "" : "expanded"}
                    size={18}
                  />
                  {category.name}
                  <span className="count-pill">{grouped.length}</span>
                </button>
              ) : (
                <h3>
                  {preferences.categories.length
                    ? "Uncategorized"
                    : "All collections"}{" "}
                  <span className="count-pill">{grouped.length}</span>
                </h3>
              )}
              {category && (
                <button
                  className="text-button"
                  aria-label={`Edit category ${category.name}`}
                  onClick={() => setCategoryEditor(category)}
                >
                  Edit
                </button>
              )}
            </div>
            {!category?.collapsed && (
              <div className="folder-grid">
                {grouped.map((folder) => (
                  <FolderCard
                    key={folder.id}
                    folder={folder}
                    selected={selected.includes(folder.id)}
                    toggle={() => toggle(folder.id)}
                  />
                ))}
                {!grouped.length && (
                  <p className="muted">
                    No matching folders in this category. Use Customize on a
                    folder to assign it.
                  </p>
                )}
              </div>
            )}
          </section>
        ),
      )}
      {!folders.length && (
        <Empty
          icon={<FolderOpen size={28} />}
          title={
            data.folders.length
              ? "No folders found"
              : "Your library starts here"
          }
          action={
            <button className="button primary" onClick={() => setCreate(true)}>
              <Plus size={17} />
              Create your first folder
            </button>
          }
        >
          {data.folders.length
            ? "Try another name or choose a different type."
            : "Collect vocabulary, interview questions or formulas. We�ll help you remember them."}
        </Empty>
      )}
      {categoryEditor && (
        <CategoryDialog
          category={categoryEditor === "new" ? undefined : categoryEditor}
          onClose={() => setCategoryEditor(null)}
        />
      )}
      {!!selected.length && (
        <div className="selection-bar">
          <span>
            <Check size={16} />
            {selected.length} folders selected
          </span>
          <button className="text-button" onClick={() => setSelected([])}>
            Clear
          </button>
          <button className="button primary" onClick={() => launch(selected)}>
            Train together
            <ArrowRight size={16} />
          </button>
        </div>
      )}
      {create && (
        <FolderEditor
          onClose={() => setCreate(false)}
          onSaved={() => {
            setCreate(false);
            void reload();
          }}
        />
      )}
      {importOpen && (
        <ImportFolder
          onClose={() => setImportOpen(false)}
          onImported={(result) => {
            setImportOpen(false);
            void reload();
            const kind =
              result.template === "english_words"
                ? "English words"
                : "interview questions";
            notify(
              `Folder imported successfully — ${result.items_created} ${kind} added.`,
            );
          }}
        />
      )}
    </>
  );
}
function LayersIcon() {
  return <BookOpen size={17} />;
}
