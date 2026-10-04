import {
  ArrowRight,
  Check,
  ChevronRight,
  Clock3,
  GitBranch,
  GraduationCap,
  LayoutGrid,
  LogOut,
  Sparkles,
  TrendingUp,
  X,
  Zap,
} from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import {
  Link,
  NavLink,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from "react-router-dom";
import { errorText, readStored } from "./api";
import { useAuth } from "./auth";
import { InterviewPage } from "./interview";
import { StatisticsPage } from "./statistics";
import { TrainingPage, TrainingSetup, type SavedTraining } from "./training";
import { Empty, ErrorBox, Logo, Modal, Spinner } from "./ui";

import {
  LibraryContext,
  useLibrary,
  type LibraryContextValue,
} from "./library/context";
import { FolderPage } from "./library/folder-page";
import { LibraryPage } from "./library/library-page";
import {
  LibraryPreferencesProvider,
  useLibraryPreferences,
} from "./library/preferences-context";
import { useLibraryData } from "./library/use-library-data";

export function App() {
  const { user } = useAuth();
  return (
    <LibraryPreferencesProvider key={user.id} userId={user.id}>
      <LibraryApp />
    </LibraryPreferencesProvider>
  );
}
function LibraryApp() {
  const { data, loading, error, reload } = useLibraryData();
  const { prune } = useLibraryPreferences();
  useEffect(() => {
    if (data) prune(data.folders);
  }, [data, prune]);
  const [toast, setToast] = useState("");
  const [setup, setSetup] = useState<{
    ids: string[];
    topics?: string[];
    planId?: string;
  } | null>(null);
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
                <Route path="/interview" element={<InterviewPage />} />
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
            aria-label="My library"
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
          <NavLink to="/training" aria-label="Training">
            <Zap size={18} />
            <span>Training</span>
          </NavLink>
          <NavLink to="/statistics" aria-label="Statistics">
            <TrendingUp size={18} />
            <span>Statistics</span>
          </NavLink>
          <NavLink to="/interview" aria-label="Mock interview">
            <GitBranch size={18} />
            <span>Mock interview</span>
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
              {location.pathname === "/interview"
                ? "Mock interview"
                : location.pathname === "/statistics"
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
