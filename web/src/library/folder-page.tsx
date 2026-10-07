import {
  ArrowDownUp,
  BookOpen,
  Check,
  ChevronRight,
  Clock3,
  ListFilter,
  MoreHorizontal,
  Plus,
  Search,
  Settings2,
  Trash2,
  TrendingUp,
  Zap,
} from "lucide-react";
import { useEffect, useLayoutEffect, useState } from "react";
import {
  Link,
  useLocation,
  useNavigate,
  useNavigationType,
} from "react-router-dom";
import { api, errorText } from "../api";
import { FolderEditor, MaterialDetails, MaterialEditor } from "../editors";
import { PlanSettings } from "../settings";
import type { Material, MaterialPage } from "../types";
import {
  algorithmNames,
  materialTitle,
  templateNames,
  topicOf,
} from "../types";
import { DateValue, Empty, ErrorBox, Spinner } from "../ui";

import { useSearchParams } from "react-router-dom";
import { FolderIcon } from "./appearance";
import { useLibrary } from "./context";
import { MaterialBulkActions } from "./material-bulk-actions";
import { CopyDialog } from "./copy-dialog";
import { CustomizeDialog } from "./customize-dialog";
import { FolderDescription } from "./folder-description";
import { useLibraryPreferences } from "./preferences-context";
export function FolderPage() {
  const folderID = useLocation().pathname.split("/")[2];
  const navigationType = useNavigationType();
  useLayoutEffect(() => {
    // BrowserRouter keeps the document scroll on client-side navigation.
    // New collections start at the top before paint; history traversal keeps
    // the browser's restoration. Data refreshes must not reset the reader.
    if (navigationType !== "POP")
      window.scrollTo({ top: 0, left: 0, behavior: "instant" });
  }, [folderID, navigationType]);
  const { data, reload, launch, notify } = useLibrary();
  const navigate = useNavigate();
  const folder = data?.folders.find((f) => f.id === folderID);
  const { preferences, markUsed } = useLibraryPreferences();
  useEffect(() => {
    if (folder?.id) markUsed([folder.id]);
  }, [folder?.id, markUsed]);
  const section = preferences.categories.find(
    (c) => c.id === preferences.folders[folderID]?.categoryId,
  );
  const [customize, setCustomize] = useState(false);
  const [copyMaterial, setCopyMaterial] = useState<Material | null>(null);
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedMaterial = searchParams.get("material");
  const [page, setPage] = useState<MaterialPage | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(true);
  const [query, setQuery] = useState("");
  const [topic, setTopic] = useState("");
  const [sort, setSort] = useState("created_at");
  const [plan, setPlan] = useState("");
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<string[]>([]);
  useEffect(
    () => setSelected([]),
    [folderID, query, topic, sort, plan, offset],
  );
  const [revision, setRevision] = useState(0);
  const [editor, setEditor] = useState<Material | "new" | null>(null);
  const [details, setDetails] = useState<Material | null>(null);
  useEffect(() => {
    if (!requestedMaterial) return;
    const controller = new AbortController();
    void api<Material>(`/folders/${folderID}/materials/${requestedMaterial}`, {
      signal: controller.signal,
    })
      .then((value) => {
        if (!controller.signal.aborted) setDetails(value);
      })
      .catch((err) => {
        if (!controller.signal.aborted) setError(errorText(err));
      });
    return () => controller.abort();
  }, [folderID, requestedMaterial]);
  function closeDetails() {
    setDetails(null);
    if (requestedMaterial) {
      const next = new URLSearchParams(searchParams);
      next.delete("material");
      setSearchParams(next, { replace: true });
    }
  }
  const [editFolder, setEditFolder] = useState(false);
  const [deleteFolder, setDeleteFolder] = useState(false);
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
          .then((value) => {
            if (!controller.signal.aborted) setPage(value);
          })
          .catch((e) => {
            if (!controller.signal.aborted) setError(errorText(e));
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
      <Link
        className="back-link"
        to={section ? `/sections/${encodeURIComponent(section.id)}` : "/"}
      >
        {section ? `← ${section.name}` : "← Back to library"}
      </Link>
      <div className="page-heading folder-page-heading">
        <div className="folder-title-row">
          <span
            className={`folder-icon ${folder.template_key} folder-color-${preferences.folders[folder.id]?.color || "default"}`}
          >
            <FolderIcon
              kind={folder.template_key}
              appearance={preferences.folders[folder.id]}
              size={27}
            />
          </span>
          <div>
            <span className="eyebrow">
              {templateNames[folder.template_key]}
            </span>
            <h1>{folder.title}</h1>
            <FolderDescription
              value={folder.description || "Make this knowledge your own."}
            />
          </div>
        </div>
        <div className="button-row">
          <button className="button" onClick={() => setCustomize(true)}>
            Customize
          </button>
          <button
            className="button"
            aria-label="Edit folder"
            onClick={() => setEditFolder(true)}
          >
            <MoreHorizontal size={20} />
            Edit folder
          </button>
          <button
            className="button danger-text"
            onClick={() => setDeleteFolder(true)}
          >
            <Trash2 size={17} />
            Delete folder
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
            <MaterialBulkActions
              folder={folder}
              shown={page.items}
              selected={selected}
              setSelected={setSelected}
              onSaved={refresh}
            />
            <div className="material-table">
              <div className="material-table-head">
                <span>MATERIAL / QUESTION</span>
                <span>TOPIC</span>
                <span>NEXT REVIEW</span>
                <span>STAGE</span>
                <span />
              </div>
              {page.items.map((m) => (
                <div className="material-row-shell" key={m.id}>
                  <input
                    type="checkbox"
                    aria-label={`Select ${materialTitle(m, folder)}`}
                    checked={selected.includes(m.id)}
                    onChange={() =>
                      setSelected((ids) =>
                        ids.includes(m.id)
                          ? ids.filter((id) => id !== m.id)
                          : [...ids, m.id],
                      )
                    }
                  />
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
                </div>
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
          onClose={closeDetails}
          onCopy={
            folder.template_key === "interview_questions"
              ? () => {
                  setCopyMaterial(details);
                  closeDetails();
                }
              : undefined
          }
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
      {copyMaterial && (
        <CopyDialog
          folder={folder}
          material={copyMaterial}
          onClose={() => setCopyMaterial(null)}
        />
      )}
      {customize && (
        <CustomizeDialog folder={folder} onClose={() => setCustomize(false)} />
      )}
      {(editFolder || deleteFolder) && (
        <FolderEditor
          folder={folder}
          initialDeleting={deleteFolder}
          onClose={() => {
            setEditFolder(false);
            setDeleteFolder(false);
          }}
          onSaved={() => {
            setEditFolder(false);
            setDeleteFolder(false);
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
