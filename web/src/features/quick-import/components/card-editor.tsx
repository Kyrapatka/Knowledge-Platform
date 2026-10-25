import { createClientUUID } from "../../../uuid";
import { useState } from "react";
import type {
  DraftCard,
  ImportDraft,
  ImportFolder,
  Issue,
  Range,
} from "../model/types";
import {
  chooseBoundary,
  assign,
  merge,
  moveExample,
  record,
  split,
} from "../model/draft";
type Props = {
  draft: ImportDraft;
  folder: ImportFolder;
  card: DraftCard;
  change: (edit: (d: ImportDraft) => void) => void;
  select: (id: string) => void;
  source: (range: Range) => void;
};
export function Fragment({
  issue,
  draft,
  folder,
  change,
  source,
}: Omit<Props, "card" | "select"> & { issue: Issue }) {
  const [target, setTarget] = useState(
    issue.cardId ?? draft.cards.find((c) => !c.excluded)?.id ?? "",
  );
  const [field, setField] = useState(
    folder.template_key === "english_words" ? "example" : "note",
  );
  return (
    <article className="qi-issue" data-testid="import-fragment">
      <strong>{issue.code}</strong>
      <p>{issue.message}</p>
      <pre>{issue.fragment}</pre>
      <small>
        {issue.path ?? `Line ${issue.range.line}`} ·{" "}
        {issue.resolved ?? "Unresolved"}
      </small>
      {!!issue.alternatives?.length && (
        <ul>
          {issue.alternatives.map((a, i) => (
            <li key={i}>
              {draft.cards.find((c) => c.id === a.cardId)?.values.foreign ??
                a.cardId}
              : {a.kind} · evidence {a.score} ({a.reasons.join(", ")})
            </li>
          ))}
        </ul>
      )}
      {issue.code === "boundary_alternative" &&
        !issue.resolved &&
        issue.alternatives
          ?.filter((a) => a.kind === "example" && a.cardId)
          .map((a) => (
            <button
              className="button small"
              key={a.cardId}
              onClick={() =>
                change((d) => chooseBoundary(d, issue.id, a.cardId!))
              }
            >
              Use as example of{" "}
              {draft.cards.find((c) => c.id === a.cardId)?.values.foreign}
            </button>
          ))}
      <div className="button-row">
        <button className="button small" onClick={() => source(issue.range)}>
          Go to source
        </button>
        {!issue.resolved && (
          <>
            <button
              className="button small"
              onClick={() =>
                change((d) => {
                  d.issues.find((i) => i.id === issue.id)!.resolved =
                    "excluded";
                  record(d, `Exclude fragment ${issue.id}`);
                })
              }
            >
              Exclude fragment
            </button>
            {[
              "boundary_alternative",
              "weak_example",
              "duplicate_word",
            ].includes(issue.code) && (
              <button
                className="button small"
                onClick={() =>
                  change((d) => {
                    d.issues.find((i) => i.id === issue.id)!.resolved =
                      "accepted";
                    record(d, `Accept interpretation ${issue.id}`);
                  })
                }
              >
                Keep interpretation
              </button>
            )}
          </>
        )}
      </div>
      {!issue.resolved && draft.cards.length > 0 && (
        <div className="qi-assignment">
          <label>
            Assign to card
            <select
              aria-label={`Assign ${issue.id} to card`}
              value={target}
              onChange={(e) => setTarget(e.target.value)}
            >
              {draft.cards
                .filter((c) => !c.excluded)
                .map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.values.foreign ?? c.values.question ?? c.id}
                  </option>
                ))}
            </select>
          </label>
          <label>
            Field
            <select
              aria-label={`Field for ${issue.id}`}
              value={field}
              onChange={(e) => setField(e.target.value)}
            >
              <option value="">Choose field</option>
              {folder.config.schema.fields
                .filter((f) => f.active && f.key !== "transcription")
                .map((f) => (
                  <option key={f.key} value={f.key}>
                    {f.label ?? f.key}
                  </option>
                ))}
            </select>
          </label>
          <button
            className="button small"
            disabled={
              !target ||
              !folder.config.schema.fields.some(
                (f) => f.key === field && f.active && f.key !== "transcription",
              )
            }
            onClick={() => change((d) => assign(d, issue.id, target, field))}
          >
            Assign fragment
          </button>
        </div>
      )}
    </article>
  );
}
export function CardEditor({
  draft,
  folder,
  card,
  change,
  select,
  source,
}: Props) {
  const [target, setTarget] = useState("");
  const mutate = (fn: (c: DraftCard) => void, action: string) =>
    change((d) => {
      fn(d.cards.find((c) => c.id === card.id)!);
      record(d, `${card.id}: ${action}`);
    });
  const fields = folder.config.schema.fields.filter(
    (f) => f.active && !["transcription", "example"].includes(f.key),
  );
  const issues = draft.issues.filter((i) => i.cardId === card.id);
  return (
    <section className="qi-editor" aria-label="Card editor">
      <div className="button-row">
        <h3>{card.values.foreign ?? card.values.question ?? "New card"}</h3>
        <button className="button small" onClick={() => source(card.range)}>
          Go to source
        </button>
        <button
          className="button small"
          onClick={() =>
            mutate((c) => {
              c.excluded = !c.excluded;
            }, "Toggle exclusion")
          }
        >
          {card.excluded ? "Include card" : "Exclude card"}
        </button>
      </div>
      {card.validation.map((v) => (
        <p role="alert" key={v}>
          {v}
        </p>
      ))}
      {fields.map((f) => (
        <label className="qi-field" key={f.key}>
          {f.label ?? f.key}
          {f.required ? " *" : ""}
          <textarea
            aria-label={f.key}
            value={card.values[f.key] ?? ""}
            rows={f.key === "foreign" || f.key === "native" ? 2 : 3}
            onChange={(e) =>
              mutate((c) => {
                c.values[f.key] = e.target.value;
              }, `Edit ${f.key}`)
            }
          />
          {card.ranges[f.key] && (
            <button
              className="qi-source-link"
              onClick={() => source(card.ranges[f.key])}
            >
              Source line {card.ranges[f.key].line}
            </button>
          )}
        </label>
      ))}
      {folder.config.metadata_schema.fields
        .filter((f) => f.active)
        .map((f) => (
          <label className="qi-field" key={f.key}>
            {f.label ?? f.key}
            <input
              aria-label={`Metadata ${f.key}`}
              value={card.metadata[f.key] ?? ""}
              onChange={(e) =>
                mutate((c) => {
                  c.metadata[f.key] = e.target.value;
                }, `Edit metadata ${f.key}`)
              }
            />
          </label>
        ))}
      <label className="qi-field">
        Difficulty
        <select
          aria-label="Card difficulty"
          value={card.difficulty}
          onChange={(e) =>
            mutate((c) => {
              c.difficulty = e.target.value as DraftCard["difficulty"];
            }, "Edit difficulty")
          }
        >
          <option value="easy">Easy</option>
          <option value="medium">Medium</option>
          <option value="hard">Hard</option>
        </select>
      </label>
      {folder.template_key === "interview_questions" && (
        <fieldset className="qi-profile">
          <legend>Interview profile</legend>
          {["frequency", "difficulty", "level_min", "level_max"].map((key) => (
            <label key={key}>
              {key}
              <input
                type="number"
                aria-label={`Profile ${key}`}
                min={1}
                max={key === "frequency" ? 10 : 5}
                value={card.profile?.[key as "frequency"] ?? ""}
                placeholder="Default"
                onChange={(e) =>
                  mutate((c) => {
                    c.profile = {
                      ...c.profile,
                      [key]: e.target.value
                        ? Number(e.target.value)
                        : undefined,
                    };
                  }, `Edit profile ${key}`)
                }
              />
            </label>
          ))}
          <label>
            Concepts (one per line)
            <textarea
              aria-label="Profile concepts"
              value={card.profile?.concepts?.join("\n") ?? ""}
              onChange={(e) =>
                mutate((c) => {
                  c.profile = {
                    ...c.profile,
                    concepts: e.target.value.split("\n").filter(Boolean),
                  };
                }, "Edit concepts")
              }
            />
          </label>
          <p className="muted">
            Without an answer this question is saved as a draft. Answered
            questions receive a ready profile; default concepts come from the
            topic.
          </p>
        </fieldset>
      )}
      {folder.config.schema.fields.some(
        (f) => f.key === "example" && f.active,
      ) && (
        <section aria-label="Examples">
          <h4>Examples · {card.examples.length}</h4>
          {card.examples.map((ex, index) => (
            <div className="qi-example" key={ex.id}>
              <textarea
                aria-label={`Example ${index + 1}`}
                rows={2}
                value={ex.text}
                onChange={(e) =>
                  mutate(
                    (c) => {
                      c.examples.find((x) => x.id === ex.id)!.text =
                        e.target.value;
                    },
                    `Edit example ${index + 1}`,
                  )
                }
              />
              <div className="button-row">
                <button
                  className="button small"
                  onClick={() => source(ex.range)}
                >
                  Go to source
                </button>
                <button
                  className="button small"
                  onClick={() =>
                    mutate((c) => {
                      c.examples = c.examples.filter((x) => x.id !== ex.id);
                    }, `Exclude example ${ex.id}`)
                  }
                >
                  Exclude example
                </button>
                <button
                  className="button small"
                  disabled={index === 0}
                  onClick={() =>
                    mutate((c) => {
                      [c.examples[index - 1], c.examples[index]] = [
                        c.examples[index],
                        c.examples[index - 1],
                      ];
                    }, "Reorder examples")
                  }
                >
                  Move up
                </button>
                <button
                  className="button small"
                  disabled={!target || target === card.id}
                  onClick={() =>
                    change((d) => moveExample(d, card.id, ex.id, target))
                  }
                >
                  Move to target
                </button>
                <button
                  className="button small"
                  disabled={!target || target === card.id}
                  onClick={() =>
                    change((d) => moveExample(d, card.id, ex.id, target, true))
                  }
                >
                  Copy to target
                </button>
              </div>
            </div>
          ))}
          <button
            className="button small"
            onClick={() =>
              mutate((c) => {
                c.examples.push({
                  id: createClientUUID(),
                  text: "",
                  range: card.range,
                });
              }, "Add example")
            }
          >
            Add example
          </button>
        </section>
      )}
      <label className="qi-field">
        Duplicate decision
        <select
          aria-label="Duplicate decision"
          value={card.duplicatePolicy ?? "skip"}
          onChange={(e) =>
            mutate((c) => {
              c.duplicatePolicy = e.target.value as
                "skip" | "allow" | "conflict";
            }, "Set duplicate decision")
          }
        >
          <option value="skip">Skip exact duplicate (default)</option>
          <option value="conflict">Stop if duplicate exists</option>
          <option value="allow">Create even if duplicate exists</option>
        </select>
      </label>
      {card.duplicateOf && (
        <p>
          Batch duplicate of {card.duplicateOf}. Translations will not be merged
          automatically.
        </p>
      )}
      <label className="qi-field">
        Target card for merge or example reassignment
        <select
          aria-label="Target card"
          value={target}
          onChange={(e) => setTarget(e.target.value)}
        >
          <option value="">Choose card</option>
          {draft.cards
            .filter((c) => c.id !== card.id && !c.excluded)
            .map((c) => (
              <option key={c.id} value={c.id}>
                {c.values.foreign ?? c.values.question ?? c.id}
              </option>
            ))}
        </select>
      </label>
      <div className="button-row">
        <button
          className="button small"
          disabled={!target}
          onClick={() => {
            change((d) => merge(d, target, card.id));
            select(target);
          }}
        >
          Merge into target
        </button>
        <button
          className="button small"
          onClick={() => {
            let id = "";
            change((d) => {
              id = split(d, card.id)?.id ?? "";
            });
            if (id) select(id);
          }}
        >
          Split card
        </button>
      </div>
      <p className="muted">
        Split adds a blank card beside this one. Edit both cards to choose which
        text belongs to each. The source and automatic interpretation stay
        available.
      </p>
      {issues.map((i) => (
        <Fragment
          key={i.id}
          issue={i}
          draft={draft}
          folder={folder}
          change={change}
          source={source}
        />
      ))}
      <details>
        <summary>Automatic interpretation</summary>
        <pre>
          {JSON.stringify(
            draft.automatic.cards.find((c) => c.id === card.id) ??
              "Manually added card",
            null,
            2,
          )}
        </pre>
      </details>
    </section>
  );
}
