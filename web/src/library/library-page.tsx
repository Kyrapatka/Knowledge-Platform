import {
  ArrowRight,
  BookOpen,
  Check,
  Clock3,
  FolderOpen,
  Plus,
  Search,
  TrendingUp,
  Upload,
} from "lucide-react";
import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { readStored } from "../api";
import { useAuth } from "../auth";
import { FolderEditor } from "../editors";
import { ImportFolder } from "../import-folder";
import { type SavedTraining } from "../training";
import { Empty, ErrorBox, Spinner } from "../ui";

import { useMemo } from "react";
import { SectionList } from "./sections";
import { useLibraryScroll } from "./use-library-scroll";
import { FolderBulkActions } from "./folder-bulk-actions";
import { FolderSection } from "./folder-section";
import { CategoryDialog } from "./category-dialog";
import { useLibrary } from "./context";
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
  const { preferences, dispatch, storageFailed } = useLibraryPreferences();
  const [params, setParams] = useSearchParams();
  const sectionsView = params.get("view") === "sections";
  useLibraryScroll(!loading && !!data);
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
  const favorites = useMemo(
    () => folders.filter((f) => preferences.folders[f.id]?.favorite),
    [folders, preferences.folders],
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
      <div
        className="tabs library-tabs"
        role="tablist"
        aria-label="Library views"
      >
        <button
          role="tab"
          aria-selected={!sectionsView}
          className={!sectionsView ? "active" : ""}
          onClick={() => {
            setQuery("");
            setParams({});
          }}
        >
          All folders
        </button>
        <button
          role="tab"
          aria-selected={sectionsView}
          className={sectionsView ? "active" : ""}
          onClick={() => {
            setQuery("");
            setParams({ view: "sections" });
          }}
        >
          Sections
        </button>
      </div>
      <div className="section-top">
        <div className="section-title">
          <h2>{sectionsView ? "Your sections" : "Your collections"}</h2>
          <span className="count-pill">{data.folders.length}</span>
        </div>
        <div className="search-input">
          <Search size={16} />
          <input
            aria-label={sectionsView ? "Filter sections" : "Filter folders"}
            placeholder={
              sectionsView ? "Find a section..." : "Find a folder..."
            }
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
      </div>
      {!sectionsView && (
        <div className="collection-controls">
          <div className="tabs" aria-label="Folder types">
            {[
              ["all", "All types"],
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
          <span className="selection-hint">
            Select folders for bulk actions
          </span>
        </div>
      )}
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
            <option value="title-asc">Title A-Z</option>
            <option value="title-desc">Title Z-A</option>
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
          New section
        </button>
      </div>
      {storageFailed && (
        <ErrorBox error="Browser storage is unavailable. Your organization changes will last until this page is closed." />
      )}
      {!sectionsView && (
        <FolderBulkActions
          selected={selected}
          setSelected={setSelected}
          shown={folders}
        />
      )}
      {!sectionsView && !!favorites.length && (
        <FolderSection
          title="Favorites"
          folders={favorites}
          selected={selected}
          toggle={toggle}
        />
      )}
      {sectionsView ? (
        <SectionList query={query} />
      ) : (
        <FolderSection
          title="All folders"
          folders={folders}
          selected={selected}
          toggle={toggle}
        />
      )}
      {!sectionsView && !folders.length && (
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
            : "Collect vocabulary, interview questions or formulas. We will help you remember them."}
        </Empty>
      )}
      {categoryEditor && (
        <CategoryDialog
          category={categoryEditor === "new" ? undefined : categoryEditor}
          onClose={() => setCategoryEditor(null)}
        />
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
