import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  CheckCheck,
  ChevronDown,
  Clock3,
  Edit3,
  Layers3,
  RotateCcw,
  Settings2,
  X,
  Zap,
} from "lucide-react";
import { api, ApiError, errorText, post, readStored, saveStored } from "./api";
import { useAuth } from "./auth";
import { useLibrary, TypeIcon } from "./App";
import type { CombinedView, Folder, Material, Source } from "./types";
import { algorithmNames } from "./types";
import {
  DateValue,
  Empty,
  ErrorBox,
  Logo,
  Markdown,
  Modal,
  Spinner,
} from "./ui";
import { MaterialEditor } from "./editors";
import { algorithmsFor } from "./settings";
import { WordExample } from "./example";
import { RecallCard } from "./recall-card";

export type SavedTraining = { sources: Source[]; session_ids: string[] };
type PendingAnswer = {
  sessionId: string;
  body: {
    command_id: string;
    presentation_id: string;
    expected_version: number;
    action: string;
  };
};
export function TrainingSetup({
  initial,
  folders,
  onClose,
}: {
  initial: { ids: string[]; topics?: string[]; planId?: string };
  folders: Folder[];
  onClose: () => void;
}) {
  const { user } = useAuth();
  const { reload } = useLibrary();
  const navigate = useNavigate();
  const [selected, setSelected] = useState(initial.ids);
  const [sources, setSources] = useState<Record<string, Source>>(() =>
    Object.fromEntries(
      folders.map((f) => [
        f.id,
        {
          folder_id: f.id,
          ...(initial.ids.includes(f.id) && initial.topics
            ? { topics: initial.topics }
            : {}),
          ...(initial.ids.length === 1 && initial.planId
            ? { plan_id: initial.planId }
            : f.selected_plan
              ? { plan_id: f.selected_plan.id }
              : {
                  algorithm_key: f.training_config.default_algorithm_key,
                  pool_size: f.training_config.pool_size,
                  ...(f.training_config.default_algorithm_key.startsWith(
                    "interview",
                  )
                    ? {
                        horizon_days:
                          f.training_config.default_algorithm_key ===
                          "interview_cram"
                            ? 5
                            : 150,
                      }
                    : {}),
                }),
        },
      ]),
    ),
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [expanded, setExpanded] = useState<string | null>(
    initial.ids.length === 1 ? initial.ids[0] : null,
  );
  function update(id: string, patch: Partial<Source>) {
    setSources((s) => ({ ...s, [id]: { ...s[id], ...patch } }));
  }
  async function start(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const chosen = selected.map((id) => sources[id]);
      const view = await post<CombinedView>("/training/combined", {
        sources: chosen,
      });
      const saved = {
        sources: chosen,
        session_ids: view.sessions.map((s) => s.session.id),
      };
      saveStored(`knowledge:training:${user.id}`, saved);
      onClose();
      void reload();
      navigate("/train", { state: { initialView: view } });
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title="Find your focus"
      subtitle="Choose your sources. Each material keeps its own learning plan."
      onClose={() => {
        if (!busy) onClose();
      }}
      wide
    >
      <form className="form-body" onSubmit={start}>
        <div className="source-selection-top">
          <span>
            {selected.length} {selected.length === 1 ? "folder" : "folders"}{" "}
            selected
          </span>
          <button
            type="button"
            className="text-button"
            onClick={() =>
              setSelected(
                selected.length === folders.length
                  ? []
                  : folders.map((f) => f.id),
              )
            }
          >
            {selected.length === folders.length
              ? "Clear selection"
              : "Select all"}
          </button>
        </div>
        <div className="source-list">
          {folders.map((f) => {
            const src = sources[f.id];
            const active = selected.includes(f.id);
            return (
              <div
                key={f.id}
                className={`source-card ${active ? "selected" : ""}`}
              >
                <div className="source-main">
                  <input
                    type="checkbox"
                    checked={active}
                    aria-label={`Train with ${f.title}`}
                    onChange={() =>
                      setSelected((ids) =>
                        active
                          ? ids.filter((id) => id !== f.id)
                          : [...ids, f.id],
                      )
                    }
                  />
                  <span className={`folder-icon small ${f.template_key}`}>
                    <TypeIcon kind={f.template_key} size={18} />
                  </span>
                  <div>
                    <strong>{f.title}</strong>
                    <span>
                      {
                        algorithmNames[
                          f.selected_plan?.algorithm_key ||
                            src.algorithm_key ||
                            f.training_config.default_algorithm_key
                        ]
                      }{" "}
                      · {f.material_count} materials
                    </span>
                  </div>
                  <button
                    type="button"
                    className="icon-button"
                    aria-label={`Configure ${f.title}`}
                    onClick={() => setExpanded(expanded === f.id ? null : f.id)}
                  >
                    <ChevronDown size={17} />
                  </button>
                </div>
                {expanded === f.id && (
                  <div className="source-options">
                    {!src.plan_id && (
                      <>
                        <label>
                          Algorithm
                          <select
                            value={src.algorithm_key}
                            onChange={(e) =>
                              update(f.id, {
                                algorithm_key: e.target.value,
                                horizon_days:
                                  e.target.value === "interview_cram"
                                    ? 5
                                    : e.target.value === "interview_long_term"
                                      ? 150
                                      : undefined,
                              })
                            }
                          >
                            {algorithmsFor(f).map((k) => (
                              <option key={k} value={k}>
                                {algorithmNames[k]}
                              </option>
                            ))}
                          </select>
                        </label>
                        <div className="form-columns">
                          <label>
                            Pool size
                            <input
                              type="number"
                              min={1}
                              max={50}
                              value={src.pool_size}
                              onChange={(e) =>
                                update(f.id, {
                                  pool_size: Number(e.target.value),
                                })
                              }
                            />
                          </label>
                          {src.algorithm_key?.startsWith("interview") && (
                            <label>
                              Learning horizon · days
                              <input
                                type="number"
                                min={
                                  src.algorithm_key === "interview_cram" ? 1 : 7
                                }
                                max={
                                  src.algorithm_key === "interview_cram"
                                    ? 7
                                    : 365
                                }
                                value={src.horizon_days}
                                onChange={(e) =>
                                  update(f.id, {
                                    horizon_days: Number(e.target.value),
                                  })
                                }
                              />
                            </label>
                          )}
                        </div>
                      </>
                    )}
                    {!!f.topics?.length && (
                      <div>
                        <span className="field-label">Focus on topics</span>
                        <div className="topic-choices">
                          <button
                            type="button"
                            className={!src.topics?.length ? "selected" : ""}
                            onClick={() => update(f.id, { topics: [] })}
                          >
                            All topics
                          </button>
                          {f.topics.map((t) => {
                            const name = t.name || "__none__";
                            const chosen = src.topics?.includes(name);
                            return (
                              <button
                                type="button"
                                key={name}
                                className={chosen ? "selected" : ""}
                                onClick={() =>
                                  update(f.id, {
                                    topics: chosen
                                      ? src.topics!.filter((x) => x !== name)
                                      : [...(src.topics || []), name],
                                  })
                                }
                              >
                                {t.name || "No topic"}
                                <span>{t.count}</span>
                              </button>
                            );
                          })}
                        </div>
                      </div>
                    )}
                    {src.plan_id && (
                      <p className="muted small-text">
                        Uses your existing plan. Algorithm and schedule changes
                        are available in the folder’s settings.
                      </p>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
        {error && <ErrorBox error={error} />}
        <div className="dialog-actions">
          <span className="save-hint">
            <CheckCheck size={15} />
            Your selection is remembered
          </span>
          <button
            className="button primary"
            disabled={busy || !selected.length}
          >
            {busy ? "Preparing your cards…" : "Start training"}
            <ArrowRight size={17} />
          </button>
        </div>
      </form>
    </Modal>
  );
}
export function TrainingPage() {
  const { user } = useAuth();
  const { data, reload, notify } = useLibrary();
  const storageKey = `knowledge:training:${user.id}`;
  const pendingKey = `knowledge:pending:${user.id}`;
  const undoKey = `knowledge:undo:${user.id}`;
  const undoPending = useRef(
    readStored<{ command_id: string; event_id: string; session_ids: string[] }>(
      undoKey,
    ),
  );
  const savedRef = useRef(readStored<SavedTraining>(storageKey));
  const [view, setView] = useState<CombinedView | null>(null);
  const [busy, setBusy] = useState(true);
  const busyRef = useRef(true);
  const [error, setError] = useState("");
  const [shownCardId, setShownCardId] = useState<string | null>(null);
  const [swipe, setSwipe] = useState("");
  const [clock, setClock] = useState(Date.now());
  const [editor, setEditor] = useState<Material | null>(null);
  const pending = useRef(readStored<PendingAnswer>(pendingKey));
  const earlyCommand = useRef<string | null>(null);
  const [uncertain, setUncertain] = useState(
    !!pending.current || !!undoPending.current,
  );
  const [editBusy, setEditBusy] = useState(false);
  function apply(next: CombinedView) {
    setView(next);
    if (savedRef.current) {
      savedRef.current = {
        ...savedRef.current,
        session_ids: next.sessions.map((s) => s.session.id),
      };
      saveStored(storageKey, savedRef.current);
    }
  }
  async function refresh(early = false) {
    const saved = savedRef.current;
    if (!saved) {
      setBusy(false);
      busyRef.current = false;
      return;
    }
    busyRef.current = true;
    setBusy(true);
    setError("");
    try {
      if (undoPending.current) {
        const restored = await post<CombinedView>(
          "/training/combined/undo",
          undoPending.current,
        );
        apply(restored);
        undoPending.current = null;
        saveStored(undoKey, null);
        setUncertain(false);
        setShownCardId(null);
        return;
      }
      if (early && !earlyCommand.current)
        earlyCommand.current = crypto.randomUUID();
      if (pending.current) {
        await post(
          `/training/sessions/${pending.current.sessionId}/actions`,
          pending.current.body,
        );
        pending.current = null;
        saveStored(pendingKey, null);
        setUncertain(false);
      }
      const next = saved.session_ids.length
        ? await post<CombinedView>("/training/combined/current", {
            session_ids: saved.session_ids,
            review_early: early,
            ...(early ? { command_id: earlyCommand.current } : {}),
          })
        : await post<CombinedView>("/training/combined", {
            sources: saved.sources,
          });
      apply(next);
      earlyCommand.current = null;
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        undoPending.current = null;
        saveStored(undoKey, null);
        pending.current = null;
        saveStored(pendingKey, null);
        setUncertain(false);
        savedRef.current = { ...saved, session_ids: [] };
        saveStored(storageKey, savedRef.current);
        setError(
          "This training changed in another tab. Refresh to load its current state.",
        );
      } else setError(errorText(e));
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }
  useEffect(() => {
    void refresh();
    return () => {
      void reload();
    };
  }, []);
  const current = view?.current;
  const card = current?.presentation;
  // A new presentation starts on its question in the very first render.
  const shown = !!card && shownCardId === card.id;
  function flip() {
    if (card) setShownCardId((id) => (id === card.id ? null : card.id));
  }
  useEffect(() => {
    const timer = window.setInterval(() => setClock(Date.now()), 30000);
    return () => window.clearInterval(timer);
  }, []);
  async function answer(action: string) {
    if (!current || !card || busyRef.current || editor || uncertain || error)
      return;
    busyRef.current = true;
    setBusy(true);
    setError("");
    const command = pending.current || {
      sessionId: current.session_id,
      body: {
        command_id: crypto.randomUUID(),
        presentation_id: card.id,
        expected_version: card.progress_version,
        action,
      },
    };
    pending.current = command;
    saveStored(pendingKey, command);
    try {
      await post(
        `/training/sessions/${command.sessionId}/actions`,
        command.body,
      );
      pending.current = null;
      saveStored(pendingKey, null);
      setUncertain(false);
      const next = await post<CombinedView>("/training/combined/current", {
        session_ids: savedRef.current!.session_ids,
      });
      setSwipe(action);
      await new Promise((resolve) =>
        window.setTimeout(
          resolve,
          window.matchMedia("(prefers-reduced-motion: reduce)").matches
            ? 0
            : 260,
        ),
      );
      apply(next);
    } catch (e) {
      if (e instanceof ApiError && e.status >= 400 && e.status < 500) {
        pending.current = null;
        saveStored(pendingKey, null);
        setUncertain(false);
        setError(
          e.status === 409
            ? "This card changed elsewhere. Refresh to continue with the current card."
            : errorText(e),
        );
      } else {
        setUncertain(true);
        setError(
          "Your connection was interrupted. Retry to safely confirm this answer.",
        );
      }
    } finally {
      setSwipe("");
      busyRef.current = false;
      setBusy(false);
    }
  }
  async function undo() {
    if (busyRef.current || !view?.undo_actions?.length || !savedRef.current)
      return;
    undoPending.current = {
      command_id: crypto.randomUUID(),
      event_id: view.undo_actions[0],
      session_ids: savedRef.current.session_ids,
    };
    saveStored(undoKey, undoPending.current);
    setUncertain(true);
    await refresh();
  }
  useEffect(() => {
    function keydown(e: KeyboardEvent) {
      if (
        e.repeat ||
        busyRef.current ||
        editor ||
        uncertain ||
        error ||
        e.ctrlKey ||
        e.metaKey ||
        e.altKey ||
        (e.target instanceof HTMLElement &&
          e.target.closest(
            "input, textarea, select, [contenteditable], dialog",
          ))
      )
        return;
      if (e.code === "Space" && card) {
        if (
          e.target instanceof HTMLElement &&
          e.target.closest("button, a, [role=button]")
        )
          return;
        e.preventDefault();
        flip();
      } else if (card && (e.key === "1" || e.key === "2")) {
        e.preventDefault();
        void answer(e.key === "1" ? "wrong" : "correct");
      }
    }
    window.addEventListener("keydown", keydown);
    return () => window.removeEventListener("keydown", keydown);
  }, [shown, card, current, editor, uncertain, error]);
  useEffect(() => {
    let last = Date.now();
    function focus() {
      if (Date.now() - last > 60000 && !busyRef.current && !editor)
        void refresh();
      last = Date.now();
    }
    window.addEventListener("focus", focus);
    return () => window.removeEventListener("focus", focus);
  }, [editor]);
  async function edit() {
    if (!card) return;
    setEditBusy(true);
    try {
      setEditor(
        await api<Material>(
          `/folders/${card.folder_id}/materials/${card.material_id}`,
        ),
      );
    } catch (e) {
      notify(errorText(e));
    } finally {
      setEditBusy(false);
    }
  }
  async function skipRehab() {
    if (!card || !current || busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    try {
      await post(
        `/training/sessions/${current.session_id}/materials/${card.material_id}/skip-rehab`,
        {
          command_id: crypto.randomUUID(),
          expected_version: card.progress_version,
        },
      );
      await refresh();
    } catch (e) {
      setError(errorText(e));
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }
  const folder = data?.folders.find((f) => f.id === card?.folder_id);
  const hideContext = ["independent", "mixed", "maintenance"].includes(
    card?.practice_mode || "",
  );
  return (
    <div className="training-screen">
      <header className="training-header">
        <Link className="training-back" to="/" aria-label="Back to library">
          <ArrowLeft size={19} />
          <span>Library</span>
        </Link>
        <Link to="/" className="training-logo">
          <Logo />
        </Link>
        <div className="training-header-right">
          <span className="saved-status">
            <span className="status-dot" />
            {busy
              ? "Syncing"
              : uncertain
                ? "Awaiting confirmation"
                : "Progress saved"}
          </span>
        </div>
      </header>
      <main className="training-main">
        {view && (
          <div className="training-history">
            <button
              className="undo-button"
              aria-label="Undo last answer"
              title="Restore up to your last three answers"
              disabled={
                busy || uncertain || !!error || !view.undo_actions?.length
              }
              onClick={undo}
            >
              <RotateCcw size={16} /> Undo · {view.undo_actions?.length || 0}
            </button>
          </div>
        )}
        {!savedRef.current ? (
          <Empty
            icon={<Layers3 size={28} />}
            title="What will you learn today?"
            action={
              <Link className="button primary" to="/training">
                Choose your sources
                <ArrowRight size={17} />
              </Link>
            }
          >
            Open a folder or choose a few collections to begin.
          </Empty>
        ) : !view && busy ? (
          <Spinner label="Bringing your knowledge into focus…" />
        ) : (
          <>
            {error && <ErrorBox error={error} retry={() => refresh()} />}
            {card && current ? (
              <>
                <div className="training-context">
                  <span className="subtle-tag">
                    {hideContext
                      ? "Formula practice"
                      : folder?.title || "Your collection"}
                  </span>
                  <span>
                    {card.final_review
                      ? "Final review"
                      : card.kind === "rehab"
                        ? "Rehab"
                        : card.kind === "extra"
                          ? "Extra review"
                          : `Stage ${card.stage}`}
                  </span>
                  <span className="flex-spacer" />
                  <span className="answer-counter">
                    {(view?.summary.correct || 0) + (view?.summary.wrong || 0)}{" "}
                    answers
                  </span>
                  <button
                    className="button edit-word-button"
                    aria-label="Edit current material"
                    onClick={edit}
                    disabled={editBusy || busy || uncertain}
                  >
                    <Edit3 size={18} />
                    {editBusy
                      ? "Opening…"
                      : current.algorithm_key.startsWith("english_")
                        ? "Edit word"
                        : "Edit card"}
                  </button>
                </div>
                <RecallCard
                  key={card.id}
                  card={card}
                  algorithm={current.algorithm_key}
                  shown={shown}
                  flip={flip}
                  answer={answer}
                  busy={busy || uncertain || !!error}
                  swipe={swipe}
                  skipRehab={skipRehab}
                />
              </>
            ) : (
              view && (
                <div className="all-done">
                  <div className="done-orbit">
                    <CheckCheck size={38} />
                  </div>
                  <span className="eyebrow">A LITTLE MORE FAMILIAR</span>
                  <h1>
                    All done for now<span className="heading-dot">.</span>
                  </h1>
                  <p>
                    {view.empty_reason === "no_matching_materials"
                      ? "No materials are ready in this selection. Formulas need at least one practice exercise."
                      : "You’ve made room for a little more knowledge. Come back when your next review is ready."}
                  </p>
                  {view.next_review_at && (
                    <div className="next-review-note">
                      <Clock3 size={17} />
                      <span>
                        Next review: <DateValue value={view.next_review_at} />
                      </span>
                    </div>
                  )}
                  {view.next_review_at &&
                    new Date(view.next_review_at).getTime() > clock &&
                    new Date(view.next_review_at).getTime() - clock <
                      3 * 3600000 && (
                      <div className="early-review-panel">
                        <Zap size={22} />
                        <div>
                          <strong>A little ahead of schedule</strong>
                          <p>
                            Your next card is ready for an early review. This
                            counts as a regular review.
                          </p>
                        </div>
                        <button
                          className="button primary"
                          disabled={busy}
                          onClick={() => refresh(true)}
                        >
                          Review early <ArrowRight size={18} />
                        </button>
                      </div>
                    )}
                  <div className="done-stats">
                    <div>
                      <strong>
                        {view.summary.correct + view.summary.wrong}
                      </strong>
                      <span>answers</span>
                    </div>
                    <div>
                      <strong>{view.summary.materials_reviewed}</strong>
                      <span>materials reviewed</span>
                    </div>
                    <div>
                      <strong>{view.summary.correct}</strong>
                      <span>correct</span>
                    </div>
                  </div>
                  <div className="button-row">
                    <Link to="/" className="button primary">
                      Back to library
                      <ArrowRight size={17} />
                    </Link>
                    <button
                      className="button"
                      onClick={() => refresh()}
                      disabled={busy}
                    >
                      Check again
                    </button>
                  </div>
                </div>
              )
            )}
          </>
        )}
      </main>
      <footer className="training-footer">
        <span>Your pace. Your progress.</span>
        <span>No rush. Let it sink in.</span>
      </footer>
      {editor && folder && (
        <MaterialEditor
          folder={folder}
          material={editor}
          topics={folder.topics}
          inTraining
          onClose={() => setEditor(null)}
          onSaved={() => {
            setEditor(null);
            notify("Saved. Your changes will appear on the next presentation.");
            void reload();
          }}
        />
      )}
    </div>
  );
}
