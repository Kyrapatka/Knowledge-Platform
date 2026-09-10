import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  Link,
  NavLink,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from "react-router-dom";
import {
  ArrowDownUp,
  ArrowRight,
  BookOpen,
  Check,
  ChevronRight,
  Clock3,
  FolderOpen,
  GraduationCap,
  LayoutGrid,
  ListFilter,
  LogOut,
  MoreHorizontal,
  Plus,
  Search,
  Settings2,
  Sparkles,
  TrendingUp,
  X,
  Zap,
} from "lucide-react";
import { api, errorText, readStored } from "./api";
import { useAuth } from "./auth";
import type {
  Folder,
  Library as LibraryData,
  Material,
  MaterialPage,
  Source,
} from "./types";
import { algorithmNames, materialTitle, templateNames, topicOf } from "./types";
import { DateValue, Empty, ErrorBox, Logo, Modal, Spinner } from "./ui";
import { FolderEditor, MaterialEditor, MaterialDetails } from "./editors";
import { TrainingPage, TrainingSetup, type SavedTraining } from "./training";
import { StatisticsPage } from "./statistics";
import { PlanSettings } from "./settings";

type LibraryContextValue = {
  data: LibraryData | null;
  loading: boolean;
  error: string;
  reload: () => Promise<void>;
  notify: (s: string) => void;
  launch: (ids: string[], topics?: string[], planId?: string) => void;
};
const LibraryContext = createContext<LibraryContextValue>(null!);
export const useLibrary = () => useContext(LibraryContext);
export function App() {
  const { user } = useAuth();
  const [data, setData] = useState<LibraryData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [toast, setToast] = useState("");
  const [setup, setSetup] = useState<{
    ids: string[];
    topics?: string[];
    planId?: string;
  } | null>(null);
  const reload = useCallback(async () => {
    setError("");
    try {
      setData(await api<LibraryData>("/library"));
    } catch (e) {
      setError(errorText(e));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    void reload();
  }, [reload, user.id]);
  useEffect(() => {
    if (toast) {
      const timer = setTimeout(() => setToast(""), 4500);
      return () => clearTimeout(timer);
    }
  }, [toast]);
  const value: LibraryContextValue = {
    data,
    loading,
    error,
    reload,
    notify: setToast,
    launch: (ids, topics, planId) => setSetup({ ids, topics, planId }),
  };
  return (
    <LibraryContext.Provider value={value}>
      <Routes>
        <Route path="/train" element={<TrainingPage />} />
        <Route
          path="*"
          element={
            <Shell>
              <Routes>
                <Route path="/" element={<LibraryPage />} />
                <Route path="/folders/:folderID" element={<FolderPage />} />
                <Route path="/training" element={<TrainingHome />} />
                <Route path="/statistics" element={<StatisticsPage />} />
                <Route
                  path="*"
                  element={
                    <Empty
                      title="A little off the path"
                      action={
                        <Link className="button primary" to="/">
                          Back to library
                        </Link>
                      }
                    >
                      This page doesn't exist.
                    </Empty>
                  }
                />
              </Routes>
            </Shell>
          }
        />
      </Routes>
      {setup && data && (
        <TrainingSetup
          initial={setup}
          folders={data.folders}
          onClose={() => setSetup(null)}
        />
      )}
      {toast && (
        <div className="toast" role="status">
          <Check size={17} />
          {toast}
          <button
            className="icon-button"
            aria-label="Dismiss notification"
            onClick={() => setToast("")}
          >
            <X size={15} />
          </button>
        </div>
      )}
    </LibraryContext.Provider>
  );
}
function Shell({ children }: { children: ReactNode }) {
  const { user, signOut } = useAuth();
  const [profileOpen, setProfileOpen] = useState(false);
  const { data, notify } = useLibrary();
  const location = useLocation();
  const today = new Date().toLocaleDateString("en", {
    timeZone: "Europe/Moscow",
    weekday: "short",
    month: "short",
    day: "numeric",
  });
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Link className="brand-link" to="/" aria-label="Knowledge home">
          <Logo />
        </Link>
        <span className="nav-label">WORKSPACE</span>
        <nav aria-label="Main navigation">
          <NavLink
            to="/"
            end
            className={({ isActive }) =>
              isActive || location.pathname.startsWith("/folders/")
                ? "active"
                : ""
            }
          >
            <LayoutGrid size={18} />
            <span>My library</span>
            {!!data?.totals.folder_count && (
              <span className="nav-count">{data.totals.folder_count}</span>
            )}
          </NavLink>
          <NavLink to="/training">
            <Zap size={18} />
            <span>Training</span>
          </NavLink>
          <NavLink to="/statistics">
            <TrendingUp size={18} />
            <span>Statistics</span>
          </NavLink>
        </nav>
        <div className="sidebar-bottom">
          <div className="quiet-note">
            <span className="tiny-stars">✧</span>
            <p>
              A little practice.
              <br />
              <strong>A lasting difference.</strong>
            </p>
          </div>
          <div className="profile">
            <span className="avatar">
              {user.nickname.slice(0, 2).toUpperCase()}
            </span>
            <div>
              <strong>{user.nickname}</strong>
              <span>Personal workspace</span>
            </div>
            <button
              className="icon-button"
              aria-label="Sign out"
              title="Sign out"
              onClick={() => {
                void signOut().catch((e) => notify(errorText(e)));
              }}
            >
              <LogOut size={16} />
            </button>
          </div>
        </div>
      </aside>
      <div className="main-shell">
        <header className="topbar">
          <div className="breadcrumbs">
            <span>Workspace</span>
            <ChevronRight size={13} />
            <span>
              {location.pathname === "/statistics"
                ? "Statistics"
                : location.pathname === "/training"
                  ? "Training"
                  : "My library"}
            </span>
          </div>
          <div className="topbar-right">
            <span className="today">{today}</span>
            <button
              className="topbar-avatar"
              aria-label="Open profile"
              onClick={() => setProfileOpen(true)}
            >
              {user.nickname[0].toUpperCase()}
            </button>
          </div>
        </header>
        <main className="main-content" key={location.pathname}>
          {children}
        </main>
        <footer className="page-footer">
          <span>Make it a little more familiar.</span>
          <span>
            <span className="status-dot" /> Your own pace. Your own progress.
          </span>
        </footer>
      </div>
      {profileOpen && (
        <Modal
          title="Your workspace"
          subtitle={user.nickname}
          onClose={() => setProfileOpen(false)}
        >
          <div className="form-body">
            <p className="muted">
              Your library and learning progress are saved to your account.
            </p>
            <button
              className="button"
              onClick={() => {
                void signOut().catch((e) => notify(errorText(e)));
              }}
            >
              <LogOut size={16} />
              Sign out
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
}
export function TypeIcon({ kind, size = 23 }: { kind: string; size?: number }) {
  return kind === "interview_questions" ? (
    <span className="type-glyph">&lt;/&gt;</span>
  ) : kind === "formulas" ? (
    <span className="type-glyph formula-glyph">ƒ</span>
  ) : (
    <BookOpen size={size} />
  );
}
function LibraryPage() {
  const navigate = useNavigate();
  const { data, loading, error, reload, launch } = useLibrary();
  const { user } = useAuth();
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("all");
  const [selected, setSelected] = useState<string[]>([]);
  const [create, setCreate] = useState(false);
  if (loading) return <Spinner />;
  if (error) return <ErrorBox error={error} retry={reload} />;
  if (!data) return null;
  const folders = data.folders.filter(
    (f) =>
      f.title.toLowerCase().includes(query.toLowerCase()) &&
      (filter === "all" || f.template_key === filter),
  );
  const saved = readStored<SavedTraining>(`knowledge:training:${user.id}`);
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
        <button className="button primary" onClick={() => setCreate(true)}>
          <Plus size={17} />
          Create folder
        </button>
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
      {folders.length ? (
        <div className="folder-grid">
          {folders.map((folder) => (
            <FolderCard
              key={folder.id}
              folder={folder}
              selected={selected.includes(folder.id)}
              toggle={() => toggle(folder.id)}
            />
          ))}
          {!query && filter === "all" && (
            <button className="new-folder-card" onClick={() => setCreate(true)}>
              <span>
                <Plus size={21} />
              </span>
              <strong>A new collection</strong>
              <p>Give your next interest a home.</p>
            </button>
          )}
        </div>
      ) : (
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
            : "Collect vocabulary, interview questions or formulas. We’ll help you remember them."}
        </Empty>
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
    </>
  );
}
function LayersIcon() {
  return <BookOpen size={17} />;
}
export function Metric({
  label,
  value,
  icon,
  accent,
}: {
  label: string;
  value: number | string;
  icon?: ReactNode;
  accent?: boolean;
}) {
  return (
    <div className={`metric ${accent ? "accent" : ""}`}>
      <span className="metric-label">
        {icon}
        {label}
      </span>
      <strong>
        {typeof value === "number" ? value.toLocaleString("en") : value}
      </strong>
    </div>
  );
}
function FolderCard({
  folder,
  selected,
  toggle,
}: {
  folder: Folder;
  selected: boolean;
  toggle: () => void;
}) {
  const { launch } = useLibrary();
  const navigate = useNavigate();
  return (
    <article
      className={`folder-card ${folder.template_key} ${selected ? "selected" : ""}`}
      onClick={(e) => {
        if (!(e.target as HTMLElement).closest("a, button, input, label"))
          navigate(`/folders/${folder.id}`);
      }}
    >
      <div className="folder-card-top">
        <span className="folder-icon">
          <TypeIcon kind={folder.template_key} />
        </span>
        <input
          type="checkbox"
          className="folder-checkbox"
          aria-label={`Select ${folder.title}`}
          checked={selected}
          onChange={toggle}
        />
      </div>
      <Link className="folder-title-link" to={`/folders/${folder.id}`}>
        <h3>{folder.title}</h3>
      </Link>
      <p className="folder-description">
        {folder.description || "A little knowledge worth keeping."}
      </p>
      <div className="folder-tags">
        <span className="type-tag">
          {templateNames[folder.template_key] || "Collection"}
        </span>
        {folder.topics
          ?.filter((t) => t.name)
          .slice(0, 2)
          .map((t) => (
            <span className="subtle-tag" key={t.name}>
              {t.name}
            </span>
          ))}
      </div>
      <div className="folder-card-meta">
        <span>{folder.material_count} materials</span>
        <span className={folder.due_count ? "text-accent" : "muted"}>
          <span className="status-dot" />
          {folder.due_count
            ? `${folder.due_count} ready`
            : folder.learning_count
              ? "On schedule"
              : "Ready to begin"}
        </span>
      </div>
      <div className="folder-card-foot">
        <Link to={`/folders/${folder.id}`}>
          Open collection
          <ChevronRight size={15} />
        </Link>
        <button
          className="icon-button train-folder"
          title={`Train ${folder.title}`}
          aria-label={`Train ${folder.title}`}
          onClick={() => launch([folder.id])}
        >
          <ArrowRight size={17} />
        </button>
      </div>
    </article>
  );
}
function FolderPage() {
  const folderID = useLocation().pathname.split("/")[2];
  const { data, reload, launch, notify } = useLibrary();
  const navigate = useNavigate();
  const folder = data?.folders.find((f) => f.id === folderID);
  const [page, setPage] = useState<MaterialPage | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(true);
  const [query, setQuery] = useState("");
  const [topic, setTopic] = useState("");
  const [sort, setSort] = useState("created_at");
  const [plan, setPlan] = useState("");
  const [offset, setOffset] = useState(0);
  const [revision, setRevision] = useState(0);
  const [editor, setEditor] = useState<Material | "new" | null>(null);
  const [details, setDetails] = useState<Material | null>(null);
  const [editFolder, setEditFolder] = useState(false);
  const [settings, setSettings] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    const timer = setTimeout(
      () => {
        setBusy(true);
        setError("");
        const params = new URLSearchParams({
          q: query,
          topic,
          sort,
          direction: sort === "created_at" ? "desc" : "asc",
          limit: "30",
          offset: String(offset),
        });
        if (plan) params.set("plan_id", plan);
        void api<MaterialPage>(
          `/library/folders/${folderID}/materials?${params}`,
          { signal: controller.signal },
        )
          .then(setPage)
          .catch((e) => {
            if (e.name !== "AbortError") setError(errorText(e));
          })
          .finally(() => {
            if (!controller.signal.aborted) setBusy(false);
          });
      },
      query ? 200 : 0,
    );
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [folderID, query, topic, sort, plan, offset, revision]);
  function refresh() {
    setRevision((r) => r + 1);
    void reload();
  }
  if (!folder)
    return data ? (
      <Empty
        title="Folder not found"
        action={
          <Link className="button" to="/">
            Back to library
          </Link>
        }
      >
        This folder may have been removed.
      </Empty>
    ) : (
      <Spinner />
    );
  return (
    <>
      <Link className="back-link" to="/">
        ← Back to library
      </Link>
      <div className="page-heading folder-page-heading">
        <div className="folder-title-row">
          <span className={`folder-icon ${folder.template_key}`}>
            <TypeIcon kind={folder.template_key} size={27} />
          </span>
          <div>
            <span className="eyebrow">
              {templateNames[folder.template_key]}
            </span>
            <h1>{folder.title}</h1>
            <p>{folder.description || "Make this knowledge your own."}</p>
          </div>
        </div>
        <div className="button-row">
          <button
            className="icon-button bordered"
            aria-label="Edit folder"
            onClick={() => setEditFolder(true)}
          >
            <MoreHorizontal size={20} />
          </button>
          <button className="button" onClick={() => setSettings(true)}>
            <Settings2 size={16} />
            Settings
          </button>
          <button
            className="button primary"
            onClick={() =>
              launch(
                [folder.id],
                topic ? [topic] : undefined,
                page?.selected_plan?.id,
              )
            }
          >
            <Zap size={16} />
            Train{topic ? " topic" : " folder"}
          </button>
        </div>
      </div>
      <div className="folder-summary">
        <span>
          <BookOpen size={16} />
          {folder.material_count} materials
        </span>
        <span className="text-accent">
          <Clock3 size={16} />
          {folder.due_count} ready to review
        </span>
        <span>
          <TrendingUp size={16} />
          {folder.learning_count} in progress
        </span>
      </div>
      <div className="materials-panel">
        <div className="section-top">
          <div className="section-title">
            <h2>Materials</h2>
            <span className="count-pill">
              {page?.total ?? folder.material_count}
            </span>
          </div>
          <button
            className="button primary small"
            onClick={() => setEditor("new")}
          >
            <Plus size={16} />
            Add material
          </button>
        </div>
        <div className="table-toolbar">
          <div className="search-input">
            <Search size={16} />
            <input
              aria-label="Search materials"
              placeholder="Search questions and answers…"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setOffset(0);
              }}
            />
          </div>
          <label className="filter-select">
            <ListFilter size={15} />
            <select
              aria-label="Filter by topic"
              value={topic}
              onChange={(e) => {
                setTopic(e.target.value);
                setOffset(0);
              }}
            >
              <option value="">All topics</option>
              {page?.topics.map((t) => (
                <option key={t.name || "__none__"} value={t.name || "__none__"}>
                  {t.name || "No topic"} ({t.count})
                </option>
              ))}
            </select>
          </label>
          <label className="filter-select">
            <ArrowDownUp size={15} />
            <select
              aria-label="Sort materials"
              value={sort}
              onChange={(e) => {
                setSort(e.target.value);
                setOffset(0);
              }}
            >
              <option value="created_at">Newest first</option>
              <option value="question">Question</option>
              <option value="topic">Topic</option>
              <option value="next_review_at">Next review</option>
              <option value="stage">Stage</option>
            </select>
          </label>
        </div>
        {!!page?.plans.length && (
          <div className="plan-context">
            <span>Showing progress for</span>
            <select
              aria-label="Progress plan"
              value={plan || page.selected_plan?.id || ""}
              onChange={(e) => {
                setPlan(e.target.value);
                setOffset(0);
              }}
            >
              {page.plans.map((p) => (
                <option key={p.id} value={p.id}>
                  {algorithmNames[p.algorithm_key]}
                  {p.horizon_days ? ` · ${p.horizon_days} days` : ""}
                  {p.status !== "active" ? ` · ${p.status}` : ""} ·{" "}
                  {new Date(p.created_at).toLocaleDateString("en", {
                    timeZone: "Europe/Moscow",
                    month: "short",
                    day: "numeric",
                  })}
                </option>
              ))}
            </select>
          </div>
        )}
        {error ? (
          <ErrorBox error={error} retry={refresh} />
        ) : busy ? (
          <Spinner label="Loading materials…" />
        ) : !page?.items.length ? (
          <Empty
            icon={<BookOpen size={26} />}
            title={
              query || topic
                ? "Nothing matches yet"
                : "A blank page, full of possibility"
            }
            action={
              !query && !topic ? (
                <button className="button" onClick={() => setEditor("new")}>
                  <Plus size={16} />
                  Add your first material
                </button>
              ) : undefined
            }
          >
            {query || topic
              ? "Try another search or topic."
              : "Add a question, a word or an idea you’d like to remember."}
          </Empty>
        ) : (
          <>
            <div className="material-table">
              <div className="material-table-head">
                <span>MATERIAL / QUESTION</span>
                <span>TOPIC</span>
                <span>NEXT REVIEW</span>
                <span>STAGE</span>
                <span />
              </div>
              {page.items.map((m) => (
                <button
                  className="material-row"
                  key={m.id}
                  onClick={() => setDetails(m)}
                >
                  <span className="material-question">
                    <span className={`difficulty-dot ${m.difficulty}`} />
                    <span>
                      {materialTitle(m, folder)}
                      <small>{m.difficulty} difficulty</small>
                    </span>
                  </span>
                  <span className="material-topic">
                    <span className="subtle-tag">
                      {topicOf(m) || "No topic"}
                    </span>
                  </span>
                  <span className="material-date">
                    {m.progress?.completed_at ? (
                      "Completed"
                    ) : m.progress ? (
                      <DateValue value={m.progress.next_review_at} />
                    ) : (
                      "Not started"
                    )}
                  </span>
                  <span className="material-stage">
                    {m.progress ? (
                      <span className="stage-pill">
                        {m.progress.completed_at ? (
                          <Check size={13} />
                        ) : (
                          `S${m.progress.stage}`
                        )}
                      </span>
                    ) : (
                      <span className="muted">—</span>
                    )}
                  </span>
                  <ChevronRight size={16} />
                </button>
              ))}
            </div>
            <div className="pagination">
              <span>
                {offset + 1}–{Math.min(offset + page.limit, page.total)} of{" "}
                {page.total} materials
              </span>
              <div className="button-row">
                <button
                  className="button small"
                  disabled={offset === 0}
                  onClick={() => setOffset(Math.max(0, offset - 30))}
                >
                  Previous
                </button>
                <button
                  className="button small"
                  disabled={offset + page.limit >= page.total}
                  onClick={() => setOffset(offset + 30)}
                >
                  Next
                </button>
              </div>
            </div>
          </>
        )}
      </div>
      {editor && (
        <MaterialEditor
          folder={folder}
          material={editor === "new" ? undefined : editor}
          topics={page?.topics || []}
          onClose={() => setEditor(null)}
          onSaved={() => {
            setEditor(null);
            refresh();
            notify("Material saved. A little more knowledge to keep.");
          }}
        />
      )}
      {details && (
        <MaterialDetails
          folder={folder}
          material={details}
          planId={page?.selected_plan?.id}
          onClose={() => setDetails(null)}
          onEdit={() => {
            setEditor(details);
            setDetails(null);
          }}
          onChanged={() => {
            setDetails(null);
            refresh();
          }}
        />
      )}
      {editFolder && (
        <FolderEditor
          folder={folder}
          onClose={() => setEditFolder(false)}
          onSaved={() => {
            setEditFolder(false);
            refresh();
          }}
          onDeleted={() => {
            void reload();
            navigate("/");
          }}
        />
      )}
      {settings && (
        <PlanSettings
          folder={folder}
          planId={page?.selected_plan?.id}
          onClose={() => setSettings(false)}
          onSaved={() => {
            setSettings(false);
            setPlan("");
            refresh();
          }}
        />
      )}
    </>
  );
}
function TrainingHome() {
  const { data, launch, loading, error, reload } = useLibrary();
  const { user } = useAuth();
  const navigate = useNavigate();
  const saved = readStored<SavedTraining>(`knowledge:training:${user.id}`);
  if (loading) return <Spinner />;
  if (error) return <ErrorBox error={error} retry={reload} />;
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">MAKE IT SECOND NATURE</span>
          <h1>
            A little practice<span className="heading-dot">.</span>
          </h1>
          <p>Bring a few ideas back. Leave knowing a little more.</p>
        </div>
      </div>
      <div className="training-home-hero">
        <span className="training-hero-icon">
          <Zap size={32} />
        </span>
        <span className="eyebrow">YOUR PACE. YOUR PROGRESS.</span>
        <h2>Pick up your next thought.</h2>
        <p>
          Mix folders, focus on a topic, or explore your whole library.
          <br />
          Every material follows its own learning schedule.
        </p>
        <div className="button-row">
          {saved && (
            <button
              className="button primary"
              onClick={() => navigate("/train")}
            >
              Continue training
              <ArrowRight size={17} />
            </button>
          )}
          <button
            className={`button ${saved ? "" : "primary"}`}
            disabled={!data?.folders.length}
            onClick={() => launch(data?.folders.map((f) => f.id) || [])}
          >
            {saved ? "Choose sources" : "Start training"}
            <ArrowRight size={17} />
          </button>
        </div>
        {!data?.folders.length && (
          <Link className="text-button" to="/">
            Create a folder to get started
          </Link>
        )}
      </div>
      <div className="training-notes">
        <div>
          <Clock3 size={20} />
          <h3>Always on your schedule</h3>
          <p>
            Reviews appear when they’re ready. Your schedule keeps moving while
            you’re away.
          </p>
        </div>
        <div>
          <GraduationCap size={20} />
          <h3>One space, many ways to learn</h3>
          <p>
            Vocabulary, interview questions and formulas can share a training
            flow.
          </p>
        </div>
        <div>
          <Sparkles size={20} />
          <h3>Every answer counts</h3>
          <p>Your progress saves as you go. Come and go whenever you like.</p>
        </div>
      </div>
    </>
  );
}
