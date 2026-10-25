import { useEffect, useMemo, useRef, useState } from "react";
import { ApiError, errorText } from "../../../api";
import { useAuth } from "../../../auth";
import { Modal, ErrorBox } from "../../../ui";
import type { Folder } from "../../../types";
import { command, revalidate } from "../model/draft";
import type { ImportDraft, ImportResult, Range } from "../model/types";
import {
  deleteDraft,
  loadDraft,
  saveDraft,
  savedInput,
  storageKey,
  type SavedDraft,
} from "../model/storage";
import {
  diagnosticsFor,
  summarize,
  isCurrentPreview,
  type Preview,
} from "../model/diagnostics";
import {
  createHistory,
  recordEdit,
  undoEdit,
  redoEdit,
} from "../model/history";
import { validateImport, commitImport } from "../api/import";
import { InputStep } from "./input-step";
import { ReviewStep } from "./review-step";
import { ImportFooter, type PreparedImport } from "./import-footer";
import "../quick-import.css";
const fatalCodes = new Set([
  "source_limit",
  "template",
  "template_mismatch",
  "format_version",
  "invalid_json",
  "duplicate_json_key",
  "json_depth",
  "item_limit",
  "items_type",
  "item_type",
]);
export function QuickImport({
  folder,
  onClose,
  onSaved,
}: {
  folder: Folder;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { user } = useAuth(),
    key = storageKey(user.id, folder.id);
  const [input, setInput] = useState(""),
    [kind, setKind] = useState<"text" | "json">("text"),
    [draft, setDraft] = useState<ImportDraft | null>(null);
  const [step, setStep] = useState<"input" | "review" | "result">("input"),
    [original, setOriginal] = useState(false);
  const [error, setError] = useState(""),
    [storageError, setStorageError] = useState(""),
    [saved, setSaved] = useState(false),
    [busy, setBusy] = useState(false),
    [loading, setLoading] = useState(true);
  const [restore, setRestore] = useState<SavedDraft | null>(null),
    [server, setServer] = useState<Preview>(),
    [prepared, setPrepared] = useState<PreparedImport | null>(null),
    [result, setResult] = useState<ImportResult | null>(null);
  const [history, setHistory] = useState(createHistory),
    historyRef = useRef(history);
  const revision = useRef(0),
    savedRevision = useRef<number | null>(null),
    guard = useRef(false),
    finished = useRef(false),
    alive = useRef(true);
  const queue = useRef<Promise<unknown>>(Promise.resolve()),
    timer = useRef<ReturnType<typeof setTimeout> | null>(null),
    textRef = useRef<HTMLTextAreaElement>(null),
    goRange = useRef<Range | null>(null);
  const latest = useRef({ input, kind, draft });
  latest.current = { input, kind, draft };
  historyRef.current = history;
  const frozen = !!draft?.pending;
  const setCurrent = (d: ImportDraft | null) => {
    latest.current.draft = d;
    setDraft(d);
  };
  const clearPreview = () => {
    revision.current++;
    setPrepared(null);
    setServer(undefined);
    setError("");
    setSaved(false);
  };
  const persist = (d = latest.current.draft, confirmedCommandId?: string) => {
    const snapshot: SavedDraft = {
      key,
      updatedAt: Date.now(),
      input: latest.current.input,
      kind: latest.current.kind,
      draft: d,
    };
    const task = queue.current.then(async () => {
      if (finished.current) return;
      savedRevision.current = await saveDraft(
        snapshot,
        savedRevision.current,
        confirmedCommandId,
      );
      if (alive.current) {
        setSaved(true);
        setStorageError("");
      }
    });
    queue.current = task.catch(() => {});
    return task;
  };
  useEffect(() => {
    alive.current = true;
    let active = true;
    loadDraft(key)
      .then((s) => {
        if (active) {
          savedRevision.current = s?.revision ?? (s ? 0 : null);
          setRestore(s);
        }
      })
      .catch((e) => {
        if (active) setStorageError(errorText(e));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      alive.current = false;
    };
  }, [key]);
  useEffect(() => {
    if (
      loading ||
      restore ||
      finished.current ||
      (!input && !draft && savedRevision.current === null)
    )
      return;
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      if (!finished.current)
        void persist().catch((e) => setStorageError(errorText(e)));
    }, 500);
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, [input, kind, draft, loading, restore, key]);
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      if (!finished.current && (latest.current.input || latest.current.draft)) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, []);
  useEffect(() => {
    if (step !== "input" || !goRange.current || !textRef.current) return;
    const r = goRange.current,
      t = textRef.current;
    goRange.current = null;
    const src = original ? (draft?.source ?? input) : input;
    const offset = (n: number) =>
      n - (src.slice(0, n).match(/\r\n/g)?.length ?? 0);
    t.focus();
    t.setSelectionRange(offset(r.start), offset(r.end));
    t.scrollTop = Math.max(
      0,
      (r.line - 4) * (parseFloat(getComputedStyle(t).lineHeight) || 22),
    );
  }, [step, original, input, draft]);
  const endGroup = () => setHistory((h) => ({ ...h, group: undefined }));
  const change = (edit: (d: ImportDraft) => void, group?: string) => {
    const before = latest.current.draft;
    if (!before || before.pending) return;
    const next = {
      ...before,
      cards: structuredClone(before.cards),
      issues: structuredClone(before.issues),
      history: [...before.history],
    };
    edit(next);
    revalidate(next, folder);
    next.updatedAt = Date.now();
    next.history = next.history.slice(-5);
    setHistory(recordEdit(historyRef.current, before, next, { group }));
    setCurrent(next);
    clearPreview();
  };
  const historyAction = (redo = false) => {
    const d = latest.current.draft;
    if (!d || d.pending) return;
    const out = (redo ? redoEdit : undoEdit)(historyRef.current, d);
    setHistory(out.history);
    setCurrent(revalidate(out.draft, folder));
    clearPreview();
  };
  const close = async () => {
    if (guard.current) {
      setError("Please wait for the current check or import to finish.");
      return;
    }
    if (
      finished.current ||
      restore ||
      (!input && !draft && savedRevision.current === null)
    ) {
      onClose();
      return;
    }
    if (
      !window.confirm(
        draft?.pending
          ? "The import result is unconfirmed. Keep the saved operation and close? Reopen to retry safely."
          : "Close this import? Your draft will be kept on this device for 7 days.",
      )
    )
      return;
    if (timer.current) clearTimeout(timer.current);
    try {
      await persist();
      onClose();
    } catch (e) {
      setStorageError(errorText(e));
    }
  };
  const parse = async () => {
    if (guard.current || frozen) return;
    if (draft && input === draft.source && kind === draft.kind) {
      setStep("review");
      return;
    }
    if (
      draft &&
      !window.confirm(
        "Reparse replaces your reviewed edits with a new automatic interpretation. Continue?",
      )
    )
      return;
    guard.current = true;
    setBusy(true);
    setError("");
    const worker = new Worker(new URL("../parser/worker.ts", import.meta.url), {
      type: "module",
    });
    try {
      const d = await new Promise<ImportDraft>((resolve, reject) => {
        worker.onmessage = (e) =>
          e.data.error
            ? reject(new Error(e.data.error))
            : resolve(e.data.draft);
        worker.onerror = () =>
          reject(new Error("Source parser could not start"));
        worker.postMessage({ source: input, kind, folder });
      });
      setCurrent(d);
      setHistory(createHistory());
      clearPreview();
      setStep("review");
      setOriginal(false);
    } catch (e) {
      setError(errorText(e));
    } finally {
      worker.terminate();
      guard.current = false;
      setBusy(false);
    }
  };
  const source = (range: Range) => {
    if (!draft) return;
    goRange.current = range;
    setOriginal(input !== draft.source || kind !== draft.kind);
    setStep("input");
    endGroup();
  };
  const showInput = () => {
    setOriginal(false);
    setStep("input");
    endGroup();
  };
  const upload = async (file?: File) => {
    if (!file) return;
    try {
      if (!file.name.toLowerCase().endsWith(".json"))
        throw new Error("Choose a .json file");
      if (file.size > 10 * 1024 * 1024) throw new Error("JSON exceeds 10 MiB");
      setInput(
        new TextDecoder("utf-8", { fatal: true }).decode(
          await file.arrayBuffer(),
        ),
      );
      setError("");
      setSaved(false);
    } catch (e) {
      setError(errorText(e));
    }
  };
  const diagnostics = useMemo(
    () => (draft ? diagnosticsFor(draft, folder, server) : []),
    [draft, folder, server],
  );
  const summary = useMemo(
    () => (draft ? summarize(draft, diagnostics) : null),
    [draft, diagnostics],
  );
  const preview = async () => {
    const current = latest.current.draft;
    if (!current || guard.current || current.pending) return;
    const snapshot = structuredClone(current),
      version = revision.current;
    const ready = summarize(
      snapshot,
      diagnosticsFor(snapshot, folder, server),
    ).readyCards;
    if (!ready.length) return;
    const body = command(snapshot, ready);
    guard.current = true;
    setBusy(true);
    setError("");
    endGroup();
    try {
      const out = await validateImport(folder.id, body);
      if (!alive.current || !isCurrentPreview(version, revision.current))
        return;
      setServer({ result: out, command: body });
      if (out.valid)
        setPrepared({
          body,
          preview: out,
          revision: version,
          excluded: snapshot.cards.length - ready.length,
          fragments: snapshot.issues.filter(
            (i) =>
              i.blocking &&
              !i.resolved &&
              (!i.cardId || !ready.some((c) => c.id === i.cardId)),
          ).length,
        });
      else
        setError(
          "Some materials need attention. Review the highlighted problems, then try importing again.",
        );
    } catch (e) {
      if (isCurrentPreview(version, revision.current)) setError(errorText(e));
    } finally {
      guard.current = false;
      if (alive.current) setBusy(false);
    }
  };
  const commit = async () => {
    const current = latest.current.draft;
    if (!current || guard.current) return;
    if (!current.pending && prepared?.revision !== revision.current) return;
    const body = current.pending ?? prepared?.body;
    if (!body) return;
    guard.current = true;
    setBusy(true);
    setError("");
    if (timer.current) clearTimeout(timer.current);
    const pending = { ...current, pending: body };
    setCurrent(pending);
    let sent = false;
    try {
      await persist(pending);
      sent = true;
      const out = await commitImport(folder.id, body);
      if (!out.valid)
        throw new Error(
          "The server did not confirm the operation. Retry the saved command.",
        );
      finished.current = true;
      if (timer.current) clearTimeout(timer.current);
      await queue.current;
      setResult(out);
      setStep("result");
      setPrepared(null);
      try {
        await deleteDraft(key, savedRevision.current, body.command_id);
        savedRevision.current = null;
      } catch (e) {
        setStorageError(
          `Import succeeded. Local recovery cleanup failed: ${errorText(e)}. Replaying the saved command is safe.`,
        );
      }
      onSaved();
    } catch (e) {
      setError(errorText(e));
      if (!sent && !current.pending) {
        setCurrent(current);
        setStorageError(
          "The operation was not sent because its recovery copy could not be saved. " +
            errorText(e),
        );
      } else if (e instanceof ApiError && [400, 404, 422].includes(e.status)) {
        const next = { ...pending, pending: undefined };
        try {
          await persist(next, body.command_id);
          setCurrent(next);
          setPrepared(null);
          const out = e.data?.result as ImportResult | undefined;
          if (out) setServer({ result: out, command: body });
        } catch (err) {
          setStorageError(errorText(err));
        }
      }
    } finally {
      guard.current = false;
      setBusy(false);
    }
  };
  const fatal = draft?.issues.some((i) => fatalCodes.has(i.code)) ?? false;
  const footer =
    step === "review" && draft && !restore && !loading ? (
      <ImportFooter
        ready={summary?.readyCards.length ?? 0}
        total={draft.cards.length}
        excluded={draft.cards.filter((c) => c.excluded).length}
        fragments={summary?.globalIssues.length ?? 0}
        fatal={fatal}
        busy={busy}
        pending={!!draft.pending}
        prepared={prepared}
        saved={saved}
        onPreview={() => void preview()}
        onCommit={() => void commit()}
        onCancel={() => setPrepared(null)}
      />
    ) : undefined;
  return (
    <Modal
      title="Import materials"
      subtitle={folder.title}
      wide
      onClose={() => void close()}
      footer={footer}
    >
      <div className="quick-import">
        <nav className="qi-steps" aria-label="Import steps">
          {(["input", "review", "result"] as const).map((s, i) => (
            <button
              className="button"
              key={s}
              aria-current={step === s ? "step" : undefined}
              disabled={
                loading ||
                !!restore ||
                (s === "result"
                  ? !result
                  : !!result || (s === "review" && !draft) || frozen || busy)
              }
              onClick={() => {
                if (s === "input") showInput();
                else setStep(s);
              }}
            >
              {i + 1} {s[0].toUpperCase() + s.slice(1)}
            </button>
          ))}
        </nav>
        {error && <ErrorBox error={error} />}{" "}
        {storageError && <p role="alert">{storageError}</p>}
        {loading ? (
          <p role="status">Loading local draft…</p>
        ) : restore ? (
          <section aria-label="Restore draft">
            <h3>There is an unfinished import</h3>
            <p>
              {restore.draft?.pending
                ? "Its result is unconfirmed. Continue to recover the saved operation safely."
                : "Continue where you left off, or start again."}
            </p>
            <div className="button-row">
              <button
                className="button primary"
                onClick={() => {
                  setInput(savedInput(restore));
                  setKind(restore.kind ?? restore.draft?.kind ?? "text");
                  setCurrent(
                    restore.draft ? revalidate(restore.draft, folder) : null,
                  );
                  setStep(restore.draft ? "review" : "input");
                  setRestore(null);
                }}
              >
                Restore draft
              </button>
              <button
                className="button"
                disabled={!!restore.draft?.pending}
                onClick={() =>
                  void deleteDraft(key, savedRevision.current)
                    .then(() => {
                      savedRevision.current = null;
                      setRestore(null);
                    })
                    .catch((e) => setStorageError(errorText(e)))
                }
              >
                Start again
              </button>
            </div>
          </section>
        ) : (
          <>
            {step === "input" && (
              <InputStep
                input={original ? (draft?.source ?? input) : input}
                kind={original ? (draft?.kind ?? kind) : kind}
                busy={busy || frozen}
                hasDraft={!!draft}
                changed={
                  !!draft && (input !== draft.source || kind !== draft.kind)
                }
                original={original}
                template={folder.template_key}
                textRef={textRef}
                onInput={(s) => {
                  setInput(s);
                  setSaved(false);
                }}
                onKind={setKind}
                onUpload={(f) => void upload(f)}
                onParse={() => void parse()}
                onReview={() => setStep("review")}
                onEdited={() => setOriginal(false)}
              />
            )}
            {draft && !result && (
              <div hidden={step !== "review"}>
                <ReviewStep
                  key={draft.draftId}
                  draft={draft}
                  folder={folder}
                  diagnostics={diagnostics}
                  frozen={frozen}
                  change={change}
                  source={source}
                  onInput={showInput}
                  undo={() => historyAction()}
                  redo={() => historyAction(true)}
                  canUndo={!!history.past.length}
                  canRedo={!!history.future.length}
                  endGroup={endGroup}
                />
              </div>
            )}
            {step === "result" && result && (
              <section aria-label="Import result" className="qi-result">
                <h3>Import complete</h3>
                <p>
                  {result.created
                    ? `${result.created} cards added`
                    : "No new materials — matching cards are already in the folder."}
                </p>
                {result.skipped > 0 && (
                  <p>{result.skipped} duplicates skipped</p>
                )}
                {result.failed > 0 && (
                  <p>{result.failed} materials could not be added</p>
                )}
                <p className="muted">
                  The folder and library have been refreshed.
                </p>
                <button className="button primary" onClick={onClose}>
                  Back to folder
                </button>
              </section>
            )}
          </>
        )}
      </div>
    </Modal>
  );
}
