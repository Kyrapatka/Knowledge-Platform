import { createClientUUID } from "./uuid";
import { useQuestionScroll } from "./question-scroll";
import { useEffect, useRef, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Check,
  ChevronDown,
  GitBranch,
  RotateCcw,
  Settings2,
  Sparkles,
  X,
} from "lucide-react";
import { api, ApiError, errorText, post, readStored, saveStored } from "./api";
import { useAuth } from "./auth";
import { useLibrary } from "./library/context";
import type { Source } from "./types";
import { MockSetup } from "./mock-setup";
import { ErrorBox, Markdown, Spinner } from "./ui";
import {
  interviewProfiles,
  interviewLevels,
  type InterviewSession,
} from "./interview-types";
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
  const location = useLocation();
  const [initialSources, setInitialSources] = useState<Source[] | undefined>(
    () => location.state?.mockSources,
  );
  const explicitSetup = useRef(!!initialSources?.length);
  useEffect(() => {
    if (location.state?.mockSources?.length) {
      setInitialSources(location.state.mockSources);
      explicitSetup.current = true;
      setView(null);
      setLoading(false);
    }
  }, [location.key]);
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
      const restored = await api<InterviewSession>(
        sessionID
          ? `/training/sessions/${sessionID}`
          : "/training/mock-interviews/active",
      );
      if (restored.graph?.state.practice_only) setView(restored);
      else {
        // Old plan-backed graph sessions retain their API/SRS semantics,
        // but must never be presented as practice-only Mock Interview.
        saveStored(storageKey, null);
        setSessionID("");
        setView(null);
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
    if (explicitSetup.current) {
      setLoading(false);
      return;
    }
    void restore();
  }, [sessionID]);
  function opened(next: InterviewSession) {
    explicitSetup.current = false;
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
            explicitSetup.current = true;
            setInitialSources(undefined);
            setView(null);
            setSessionID("");
            saveStored(storageKey, null);
          }}
        />
      ) : (
        <MockSetup
          folders={(data?.folders || []).filter(
            (f) => f.template_key === "interview_questions",
          )}
          onStarted={opened}
          initialSources={initialSources}
        />
      )}
    </div>
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
  const questionContainer = useQuestionScroll(card?.id);
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
        command_id: createClientUUID(),
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
        <main className="interview-conversation" ref={questionContainer}>
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
              {graph?.config.include_draft && !bankWarningDismissed && (
                <div className="interview-bank-warning" role="status">
                  <strong>Bank testing mode</strong>
                  <span>
                    Reference answers may be unfilled. All actions are practice
                    only; your SRS schedule stays unchanged.
                  </span>
                  <button
                    type="button"
                    aria-label="Dismiss bank testing warning"
                    onClick={() => setBankWarningDismissed(true)}
                  >
                    <X size={18} />
                  </button>
                </div>
              )}
              {!graph?.config.include_draft && selection?.answer_incomplete && (
                <p className="interview-bank-warning">
                  Reference answer has not been filled in yet.
                </p>
              )}
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
                  {card.answer
                    .filter((field) =>
                      ["short_answer", "answer", "sources"].includes(field.key),
                    )
                    .map((field) => (
                      <div key={field.key} data-field={field.key}>
                        <h3>
                          {
                            {
                              short_answer: "Short Answer",
                              answer: "Detailed Answer",
                              sources: "Source",
                            }[field.key]
                          }
                        </h3>
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
              <button
                className="interview-next-route button"
                disabled={busy || !!pending.current}
                onClick={() => answer("next_route")}
              >
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
              <button
                className="button primary"
                disabled={busy || !!pending.current}
                onClick={onSetup}
              >
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
                    command_id: createClientUUID(),
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
              <span>
                Shown <b>{graph.questions_asked}</b>
              </span>
              <span>
                Answered{" "}
                <b>
                  {graph.answered_questions ??
                    view.summary.correct + view.summary.wrong}{" "}
                  / {graph.config.question_limit}
                </b>
              </span>
              <span>
                Root{" "}
                <b>
                  {graph.current_root}{" "}
                  <small>
                    (
                    {graph.interview_plan?.strategy.target_roots ??
                      graph.config.max_roots}{" "}
                    planned)
                  </small>
                </b>
              </span>
              <span>
                Completed roots{" "}
                <b data-testid="completed-roots">
                  {graph.completed_root_ids?.length ?? 0}
                </b>
              </span>
              <span>
                Skipped roots <b>{graph.skipped_root_ids?.length ?? 0}</b>
              </span>
              <span>
                Mode{" "}
                <b>
                  {graph.interview_plan?.mode ?? "Graph"}
                  {graph.interview_plan?.mode === "deep"
                    ? ` · Depth ${graph.interview_plan.depth_level}`
                    : ""}
                </b>
              </span>
              <span>
                Branch <b>{graph.current_branch}</b>
              </span>
              <span>
                Depth{" "}
                <b>
                  {graph.current_depth} / {graph.config.max_depth_per_branch}
                </b>
              </span>
              <span>
                Profile{" "}
                <b>
                  {interviewProfiles.find(
                    ([slug]) => slug === graph.config.profile,
                  )?.[1] || graph.config.profile}
                </b>
              </span>
              <span>
                Level <b>{interviewLevels[graph.config.level - 1]}</b>
              </span>
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
        <>
          <h3>Routing concepts</h3>
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
