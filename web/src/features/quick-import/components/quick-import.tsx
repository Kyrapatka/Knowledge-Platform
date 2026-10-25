import { useEffect, useRef, useState } from "react";
import { ApiError, errorText } from "../../../api";
import { useAuth } from "../../../auth";
import { Modal, ErrorBox } from "../../../ui";
import type { Folder } from "../../../types";
import { command, record, revalidate } from "../model/draft";
import type {
  ImportCommand,
  ImportDraft,
  ImportResult,
  Range,
} from "../model/types";
import {
  deleteDraft,
  loadDraft,
  saveDraft,
  storageKey,
  type SavedDraft,
} from "../model/storage";
import { validateImport, commitImport } from "../api/import";
import { CardEditor, Fragment } from "./card-editor";
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
  const { user } = useAuth();
  const key = storageKey(user.id, folder.id);
  const [input, setInput] = useState("");
  const [kind, setKind] = useState<"text" | "json">("text");
  const [draft, setDraft] = useState<ImportDraft | null>(null);
  const [step, setStep] = useState<"input" | "review" | "result">("input");
  const [selected, setSelected] = useState("");
  const [filter, setFilter] = useState("all");
  const [page, setPage] = useState(0);
  const [mobileEditor, setMobileEditor] = useState(false);
  const [error, setError] = useState("");
  const [storageError, setStorageError] = useState("");
  const [savedAt, setSavedAt] = useState(0);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [restore, setRestore] = useState<SavedDraft | null>(null);
  const [server, setServer] = useState<ImportResult | null>(null);
  const [prepared, setPrepared] = useState<{
    body: ImportCommand;
    preview: ImportResult;
    excluded: number;
    fragments: number;
  } | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const textRef = useRef<HTMLTextAreaElement>(null);
  const goRange = useRef<Range | null>(null);
  const guard = useRef(false);
  const finished = useRef(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const latest = useRef({ input, kind, draft });
  latest.current = { input, kind, draft };
  const frozen = busy || !!draft?.pending;
  const persist = async (d = latest.current.draft) => {
    await saveDraft({
      key,
      updatedAt: Date.now(),
      input: latest.current.input,
      kind: latest.current.kind,
      draft: d,
    });
    if (!finished.current) {
      setSavedAt(Date.now());
      setStorageError("");
    }
  };
  useEffect(() => {
    let alive = true;
    loadDraft(key)
      .then((s) => {
        if (alive) setRestore(s);
      })
      .catch(() => {
        if (alive) setStorageError("Local draft storage is unavailable.");
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [key]);
  useEffect(() => {
    if (loading || restore || finished.current || (!input && !draft)) return;
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
    const r = goRange.current;
    goRange.current = null;
    const t = textRef.current;
    t.focus();
    // Textareas normalize CRLF; draft offsets continue to refer to the original.
    const offset = (n: number) =>
      n - (input.slice(0, n).match(/\r\n/g)?.length ?? 0);
    t.setSelectionRange(offset(r.start), offset(r.end));
    const lineHeight = parseFloat(getComputedStyle(t).lineHeight) || 22;
    t.scrollTop = Math.max(0, (r.line - 4) * lineHeight);
  }, [step, input]);
  const close = async () => {
    if (finished.current || (!input && !draft)) {
      onClose();
      return;
    }
    if (
      !window.confirm(
        draft?.pending
          ? "The import result is unconfirmed. Keep the saved command and close? Reopen to retry safely."
          : "Close with unsaved materials? Your local draft will be kept for 7 days.",
      )
    )
      return;
    try {
      await persist();
      onClose();
    } catch (e) {
      setStorageError(errorText(e));
    }
  };
  const change = (edit: (d: ImportDraft) => void) => {
    if (!draft || frozen) return;
    const next = structuredClone(draft);
    edit(next);
    revalidate(next, folder);
    setDraft(next);
    setPrepared(null);
    setServer(null);
  };
  const parse = async () => {
    if (guard.current) return;
    if (
      draft?.history.length &&
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
      setDraft(d);
      setSelected(d.cards[0]?.id ?? "");
      setPage(0);
      setStep("review");
      setPrepared(null);
      setServer(null);
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
    if (
      input !== draft.source &&
      !window.confirm(
        "Return to the reviewed source? This replaces the unparsed input changes.",
      )
    )
      return;
    setInput(draft.source);
    setKind(draft.kind);
    goRange.current = range;
    setStep("input");
  };
  const upload = async (file?: File) => {
    if (!file) return;
    try {
      if (!file.name.toLowerCase().endsWith(".json"))
        throw new Error("Choose a .json file");
      if (file.size > 10 * 1024 * 1024) throw new Error("JSON exceeds 10 MiB");
      const source = new TextDecoder("utf-8", { fatal: true }).decode(
        await file.arrayBuffer(),
      );
      setInput(source);
      setError("");
    } catch (e) {
      setError(errorText(e));
    }
  };
  const preview = async (validOnly: boolean) => {
    if (!draft || guard.current || draft.pending) return;
    guard.current = true;
    setBusy(true);
    setError("");
    try {
      const selected = draft.cards.filter(
        (c) => !c.excluded && (!validOnly || c.status === "ready"),
      );
      if (!selected.length) throw new Error("No ready cards to import");
      const body = command(draft, selected);
      const out = await validateImport(folder.id, body);
      setServer(out);
      if (out.valid)
        setPrepared({
          body,
          preview: out,
          excluded: draft.cards.length - selected.length,
          fragments: draft.issues.filter(
            (i) =>
              i.blocking &&
              !i.resolved &&
              (!i.cardId || !selected.some((c) => c.id === i.cardId)),
          ).length,
        });
      else
        setError(
          "Server validation found issues. Correct them before importing.",
        );
    } catch (e) {
      setError(errorText(e));
    } finally {
      guard.current = false;
      setBusy(false);
    }
  };
  const commit = async () => {
    if (!draft || guard.current) return;
    const body = draft.pending ?? prepared?.body;
    if (!body) return;
    guard.current = true;
    setBusy(true);
    setError("");
    let pending = structuredClone(draft);
    pending.pending = body;
    record(pending, "Submit immutable import command");
    setDraft(pending);
    try {
      // Persist before sending. A network error cannot leave an unknown command ID.
      await persist(pending);
      const out = await commitImport(folder.id, body);
      if (!out.valid)
        throw new Error("Server did not confirm a valid import result");
      finished.current = true;
      if (timer.current) clearTimeout(timer.current);
      setResult(out);
      setStep("result");
      setPrepared(null);
      setServer(out);
      try {
        await deleteDraft(key);
      } catch {
        setStorageError(
          "Import succeeded, but local draft cleanup failed. A retry of the saved command is safe.",
        );
      }
      onSaved();
    } catch (e) {
      setError(errorText(e));
      if (e instanceof ApiError && [400, 404, 422].includes(e.status)) {
        pending = { ...pending, pending: undefined };
        setDraft(pending);
        setPrepared(null);
        const response = e.data?.result as ImportResult | undefined;
        if (response) setServer(response);
        try {
          await persist(pending);
        } catch (err) {
          setStorageError(errorText(err));
        }
      }
    } finally {
      guard.current = false;
      setBusy(false);
    }
  };
  const active = draft?.cards.filter((c) => !c.excluded) ?? [];
  const unresolved =
    draft?.issues.filter(
      (i) =>
        i.blocking &&
        !i.resolved &&
        (!i.cardId || active.some((c) => c.id === i.cardId)),
    ) ?? [];
  const fatal = draft?.issues.some((i) => fatalCodes.has(i.code)) ?? false;
  const cards =
    draft?.cards.filter(
      (c) =>
        filter === "all" ||
        (filter === "ready" && c.status === "ready") ||
        (filter === "issues" &&
          (c.status === "invalid" ||
            c.status === "needs_review" ||
            !!c.duplicateOf ||
            server?.issues.some((i) => i.client_id === c.id))),
    ) ?? [];
  const card = draft?.cards.find((c) => c.id === selected);
  const select = (id: string) => {
    setSelected(id);
    setMobileEditor(true);
  };
  return (
    <Modal
      title="Import materials"
      subtitle={folder.title}
      wide
      onClose={() => void close()}
    >
      <div className="quick-import">
        <nav className="qi-steps" aria-label="Import steps">
          <span aria-current={step === "input" ? "step" : undefined}>
            1 Input
          </span>
          <span aria-current={step === "review" ? "step" : undefined}>
            2 Review
          </span>
          <span aria-current={step === "result" ? "step" : undefined}>
            3 Result
          </span>
        </nav>
        {error && <ErrorBox error={error} />}
        {storageError && <p role="alert">{storageError}</p>}
        {savedAt > 0 && !result && (
          <small className="muted" role="status">
            Local draft saved · 7-day recovery
          </small>
        )}
        {loading ? (
          <p>Loading local draft…</p>
        ) : restore ? (
          <section aria-label="Restore draft">
            <h3>Restore saved import?</h3>
            <p>
              A local draft for this account and folder is available
              {restore.draft?.pending
                ? " with an unconfirmed import command"
                : ""}
              .
            </p>
            <div className="button-row">
              <button
                className="button primary"
                onClick={() => {
                  setInput(restore.input);
                  setKind(restore.kind);
                  setDraft(
                    restore.draft ? revalidate(restore.draft, folder) : null,
                  );
                  setSelected(restore.draft?.cards[0]?.id ?? "");
                  setStep(restore.draft ? "review" : "input");
                  setRestore(null);
                }}
              >
                Restore draft
              </button>
              <button
                className="button"
                disabled={!!restore.draft?.pending}
                onClick={() => {
                  void deleteDraft(key)
                    .then(() => setRestore(null))
                    .catch((e) => setStorageError(errorText(e)));
                }}
              >
                Discard saved draft
              </button>
            </div>
          </section>
        ) : (
          <>
            {step === "input" && (
              <fieldset disabled={frozen} className="qi-form">
                <div className="button-row">
                  <button
                    className={`button ${kind === "text" ? "primary" : ""}`}
                    onClick={() => setKind("text")}
                  >
                    Paste text
                  </button>
                  <button
                    className={`button ${kind === "json" ? "primary" : ""}`}
                    onClick={() => setKind("json")}
                  >
                    Upload JSON
                  </button>
                </div>
                {kind === "json" && (
                  <label className="qi-field">
                    JSON file · UTF-8 · up to 10 MiB
                    <input
                      type="file"
                      accept=".json,application/json"
                      onChange={(e) => void upload(e.target.files?.[0])}
                    />
                  </label>
                )}
                <label className="qi-field">
                  {kind === "text" ? "Notes · up to 1 MiB" : "JSON source"}
                  <textarea
                    ref={textRef}
                    className="qi-source"
                    wrap="off"
                    aria-label="Import source"
                    rows={16}
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    placeholder={
                      folder.template_key === "english_words"
                        ? "subtle - тонкий\nThere is a subtle difference."
                        : "Question: What is a goroutine?\nShort answer: A lightweight concurrent function."
                    }
                  />
                </label>
                <div className="button-row">
                  <button
                    className="button primary"
                    disabled={!input.trim()}
                    onClick={parse}
                  >
                    {busy
                      ? "Parsing…"
                      : draft
                        ? "Reparse source"
                        : "Review import"}
                  </button>
                  {draft && (
                    <button
                      className="button"
                      onClick={() => setStep("review")}
                    >
                      Back to review
                    </button>
                  )}
                </div>
              </fieldset>
            )}
            {step === "review" && draft && (
              <>
                <div className="qi-counts" aria-live="polite">
                  <span>{draft.cards.length} cards</span>
                  <span>
                    {draft.cards.reduce((n, c) => n + c.examples.length, 0)}{" "}
                    examples
                  </span>
                  <span>
                    {draft.cards.reduce((n, c) => n + c.validation.length, 0) +
                      draft.issues.filter(
                        (i) => i.severity === "error" && !i.resolved,
                      ).length}{" "}
                    errors
                  </span>
                  <span>
                    {
                      draft.issues.filter(
                        (i) => i.severity === "warning" && !i.resolved,
                      ).length
                    }{" "}
                    warnings
                  </span>
                  <span>
                    {draft.cards.filter((c) => c.duplicateOf).length} batch
                    duplicates
                  </span>
                  <span>
                    {server
                      ? server.items.filter(
                          (i) => i.duplicate_kind === "existing",
                        ).length
                      : "Unchecked"}{" "}
                    existing duplicates
                  </span>
                </div>
                <fieldset disabled={frozen} className="qi-form">
                  <div className="button-row">
                    <button
                      className="button small"
                      onClick={() => setStep("input")}
                    >
                      Edit source
                    </button>
                    <button
                      className="button small"
                      onClick={() => {
                        change((d) => {
                          record(d, "Revalidate edited cards");
                        });
                        void preview(false);
                      }}
                    >
                      Revalidate
                    </button>
                    <label>
                      Show
                      <select
                        aria-label="Review filter"
                        value={filter}
                        onChange={(e) => {
                          setFilter(e.target.value);
                          setPage(0);
                        }}
                      >
                        <option value="all">All</option>
                        <option value="issues">Issues</option>
                        <option value="ready">Ready</option>
                      </select>
                    </label>
                  </div>
                  <div
                    className={`qi-review ${mobileEditor ? "qi-mobile-editor" : ""}`}
                  >
                    <aside className="qi-list">
                      <ol>
                        {cards.slice(page * 25, (page + 1) * 25).map((c) => (
                          <li key={c.id}>
                            <button
                              aria-current={
                                c.id === selected ? true : undefined
                              }
                              onClick={() => select(c.id)}
                            >
                              <strong>
                                {c.values.foreign ??
                                  c.values.question ??
                                  "Incomplete card"}
                              </strong>
                              <small>
                                {c.status}
                                {c.duplicateOf ? " · duplicate" : ""}
                              </small>
                            </button>
                          </li>
                        ))}
                      </ol>
                      <div className="button-row">
                        <button
                          className="button small"
                          disabled={page === 0}
                          onClick={() => setPage(page - 1)}
                        >
                          Previous cards
                        </button>
                        <span>
                          {page + 1}/{Math.max(1, Math.ceil(cards.length / 25))}
                        </span>
                        <button
                          className="button small"
                          disabled={(page + 1) * 25 >= cards.length}
                          onClick={() => setPage(page + 1)}
                        >
                          Next cards
                        </button>
                      </div>
                    </aside>
                    <div className="qi-detail">
                      <button
                        className="button small qi-back"
                        onClick={() => setMobileEditor(false)}
                      >
                        Back to cards
                      </button>
                      {card ? (
                        <CardEditor
                          key={card.id}
                          draft={draft}
                          folder={folder}
                          card={card}
                          change={change}
                          select={select}
                          source={source}
                        />
                      ) : (
                        <p>Select a card to review its fields.</p>
                      )}
                    </div>
                  </div>
                  {draft.issues.some((i) => !i.cardId) && (
                    <section aria-label="Unassigned fragments">
                      <h3>Source fragments requiring a decision</h3>
                      {draft.issues
                        .filter((i) => !i.cardId)
                        .map((i) => (
                          <Fragment
                            key={i.id}
                            issue={i}
                            draft={draft}
                            folder={folder}
                            change={change}
                            source={source}
                          />
                        ))}
                    </section>
                  )}
                  <details>
                    <summary>Edit history · {draft.history.length}</summary>
                    <ol>
                      {draft.history.map((h, i) => (
                        <li key={i}>{h.action}</li>
                      ))}
                    </ol>
                  </details>
                  <div className="button-row qi-actions">
                    <button
                      className="button primary"
                      disabled={
                        fatal ||
                        active.length === 0 ||
                        active.some((c) => c.status !== "ready") ||
                        unresolved.length > 0
                      }
                      onClick={() => void preview(false)}
                    >
                      Import all
                    </button>
                    <button
                      className="button"
                      disabled={
                        fatal || !active.some((c) => c.status === "ready")
                      }
                      onClick={() => void preview(true)}
                    >
                      Import valid only
                    </button>
                  </div>
                </fieldset>
                {server && server.issues.length > 0 && (
                  <section aria-label="Server validation">
                    <h3>Server validation</h3>
                    <ul>
                      {server.issues.map((i, n) => (
                        <li key={n}>
                          {i.path}: {i.message}
                          {i.client_id && (
                            <button
                              className="qi-source-link"
                              disabled={frozen}
                              onClick={() => select(i.client_id!)}
                            >
                              Open card
                            </button>
                          )}
                        </li>
                      ))}
                    </ul>
                  </section>
                )}
                {prepared && !draft.pending && (
                  <section className="qi-confirm" aria-label="Confirm import">
                    <h3>Confirm import into {folder.title}</h3>
                    <p>
                      {prepared.preview.created} will be created ·{" "}
                      {prepared.preview.skipped} duplicates skipped ·{" "}
                      {prepared.excluded} cards excluded · {prepared.fragments}{" "}
                      unresolved fragments excluded.
                    </p>
                    <p>Only the reviewed selected set will be submitted.</p>
                    <button
                      className="button primary"
                      disabled={busy}
                      onClick={() => void commit()}
                    >
                      Confirm import
                    </button>
                  </section>
                )}
                {draft.pending && (
                  <section className="qi-confirm" aria-label="Pending import">
                    <h3>
                      {busy ? "Importing…" : "Import result is unconfirmed"}
                    </h3>
                    <p>
                      The saved command and card values are frozen. Retry to
                      retrieve the confirmed result without creating a second
                      batch.
                    </p>
                    <button
                      className="button primary"
                      disabled={busy}
                      onClick={() => void commit()}
                    >
                      Retry same import
                    </button>
                  </section>
                )}
              </>
            )}
            {step === "result" && result && (
              <section aria-label="Import result">
                <h3>Import confirmed</h3>
                <p>
                  {result.created} created · {result.skipped} skipped ·{" "}
                  {result.failed} failed
                </p>
                {result.issues.length > 0 && (
                  <ul>
                    {result.issues.map((i, n) => (
                      <li key={n}>
                        {i.path}: {i.message}
                      </li>
                    ))}
                  </ul>
                )}
                <p>The folder and library counts have been refreshed.</p>
                <button className="button primary" onClick={onClose}>
                  Done
                </button>
              </section>
            )}
          </>
        )}
      </div>
    </Modal>
  );
}
