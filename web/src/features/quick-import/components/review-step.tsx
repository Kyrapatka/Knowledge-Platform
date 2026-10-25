import { useEffect, useMemo, useRef, useState } from "react";
import type { ImportDraft, ImportFolder, Range } from "../model/types";
import {
  filteredCards,
  summarize,
  type Diagnostic,
} from "../model/diagnostics";
import { CardEditor, Fragment } from "./card-editor";
export function ReviewStep({
  draft,
  folder,
  diagnostics,
  frozen,
  change,
  source,
  onInput,
  undo,
  redo,
  canUndo,
  canRedo,
  endGroup,
}: {
  draft: ImportDraft;
  folder: ImportFolder;
  diagnostics: Diagnostic[];
  frozen: boolean;
  change: (fn: (d: ImportDraft) => void, group?: string) => void;
  source: (r: Range) => void;
  onInput: () => void;
  undo: () => void;
  redo: () => void;
  canUndo: boolean;
  canRedo: boolean;
  endGroup: () => void;
}) {
  const [filter, setFilter] = useState("all"),
    [selected, setSelected] = useState(draft.cards[0]?.id ?? ""),
    [page, setPage] = useState(0),
    [mobile, setMobile] = useState(false),
    [focus, setFocus] = useState<Diagnostic | null>(null);
  const listScroll = useRef(0),
    root = useRef<HTMLDivElement>(null);
  const summary = useMemo(
    () => summarize(draft, diagnostics),
    [draft, diagnostics],
  );
  const cards = useMemo(
    () => filteredCards(draft, diagnostics, filter),
    [draft, diagnostics, filter],
  );
  const visible = cards.slice(page * 25, (page + 1) * 25);
  const card = cards.find((c) => c.id === selected);
  useEffect(() => {
    if (!cards.some((c) => c.id === selected)) {
      setSelected(cards[0]?.id ?? "");
      setPage(0);
      setMobile(false);
    } else {
      const i = cards.findIndex((c) => c.id === selected);
      setPage(Math.floor(i / 25));
    }
  }, [cards, selected]);
  useEffect(() => {
    if (!focus) return;
    const frame = requestAnimationFrame(() => {
      const candidates = Array.from(
        root.current?.querySelectorAll<HTMLElement>(
          "[data-field], [data-diagnostic]",
        ) ?? [],
      );
      const target =
        candidates.find(
          (e) => e.dataset.field === focus.field && !!focus.field,
        ) ?? candidates.find((e) => e.dataset.diagnostic === focus.id);
      if (target) {
        let p = target.parentElement;
        while (p) {
          if (p instanceof HTMLDetailsElement) p.open = true;
          p = p.parentElement;
        }
        target.focus({ preventScroll: true });
        target.scrollIntoView({ block: "center", behavior: "smooth" });
      }
    });
    return () => cancelAnimationFrame(frame);
  }, [focus, selected, mobile]);
  const select = (id: string) => {
    if (filter !== "all" && !cards.some((c) => c.id === id)) setFilter("all");
    listScroll.current =
      root.current?.closest(".dialog-content")?.scrollTop ?? 0;
    setSelected(id);
    setMobile(true);
  };
  const navigate = (delta: number) => {
    const problems = summary.active;
    if (!problems.length) return;
    let i = problems.findIndex((d) => d.id === focus?.id);
    if (i < 0) i = delta > 0 ? -1 : 0;
    const d = problems[(i + delta + problems.length) % problems.length];
    setFilter("issues");
    setFocus(d);
    if (d.cardId) select(d.cardId);
    else {
      setMobile(false);
      requestAnimationFrame(() =>
        root.current
          ?.querySelector("[data-global-issues]")
          ?.scrollIntoView({ block: "center" }),
      );
    }
  };
  const changeFilter = (f: string) => {
    setFilter(f);
    setFocus(null);
    setMobile(false);
    endGroup();
  };
  return (
    <div ref={root}>
      <div className="qi-summary" aria-live="polite">
        <strong>
          {draft.cards.length} {draft.cards.length === 1 ? "card" : "cards"} ·{" "}
          {summary.readyCards.length} ready
        </strong>
        {summary.issueCards.length > 0 && (
          <button
            className="qi-source-link"
            onClick={() => {
              changeFilter("issues");
              navigate(1);
            }}
          >
            {summary.issueCards.length}{" "}
            {summary.issueCards.length === 1 ? "card needs" : "cards need"}{" "}
            attention
          </button>
        )}
      </div>
      <div className="qi-toolbar">
        <div className="qi-tabs" role="group" aria-label="Review filter">
          {["all", "issues", "ready"].map((f) => (
            <button
              key={f}
              className="button"
              aria-pressed={filter === f}
              onClick={() => changeFilter(f)}
            >
              {f === "all"
                ? `All ${draft.cards.length}`
                : f === "issues"
                  ? `Issues ${summary.issueCards.length + summary.globalIssues.length}`
                  : `Ready ${summary.readyCards.length}`}
            </button>
          ))}
        </div>
        <div className="button-row">
          <button
            className="button"
            disabled={frozen || !canUndo}
            onClick={undo}
          >
            Undo
          </button>
          <button
            className="button"
            disabled={frozen || !canRedo}
            onClick={redo}
          >
            Redo
          </button>
          <button className="button" disabled={frozen} onClick={onInput}>
            Edit source
          </button>
        </div>
      </div>
      {!!summary.active.length && (
        <div className="qi-problem-nav">
          <span>
            {summary.active.length}{" "}
            {summary.active.length === 1 ? "diagnostic" : "diagnostics"}
            {focus
              ? ` · ${summary.active.findIndex((x) => x.id === focus.id) + 1} of ${summary.active.length}`
              : ""}
          </span>
          <div className="button-row">
            <button className="button small" onClick={() => navigate(-1)}>
              Previous problem
            </button>
            <button className="button small" onClick={() => navigate(1)}>
              Next problem
            </button>
          </div>
        </div>
      )}
      <fieldset disabled={frozen} className="qi-form">
        <div className={`qi-review ${mobile ? "qi-mobile-editor" : ""}`}>
          <aside className="qi-list" aria-label="Materials">
            <ol>
              {visible.map((c) => (
                <li key={c.id}>
                  <button
                    aria-current={selected === c.id ? true : undefined}
                    onClick={() => select(c.id)}
                  >
                    <strong>
                      {c.values.foreign ||
                        c.values.question ||
                        "Incomplete card"}
                    </strong>
                    <small>
                      {c.excluded
                        ? "Excluded"
                        : summary.issueCards.some((x) => x.id === c.id)
                          ? "Needs attention"
                          : diagnostics.some(
                                (d) =>
                                  d.cardId === c.id && d.code === "duplicate",
                              )
                            ? "Repeat"
                            : "Ready"}
                    </small>
                  </button>
                </li>
              ))}
            </ol>
            {!cards.length && (
              <p className="qi-empty">
                {filter === "issues"
                  ? "No cards need attention."
                  : filter === "ready"
                    ? "Fix the highlighted problems to prepare cards."
                    : "No materials found. Edit the source to add some."}
              </p>
            )}
            {cards.length > 25 && (
              <div className="qi-pagination">
                <button
                  className="button small"
                  disabled={page === 0}
                  onClick={() => {
                    setSelected(cards[(page - 1) * 25].id);
                    setMobile(false);
                  }}
                >
                  Previous cards
                </button>
                <span>
                  {page + 1}/{Math.ceil(cards.length / 25)}
                </span>
                <button
                  className="button small"
                  disabled={(page + 1) * 25 >= cards.length}
                  onClick={() => {
                    setSelected(cards[(page + 1) * 25].id);
                    setMobile(false);
                  }}
                >
                  Next cards
                </button>
              </div>
            )}
          </aside>
          <div className="qi-detail">
            <button
              className="button qi-back"
              onClick={() => {
                setMobile(false);
                requestAnimationFrame(() => {
                  const e = root.current?.closest(".dialog-content");
                  if (e) e.scrollTop = listScroll.current;
                });
              }}
            >
              Back to cards
            </button>
            {card ? (
              <CardEditor
                key={card.id}
                draft={draft}
                folder={folder}
                card={card}
                diagnostics={diagnostics.filter((d) => d.cardId === card.id)}
                change={change}
                endGroup={endGroup}
                select={select}
                source={source}
              />
            ) : (
              <p className="qi-empty">
                {filter === "issues"
                  ? "All visible card problems are resolved."
                  : "Select a card to review its fields."}
              </p>
            )}
          </div>
        </div>
        {filter !== "ready" && summary.globalIssues.length > 0 && (
          <section aria-label="Source problems" data-global-issues>
            <h3>Source needs attention</h3>
            {summary.globalIssues.map((d) => {
              const issue = draft.issues.find((i) => d.issueIds.includes(i.id));
              return issue ? (
                <Fragment
                  key={d.id}
                  issue={issue}
                  draft={draft}
                  folder={folder}
                  change={change}
                  source={source}
                />
              ) : (
                <article
                  key={d.id}
                  className="qi-issue"
                  data-diagnostic={d.id}
                  tabIndex={-1}
                >
                  <strong>{d.message}</strong>
                  <p>
                    {d.why} {d.action}
                  </p>
                  <button className="button" onClick={onInput}>
                    Edit source
                  </button>
                  <details>
                    <summary>Technical details</summary>
                    {d.code} · {d.path}
                  </details>
                </article>
              );
            })}
          </section>
        )}
      </fieldset>
    </div>
  );
}
