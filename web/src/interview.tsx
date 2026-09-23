import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Check,
  ChevronDown,
  GitBranch,
  Layers3,
  RotateCcw,
  Settings2,
  Sparkles,
  X,
} from "lucide-react";
import { api, ApiError, errorText, post, readStored, saveStored } from "./api";
import { useAuth } from "./auth";
import { useLibrary } from "./App";
import type { Folder } from "./types";
import { ErrorBox, Markdown, Spinner } from "./ui";
import {
  defaultGraphConfig,
  interviewProfiles,
  interviewLevels,
  type InterviewGraphConfig,
  type InterviewSession,
  type InterviewPlan,
} from "./interview-types";
import { InterviewBankTools } from "./interview-editor";
import "./interview.css";

type GraphCommand = {
  session_id: string;
  kind: "answer" | "undo";
  body: {
    command_id: string;
    presentation_id?: string;
    expected_version?: number;
    action?: string;
    event_id?: string;
  };
};

export function InterviewPage() {
  const { user } = useAuth();
  const { data, reload } = useLibrary();
  const storageKey = `knowledge:interview:${user.id}`;
  const [sessionID, setSessionID] = useState(
    () => readStored<string>(storageKey) || "",
  );
  const [view, setView] = useState<InterviewSession | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  async function restore() {
    setLoading(true);
    setError("");
    try {
      const restored = await api<InterviewSession>(sessionID ? `/training/sessions/${sessionID}` : "/training/mock-interviews/active");
      if (restored.graph?.state.practice_only) setView(restored);
      else { // Old plan-backed graph sessions retain their API/SRS semantics,
        // but must never be presented as practice-only Mock Interview.
        saveStored(storageKey, null);setSessionID("");setView(null);
      }
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        saveStored(storageKey, null);
        setSessionID("");
      } else setError(errorText(e));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void restore();
  }, [sessionID]);
  function opened(next: InterviewSession) {
    saveStored(storageKey, next.session.id);
    setSessionID(next.session.id);
    setView(next);
    void reload();
  }
  if (loading) return <Spinner label="Opening your interview…" />;
  return (
    <div className="interview-page">
      {error && <ErrorBox error={error} retry={() => void restore()} />}
      {view ? (
        <InterviewRun
          key={view.session.id}
          initial={view}
          onSetup={() => {
            setView(null);
            setSessionID("");
            saveStored(storageKey, null);
          }}
        />
      ) : (
        <InterviewSetup
          folders={(data?.folders || []).filter(
            (f) => f.template_key === "interview_questions",
          )}
          onStarted={opened}
        />
      )}
    </div>
  );
}

function InterviewSetup({
  folders,
  onStarted,
}: {
  folders: Folder[];
  onStarted: (view: InterviewSession) => void;
}) {
  const [selected, setSelected] = useState<string[]>([]);
  const [topics, setTopics] = useState<Record<string, string[]>>({});
  const [preview, setPreview] = useState<InterviewPlan | null>(null);
  const [previewError, setPreviewError] = useState("");
  const [previewBusy, setPreviewBusy] = useState(false);
  const [availableTopicKeys, setAvailableTopicKeys] = useState<string[]>([]);
  const [canResume, setCanResume] = useState(false);
  const [config, setConfig] = useState(defaultGraphConfig);
  const [advanced, setAdvanced] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const startCommand = useRef<{
    id: string;
    sources: { folder_id: string; topics?: string[] }[];
    config: InterviewGraphConfig;
  } | null>(null);
  useEffect(() => {
    let alive = true;
    async function load() {
      const defaults = await api<InterviewGraphConfig>(
        "/training/interview-graph/config",
      );
      if (alive) {
        setConfig(defaults);
      }
    }
    void load().catch((e) => {
      if (alive) setError(errorText(e));
    });
    return () => {
      alive = false;
    };
  }, []);
  const available = folders;
  const sources = selected.map(folder_id => ({folder_id, ...(topics[folder_id]?.length ? {topics:topics[folder_id]} : {})}));
  const previewKey = JSON.stringify({sources,config});
  useEffect(() => {
    let alive = true;
    setPreview(null); setPreviewError(""); setPreviewBusy(!!selected.length);
    if (!selected.length) {setAvailableTopicKeys([]);return;}
    const timer = setTimeout(() => {
      void post<InterviewPlan>("/training/mock-interviews/preview", {sources,config}).then(value => {
        if (alive) {setPreview(value);setAvailableTopicKeys(value.topics.map(t=>t.key));}
      }).catch(async e => {
        if (alive) setPreviewError(errorText(e));
        // Keep the controls for available topics even when all custom weights
        // are zero, so the user can correct the invalid configuration.
        if (config.interview_mode === "custom") {
          try {const available = await post<InterviewPlan>("/training/mock-interviews/preview", {sources,config:{...config,interview_mode:"balanced",custom_weights:undefined}});if(alive)setAvailableTopicKeys(available.topics.map(t=>t.key));} catch { /* The original source/validation error remains visible. */ }
        }
      }).finally(() => {if (alive) setPreviewBusy(false);});
    }, 200);
    return () => { alive = false; clearTimeout(timer); };
  }, [previewKey]);
  async function start(e: FormEvent) {
    e.preventDefault();
    if (busy || !selected.length) return;
    setBusy(true);
    setError("");
    setCanResume(false);
    try {
      if (!startCommand.current) {
        startCommand.current = {
          id: crypto.randomUUID(),
          sources: selected.map((folder_id) => ({
            folder_id,
            ...(topics[folder_id]?.length ? { topics: topics[folder_id] } : {}),
          })),
          config: structuredClone(config),
        };
      }
      const command = startCommand.current;
      const next = await post<InterviewSession>(
        "/training/mock-interviews",
        {
          command_id: command.id,
          config: command.config,
          sources: command.sources,
        },
      );
      startCommand.current = null;
      onStarted(next);
    } catch (e) {
      if (e instanceof ApiError && e.status >= 400 && e.status < 500)
        startCommand.current = null;
      setError(errorText(e));
      if (e instanceof ApiError && e.code === "active_mock_interview") setCanResume(true);
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <header className="interview-heading">
        <div>
          <span className="eyebrow">A conversation with your knowledge</span>
          <h1>
            Mock interview<span>.</span>
          </h1>
          <p>
            Answer out loud or in your head. Check the reference, rate yourself,
            and follow the next connection.
          </p>
        </div>
        <span className="interview-heading-icon">
          <GitBranch size={32} />
        </span>
      </header>
      <InterviewBankTools />
      <form className="interview-setup" onSubmit={start}>
        <div className="interview-setup-main">
          <div className="interview-section-title">
            <div>
              <h2>Choose your subjects</h2>
              <p>Ready questions from these folders shape your interview.</p>
            </div>
            <span>{selected.length && selected.length === available.length ? `All selected (${selected.length})` : `${selected.length} selected`}</span>
          </div>
          <div className="interview-source-actions">
            <button type="button" className="button" disabled={busy || !!startCommand.current || !available.length} onClick={() => {setSelected(available.map(f => f.id));setTopics({});}}>Select all</button>
            <button type="button" className="button" disabled={busy || !!startCommand.current || !selected.length} onClick={() => {setSelected([]);setTopics({});}}>Clear all</button>
          </div>
          {!available.length ? (
            <div className="interview-empty-source">
              <BookOpen size={32} />
              <h3>Your question bank starts here</h3>
              <p>
                Create an Interview prep folder, add questions with answers, and
                mark them ready.
              </p>
              <Link className="button" to="/">
                Open library <ArrowRight size={16} />
              </Link>
            </div>
          ) : (
            <fieldset
              className="interview-sources"
              disabled={busy || !!startCommand.current}
            >
              <legend className="sr-only">Interview folders</legend>
              {available.map((folder) => {
                const active = selected.includes(folder.id);
                return (
                  <div
                    key={folder.id}
                    className={`interview-source ${active ? "selected" : ""}`}
                  >
                    <label>
                      <input
                        type="checkbox"
                        checked={active}
                        onChange={() =>
                          setSelected((ids) =>
                            active
                              ? ids.filter((id) => id !== folder.id)
                              : [...ids, folder.id],
                          )
                        }
                      />
                      <span className="interview-source-icon">
                        <Layers3 size={21} />
                      </span>
                      <span>
                        <strong>{folder.title}</strong>
                        <small>{folder.material_count} questions</small>
                      </span>
                      <Check size={18} className="interview-source-check" />
                    </label>
                    {active && !!folder.topics?.length && (
                      <div className="interview-topic-list">
                        {folder.topics
                          .filter((t) => t.name)
                          .map((topic) => (
                            <button
                              type="button"
                              key={topic.name}
                              className={
                                (topics[folder.id] || []).includes(topic.name)
                                  ? "active"
                                  : ""
                              }
                              onClick={() =>
                                setTopics((values) => ({
                                  ...values,
                                  [folder.id]: (
                                    values[folder.id] || []
                                  ).includes(topic.name)
                                    ? values[folder.id].filter(
                                        (name) => name !== topic.name,
                                      )
                                    : [
                                        ...(values[folder.id] || []),
                                        topic.name,
                                      ],
                                }))
                              }
                            >
                              {topic.name}
                            </button>
                          ))}
                      </div>
                    )}
                  </div>
                );
              })}
            </fieldset>
          )}
        </div>
        <aside className="interview-session-options">
          <span className="eyebrow">Your session</span>
          <h2>Make room to think.</h2>
          <p>
            Explain in your own words. You decide whether your answer was
            correct. Every mode is practice only — your SRS schedule stays unchanged.
          </p>
          <label className="interview-label">Interview mode
            <select value={config.interview_mode} disabled={busy || !!startCommand.current} onChange={e => setConfig({...config,interview_mode:e.target.value as InterviewGraphConfig["interview_mode"],custom_weights:config.custom_weights || {go:5,sql:3,http:2,architecture:1,messaging:1,ops:1,other:1}})}>
              <option value="real">Real Interview</option><option value="balanced">Balanced</option><option value="custom">Custom</option><option value="deep">Deep Interview</option>
            </select>
          </label>
          <p className="interview-mode-help">{{real:"A Go backend preset, adjusted to your selected subjects. Broad coverage with short branches.",balanced:"Give every available subject an equal share of the interview.",custom:"Set relative weights. For example, 5 / 3 / 2 means 50% / 30% / 20%.",deep:"Fewer starting topics, longer connected branches and more challenging follow-ups."}[config.interview_mode]}</p>
          {config.interview_mode === "deep" && <label className="interview-label">Depth level
            <select value={config.depth_level} disabled={busy || !!startCommand.current} onChange={e => setConfig({...config,depth_level:+e.target.value})}>
              <option value={1}>1 — Focused</option><option value={2}>2 — In depth</option><option value={3}>3 — Expert deep dive</option>
            </select>
          </label>}
          {config.interview_mode === "custom" && <fieldset className="interview-weights" disabled={busy || !!startCommand.current}>
            <legend>Relative topic weights</legend>
            {Object.entries({go:"Go",sql:"SQL",http:"HTTP / Networks",architecture:"Architecture",messaging:"Messaging",ops:"Testing / Ops",other:"Other"}).filter(([key])=>availableTopicKeys.includes(key)).map(([key,label]) => <label key={key}>{label}<input aria-label={`${label} weight`} type="number" min={0} max={1000000} step="any" value={config.custom_weights?.[key] ?? 0} onChange={e => setConfig({...config,custom_weights:{...config.custom_weights,[key]:+e.target.value}})} /></label>)}
            <small>Only topics with eligible questions participate. Zero excludes a topic.</small>
          </fieldset>}
          <label className="interview-label">
            Questions
            <input
              type="number"
              min={1}
              max={100}
              value={config.question_limit}
              disabled={busy || !!startCommand.current}
              onChange={(e) =>
                setConfig((value) => ({
                  ...value,
                  question_limit: +e.target.value,
                }))
              }
            />
          </label>
          <label className="interview-label">Interview profile
            <select value={config.profile} disabled={busy || !!startCommand.current} onChange={e => setConfig({...config,profile:e.target.value})}>
              {interviewProfiles.map(([slug,label]) => <option value={slug} key={slug}>{label}</option>)}
            </select>
          </label>
          <label className="interview-label">Interview level
            <select value={config.level} disabled={busy || !!startCommand.current} onChange={e => setConfig({...config,level:+e.target.value})}>
              {interviewLevels.map((label,i) => <option value={i+1} key={label}>{label}</option>)}
            </select>
          </label>
          <label className="interview-bank-mode">
            <input type="checkbox" checked={config.include_draft} disabled={busy || !!startCommand.current} onChange={e => setConfig({...config,include_draft:e.target.checked})} />
            <span><strong>Bank testing mode / Include drafts</strong><small>Allows draft questions for graph testing. Practice only; does not modify SRS.</small></span>
          </label>
          <button
            className="interview-advanced-toggle"
            type="button"
            onClick={() => setAdvanced(!advanced)}
          >
            <Settings2 size={16} /> Session options <ChevronDown size={15} />
          </button>
          {advanced && (
            <div className="interview-advanced">
              {(
                [
                  ["max_depth_per_branch", "Maximum depth", 1, 20],
                  ["max_forks_per_root", "Branches per topic", 0, 10],
                ] as const
              ).map(([key, label, min, max]) => (
                <label className="interview-label" key={key}>
                  {label}
                  <input
                    type="number"
                    min={min}
                    max={max}
                    disabled={busy || !!startCommand.current}
                    value={config[key]}
                    onChange={(e) =>
                      setConfig((value) => ({
                        ...value,
                        [key]: +e.target.value,
                      }))
                    }
                  />
                </label>
              ))}
            </div>
          )}
          {previewBusy && <p role="status">Checking available questions…</p>}
          {preview && <div className="interview-distribution" aria-label="Interview distribution">
            <strong>{preview.strategy.target_roots} root branches planned</strong>
            {preview.topics.map(topic => <div key={topic.key}><span>{topic.label}</span><b>{(topic.weight*100).toFixed(1)}%</b><small>{topic.roots} roots · {topic.available} available areas</small></div>)}
            <small>Percentages are relative shares. Root counts account for rounding and available areas.</small>
          </div>}
          {previewError && <ErrorBox error={previewError} />}
          {error && <ErrorBox error={error} />}
          {canResume && <button type="button" className="button" disabled={busy} onClick={() => {setBusy(true);void api<InterviewSession>("/training/mock-interviews/active").then(onStarted).catch(e=>setError(errorText(e))).finally(()=>setBusy(false));}}>Resume existing interview</button>}
          <button
            className="button primary interview-start"
            disabled={busy || !selected.length || previewBusy || !!previewError}
          >
            {busy
              ? "Preparing…"
              : startCommand.current
                ? "Retry start"
                : "Start interview"}
            <ArrowRight size={18} />
          </button>
          <small>
            Upcoming questions can appear for practice. Their review dates stay
            on schedule.
          </small>
        </aside>
      </form>
    </>
  );
}

function InterviewRun({
  initial,
  onSetup,
}: {
  initial: InterviewSession;
  onSetup: () => void;
}) {
  const { user } = useAuth();
  const pendingKey = `knowledge:interview:pending:${user.id}`;
  const [view, setView] = useState(initial);
  const [bankWarningDismissed, setBankWarningDismissed] = useState(false);
  const [revealedID, setRevealedID] = useState("");
  const [debug, setDebug] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const pending = useRef<GraphCommand | null>(
    readStored<GraphCommand>(pendingKey),
  );
  const lock = useRef(false);
  const card = view.current;
  const graph = view.graph?.state;
  const statistics = view.graph?.statistics;
  const selection = card?.interview_graph;
  const undoActions = view.undo_actions || [];
  async function send(command: GraphCommand) {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setError("");
    pending.current = command;
    saveStored(pendingKey, command);
    try {
      const response = await post<
        InterviewSession | { session: InterviewSession }
      >(
        `/training/sessions/${command.session_id}/${command.kind === "answer" ? "actions" : "undo"}`,
        command.body,
      );
      const next = "current" in response ? response : response.session;
      pending.current = null;
      saveStored(pendingKey, null);
      setView(next);
      setRevealedID("");
    } catch (e) {
      if (e instanceof ApiError && e.status >= 400 && e.status < 500) {
        pending.current = null;
        saveStored(pendingKey, null);
        if (e.status === 409) {
          try {
            setView(
              await api<InterviewSession>(
                `/training/sessions/${view.session.id}`,
              ),
            );
            setRevealedID("");
          } catch (refreshError) {
            setError(errorText(refreshError));
            return;
          }
        }
      }
      setError(errorText(e));
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  function answer(action: string) {
    if (!card || pending.current || busy) return;
    void send({
      session_id: view.session.id,
      kind: "answer",
      body: {
        command_id: crypto.randomUUID(),
        presentation_id: card.id,
        expected_version: card.progress_version,
        action,
      },
    });
  }
  async function finish() {
    if (lock.current || pending.current) return;
    lock.current = true;
    setBusy(true);
    setError("");
    try {
      setView(
        await post<InterviewSession>(
          `/training/sessions/${view.session.id}/finish`,
        ),
      );
    } catch (e) {
      setError(errorText(e));
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  useEffect(() => {
    if (pending.current?.session_id === view.session.id) {
      setError(
        "Your last action has not been confirmed. Retry to continue safely.",
      );
    }
  }, []);
  return (
    <>
      <div className="interview-run-top">
        <Link to="/" className="training-back">
          <ArrowLeft size={17} /> Library
        </Link>
        <span>
          <GitBranch size={16} /> Mock interview
        </span>
        {view.session.status === "active" && (
          <button
            className="button"
            disabled={busy || !!pending.current}
            onClick={() => void finish()}
          >
            End interview
          </button>
        )}
        <button
          className={`interview-debug-toggle ${debug ? "active" : ""}`}
          onClick={() => setDebug(!debug)}
          aria-pressed={debug}
        >
          Graph debug
        </button>
      </div>
      <div className="interview-run-layout">
        <main className="interview-conversation">
          {card ? (
            <>
              <div className="interview-question-heading">
                <span className="eyebrow">
                  Question{" "}
                  {graph?.questions_asked ||
                    view.summary.correct + view.summary.wrong + 1}
                </span>
                <span
                  className={`interview-credit ${selection?.review_credit === false ? "probe" : ""}`}
                >
                  {selection?.review_credit === false
                    ? "Practice question"
                    : "Scheduled review"}
                </span>
              </div>
              <article className="interview-question" key={card.id}>
                {card.question.map((field) => (
                  <Markdown
                    key={field.key}
                    value={field.value}
                    field={field.key}
                  />
                ))}
              </article>
              {graph?.config.include_draft && !bankWarningDismissed && <div className="interview-bank-warning" role="status"><strong>Bank testing mode</strong><span>Reference answers may be unfilled. All actions are practice only; your SRS schedule stays unchanged.</span><button type="button" aria-label="Dismiss bank testing warning" onClick={() => setBankWarningDismissed(true)}><X size={18} /></button></div>}
              {!graph?.config.include_draft && selection?.answer_incomplete && <p className="interview-bank-warning">Reference answer has not been filled in yet.</p>}
              <button
                type="button"
                className="interview-reveal"
                onClick={() =>
                  setRevealedID(revealedID === card.id ? "" : card.id)
                }
                aria-expanded={revealedID === card.id}
              >
                <BookOpen size={17} />
                {revealedID === card.id ? "Hide Answer" : "Reveal Answer"}
                <ChevronDown size={16} />
              </button>
              {revealedID === card.id && (
                <section className="interview-reference">
                  {card.answer.filter(field => ["short_answer","answer","sources"].includes(field.key)).map((field) => (
                    <div key={field.key}>
                      <h3>{{short_answer:"Short Answer",answer:"Detailed Answer",sources:"Source"}[field.key]}</h3>
                      <Markdown value={field.value} field={field.key} />
                    </div>
                  ))}
                </section>
              )}
              <div className="interview-grade">
                <button
                  className="button answer-wrong"
                  disabled={busy || !!pending.current}
                  onClick={() => answer("wrong")}
                >
                  <RotateCcw size={20} />
                  Wrong
                </button>
                <button
                  className="button answer-correct"
                  disabled={busy || !!pending.current}
                  onClick={() => answer("correct")}
                >
                  <Check size={21} />
                  Correct
                </button>
              </div>
              <button className="interview-next-route button" disabled={busy || !!pending.current} onClick={() => answer("next_route")}>
                <GitBranch size={18} /> Next Root <ArrowRight size={17} />
              </button>
            </>
          ) : (
            <div className="interview-finished">
              <span className="interview-finished-icon">
                <Sparkles size={35} />
              </span>
              <span className="eyebrow">Interview complete</span>
              <h1>A little more prepared.</h1>
              <p>
                {view.summary.correct + view.summary.wrong
                  ? "Your answers have been saved. Take a moment to review your path."
                  : "There are no ready questions in these sources yet. Add answers and mark questions ready to begin."}
              </p>
              <button className="button primary" disabled={busy || !!pending.current} onClick={onSetup}>
                Choose another interview
                <ArrowRight size={17} />
              </button>
            </div>
          )}
          {error && (
            <ErrorBox
              error={error}
              retry={
                pending.current ? () => void send(pending.current!) : undefined
              }
            />
          )}
          {!!undoActions.length && (
            <button
              className="interview-undo"
              disabled={busy || !!pending.current}
              onClick={() =>
                void send({
                  session_id: view.session.id,
                  kind: "undo",
                  body: {
                    command_id: crypto.randomUUID(),
                    event_id: undoActions[0],
                  },
                })
              }
            >
              <RotateCcw size={16} /> Undo last action{" "}
              <span>{Math.min(3, undoActions.length)} available</span>
            </button>
          )}
        </main>
        <aside className="interview-progress">
          <span className="eyebrow">Your conversation</span>
          <h2>
            {view.summary.correct + view.summary.wrong}
            <small>questions answered</small>
          </h2>
          <div className="interview-progress-scores">
            <span>
              Correct <b>{view.summary.correct}</b>
            </span>
            <span>
              Wrong <b>{view.summary.wrong}</b>
            </span>
          </div>
          {graph && (
            <div className="interview-progress-detail">
              <span>Shown <b>{graph.questions_asked}</b></span>
              <span>Answered <b>{graph.answered_questions ?? view.summary.correct + view.summary.wrong} / {graph.config.question_limit}</b></span>
              <span>Root slot <b>{graph.current_root} / {graph.interview_plan?.strategy.target_roots ?? graph.config.max_roots}</b></span>
              <span>Completed roots <b data-testid="completed-roots">{graph.completed_root_ids?.length ?? 0}</b></span>
              <span>Skipped roots <b>{graph.skipped_root_ids?.length ?? 0}</b></span>
              <span>Mode <b>{graph.interview_plan?.mode ?? "Graph"}{graph.interview_plan?.mode === "deep" ? ` · Depth ${graph.interview_plan.depth_level}` : ""}</b></span>
              <span>Branch <b>{graph.current_branch}</b></span>
              <span>Depth <b>{graph.current_depth} / {graph.config.max_depth_per_branch}</b></span>
              <span>Profile <b>{interviewProfiles.find(([slug])=>slug===graph.config.profile)?.[1] || graph.config.profile}</b></span>
              <span>Level <b>{interviewLevels[graph.config.level-1]}</b></span>
              <span>
                Topics explored <b>{graph.roots_used}</b>
              </span>
              <span>
                Depth reached{" "}
                <b>{statistics?.max_depth ?? graph.current_depth}</b>
              </span>
              {statistics && (
                <>
                  <span>
                    Concepts covered{" "}
                    <b>{statistics.concept_coverage?.length || 0}</b>
                  </span>
                  <span>
                    Practice questions <b>{statistics.interview_probes}</b>
                  </span>
                </>
              )}
            </div>
          )}
          <p>
            Follow the connections. Clear reasoning matters more than a perfect
            answer.
          </p>
          {!card && !!statistics?.concept_coverage?.length && (
            <div className="interview-coverage">
              <h3>Concept coverage</h3>
              {statistics.concept_coverage.map((concept) => (
                <div key={concept.slug}>
                  <span>{concept.slug.replaceAll("_", " ")}</span>
                  <b>
                    {concept.correct} / {concept.asked}
                  </b>
                  <meter
                    min={0}
                    max={Math.max(1, concept.asked)}
                    value={concept.correct}
                  />
                </div>
              ))}
            </div>
          )}
        </aside>
      </div>
      {debug && <GraphDebug view={view} />}
    </>
  );
}

function GraphDebug({ view }: { view: InterviewSession }) {
  const current = view.current?.interview_graph;
  const graph = view.graph?.state;
  const selection = view.graph?.selection;
  return (
    <section className="interview-debug">
      <h2>
        <GitBranch size={18} /> Graph debug
      </h2>
      <div className="interview-debug-meta">
        <span>
          Root <b>{current?.root_index ?? graph?.current_root ?? "—"}</b>
        </span>
        <span>
          Depth{" "}
          <b>
            {current?.depth ?? graph?.current_depth ?? "—"} /{" "}
            {graph?.config.max_depth_per_branch ?? "—"}
          </b>
        </span>
        <span>
          Probe <b>{current ? String(current.probe) : "—"}</b>
        </span>
        <span>
          Review credit <b>{current ? String(current.review_credit) : "—"}</b>
        </span>
      </div>
      <p>
        {selection?.selection_reason ||
          "No selection explanation is available yet."}
      </p>
      {!!selection?.detected_concepts?.length && (
        <><h3>Routing concepts</h3>
        <div className="interview-concept-chips">
          {selection.detected_concepts.map((match, i) => (
            <span key={`${match.slug}-${i}`}>
              {match.slug}
              <small>{match.source}</small>
              <b>{match.strength.toFixed(2)}</b>
            </span>
          ))}
        </div>
        </>
      )}
      {!!selection?.candidates?.length && (
        <table>
          <thead>
            <tr>
              <th>Candidate</th>
              <th>Score</th>
              <th>Reason</th>
            </tr>
          </thead>
          <tbody>
            {selection.candidates.map((candidate) => (
              <tr key={candidate.material_id}>
                <td>
                  {candidate.seed_key ||
                    candidate.question ||
                    candidate.material_id}
                </td>
                <td>{candidate.score.toFixed(2)}</td>
                <td>{candidate.reason || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}
