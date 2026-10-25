import { createClientUUID } from "../../../uuid";
import { useEffect, useState } from "react";
import type {
  DraftCard,
  ImportDraft,
  ImportFolder,
  Issue,
  Range,
  SchemaField,
} from "../model/types";
import { diagnosticsFor, type Diagnostic } from "../model/diagnostics";
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
  diagnostics?: Diagnostic[];
  change: (edit: (d: ImportDraft) => void, group?: string) => void;
  endGroup?: () => void;
  select: (id: string) => void;
  source: (range: Range) => void;
};
const title = (card?: DraftCard) =>
  card?.values.foreign || card?.values.question || "Untitled card";
const fieldTitle = (field: SchemaField) => {
  if (field.key === "foreign") return "Word or phrase";
  if (field.key === "native") return "Translation";
  return field.label ?? field.key.replaceAll("_", " ");
};

export function Fragment({
  issue,
  draft,
  folder,
  change,
  source,
}: Omit<Props, "card" | "select"> & { issue: Issue }) {
  const availableCards = draft.cards.filter((card) => !card.excluded);
  const [chosenTarget, setTarget] = useState(
    issue.cardId ?? availableCards[0]?.id ?? "",
  );
  const target = availableCards.some((card) => card.id === chosenTarget)
    ? chosenTarget
    : (availableCards[0]?.id ?? "");
  const fields = folder.config.schema.fields.filter(
    (field) => field.active && field.key !== "transcription",
  );
  const [field, setField] = useState(
    folder.template_key === "english_words" ? "example" : "note",
  );
  if (issue.resolved) return null;
  const unsupported = ["unsupported_fragment", "unsupported_field"].includes(
    issue.code,
  );
  const pronunciation =
    issue.code === "unsupported_fragment" ||
    /pronunciation|transcription/i.test(
      `${issue.path ?? ""} ${issue.fragment}`,
    );
  const heading = unsupported
    ? "Unrecognized fragment"
    : issue.code === "boundary_alternative"
      ? "Check where this example belongs"
      : issue.code === "weak_example"
        ? "Check this example"
        : "Review this fragment";
  return (
    <article
      className="qi-issue"
      data-testid="import-fragment"
      data-diagnostic={issue.id}
      tabIndex={-1}
    >
      <strong>{heading}</strong>
      <p>
        {unsupported
          ? pronunciation
            ? "This may be a pronunciation hint. Pronunciation is not imported."
            : "This field is not part of the import format."
          : issue.message}
      </p>
      {issue.fragment && (
        <pre className="qi-fragment-text">{issue.fragment}</pre>
      )}
      <p className="muted">
        {unsupported
          ? "Exclude this fragment, assign it to a supported field, or edit the source."
          : "Choose an interpretation below, or edit the source."}
      </p>
      {issue.code === "boundary_alternative" &&
        issue.alternatives
          ?.filter(
            (alternative) =>
              alternative.kind === "example" && alternative.cardId,
          )
          .map((alternative) => (
            <button
              className="button small"
              key={alternative.cardId}
              onClick={() =>
                change((d) => chooseBoundary(d, issue.id, alternative.cardId!))
              }
            >
              Use as example of{" "}
              {title(
                draft.cards.find((card) => card.id === alternative.cardId),
              )}
            </button>
          ))}
      <div className="button-row">
        <button className="button small" onClick={() => source(issue.range)}>
          Go to source
        </button>
        <button
          className="button small"
          onClick={() =>
            change((d) => {
              const found = d.issues.find((i) => i.id === issue.id);
              if (found) found.resolved = "excluded";
              record(d, "Exclude fragment");
            })
          }
        >
          Exclude fragment
        </button>
        {["boundary_alternative", "weak_example", "duplicate_word"].includes(
          issue.code,
        ) && (
          <button
            className="button small"
            onClick={() =>
              change((d) => {
                const found = d.issues.find((i) => i.id === issue.id);
                if (found) found.resolved = "accepted";
                record(d, "Keep interpretation");
              })
            }
          >
            Keep interpretation
          </button>
        )}
      </div>
      {availableCards.length > 0 && (
        <details className="qi-assignment-details">
          <summary>Assign to a card</summary>
          <div className="qi-assignment">
            <label>
              Card
              <select
                aria-label={`Assign ${issue.id} to card`}
                value={target}
                onChange={(e) => setTarget(e.target.value)}
              >
                {availableCards.map((card) => (
                  <option key={card.id} value={card.id}>
                    {title(card)}
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
                {fields.map((f) => (
                  <option key={f.key} value={f.key}>
                    {fieldTitle(f)}
                  </option>
                ))}
              </select>
            </label>
            <button
              className="button small"
              disabled={!target || !fields.some((f) => f.key === field)}
              onClick={() => change((d) => assign(d, issue.id, target, field))}
            >
              Assign fragment
            </button>
          </div>
        </details>
      )}
      <details className="qi-technical">
        <summary>Details</summary>
        <p>
          <code>{issue.code}</code> · {issue.path ?? `Line ${issue.range.line}`}
        </p>
        {!!issue.alternatives?.length && (
          <ul>
            {issue.alternatives.map((alternative, index) => (
              <li key={index}>
                {title(
                  draft.cards.find((card) => card.id === alternative.cardId),
                )}
                : {alternative.kind} · evidence {alternative.score} (
                {alternative.reasons.join(", ")})
              </li>
            ))}
          </ul>
        )}
      </details>
    </article>
  );
}

function DiagnosticMessages({
  diagnostics,
  id,
}: {
  diagnostics: Diagnostic[];
  id?: string;
}) {
  if (!diagnostics.length) return null;
  return (
    <div className="qi-field-diagnostics" id={id}>
      {diagnostics.map((diagnostic) => (
        <div
          className={`qi-diagnostic qi-diagnostic-${diagnostic.severity}`}
          key={diagnostic.id}
          data-diagnostic={diagnostic.id}
          tabIndex={-1}
          role={diagnostic.severity === "info" ? "note" : "alert"}
        >
          <strong>{diagnostic.message}</strong>
          <p>
            {diagnostic.why} {diagnostic.action}
          </p>
          {diagnostic.sources.includes("server") && (
            <details className="qi-technical">
              <summary>Details</summary>
              <p>
                <code>{diagnostic.code}</code>
                {diagnostic.path ? ` · ${diagnostic.path}` : ""}
              </p>
            </details>
          )}
        </div>
      ))}
    </div>
  );
}

export function CardEditor({
  draft,
  folder,
  card,
  diagnostics,
  change,
  endGroup,
  select,
  source,
}: Props) {
  const [target, setTarget] = useState("");
  const conceptsValue = card.profile?.concepts?.join("\n") ?? "";
  const [conceptInput, setConceptInput] = useState(conceptsValue);
  useEffect(() => {
    setConceptInput((current) =>
      current
        .split("\n")
        .filter((line) => line.trim())
        .join("\n") === conceptsValue
        ? current
        : conceptsValue,
    );
  }, [conceptsValue]);
  useEffect(() => setTarget(""), [card.id]);
  const currentDiagnostics = (
    diagnostics ?? diagnosticsFor(draft, folder)
  ).filter((diagnostic) => diagnostic.cardId === card.id);
  const visibleIssues = new Set(
    currentDiagnostics.flatMap((diagnostic) => diagnostic.issueIds),
  );
  const issues = draft.issues.filter(
    (issue) =>
      issue.cardId === card.id &&
      !issue.resolved &&
      visibleIssues.has(issue.id),
  );
  const mutate = (fn: (c: DraftCard) => void, action: string, group?: string) =>
    change(
      (d) => {
        const found = d.cards.find((c) => c.id === card.id);
        if (!found) return;
        fn(found);
        if (group) d.updatedAt = Date.now();
        else record(d, action);
      },
      group ? `${card.id}:${group}` : undefined,
    );
  const fields = folder.config.schema.fields.filter(
    (field) =>
      field.active && !["transcription", "example"].includes(field.key),
  );
  const basicKeys =
    folder.template_key === "english_words"
      ? ["foreign", "native"]
      : ["question", "short_answer", "answer"];
  const basic = fields.filter((field) => basicKeys.includes(field.key));
  const advanced = fields.filter((field) => !basicKeys.includes(field.key));
  const atField = (field: string) =>
    currentDiagnostics.filter((diagnostic) => diagnostic.field === field);
  const errorId = (field: string) =>
    `qi-${card.id}-${field.replaceAll(".", "-")}-messages`;
  const accessibility = (field: string) => {
    const messages = atField(field);
    return {
      "data-field": field,
      "aria-invalid":
        messages.some((diagnostic) => diagnostic.severity === "error") ||
        undefined,
      "aria-describedby": messages.length ? errorId(field) : undefined,
    };
  };
  const renderField = (field: SchemaField) => {
    const path = `values.${field.key}`;
    const id = `qi-${card.id}-${field.key}`;
    return (
      <div className="qi-field" key={field.key}>
        <label htmlFor={id}>
          {fieldTitle(field)}
          {field.required ? " *" : ""}
        </label>
        <textarea
          id={id}
          aria-label={field.key}
          {...accessibility(path)}
          value={card.values[field.key] ?? ""}
          rows={basicKeys.includes(field.key) ? 2 : 3}
          onChange={(e) =>
            mutate(
              (c) => {
                c.values[field.key] = e.target.value;
              },
              `Edit ${field.key}`,
              path,
            )
          }
          onBlur={endGroup}
        />
        <DiagnosticMessages diagnostics={atField(path)} id={errorId(path)} />
        {card.ranges[field.key] && (
          <button
            className="qi-source-link"
            onClick={() => source(card.ranges[field.key])}
          >
            Source line {card.ranges[field.key].line}
          </button>
        )}
      </div>
    );
  };
  const examplesEnabled = folder.config.schema.fields.some(
    (field) => field.key === "example" && field.active,
  );
  const hasTarget =
    !!target &&
    draft.cards.some(
      (candidate) =>
        candidate.id === target &&
        !candidate.excluded &&
        candidate.id !== card.id,
    );
  return (
    <section className="qi-editor" aria-label="Card editor">
      <div className="qi-editor-heading">
        <h3>{title(card)}</h3>
        <div className="button-row">
          <button className="button small" onClick={() => source(card.range)}>
            Go to source
          </button>
          <button
            className="button small"
            onClick={() =>
              mutate(
                (c) => {
                  c.excluded = !c.excluded;
                },
                card.excluded ? "Include card" : "Exclude card",
              )
            }
          >
            {card.excluded ? "Include card" : "Exclude card"}
          </button>
        </div>
      </div>
      {card.excluded && (
        <p className="muted">This card is excluded from the import.</p>
      )}
      <DiagnosticMessages
        diagnostics={currentDiagnostics.filter(
          (diagnostic) => !diagnostic.field && !diagnostic.issueIds.length,
        )}
      />
      {basic.map(renderField)}
      {examplesEnabled && (
        <section className="qi-examples" aria-label="Examples">
          {!!card.examples.length && <h4>Examples</h4>}
          {card.examples.map((example, index) => (
            <div className="qi-example" key={example.id}>
              <label className="sr-only" htmlFor={`qi-example-${example.id}`}>
                Example {index + 1}
              </label>
              <textarea
                id={`qi-example-${example.id}`}
                aria-label={`Example ${index + 1}`}
                {...accessibility("values.example")}
                rows={2}
                value={example.text}
                onChange={(e) =>
                  mutate(
                    (c) => {
                      c.examples.find(
                        (candidate) => candidate.id === example.id,
                      )!.text = e.target.value;
                    },
                    `Edit example ${index + 1}`,
                    `example:${example.id}`,
                  )
                }
                onBlur={endGroup}
              />
              <div className="button-row">
                <button
                  className="qi-source-link"
                  onClick={() => source(example.range)}
                >
                  Source line {example.range.line}
                </button>
                <button
                  className="qi-source-link"
                  onClick={() =>
                    mutate((c) => {
                      c.examples = c.examples.filter(
                        (candidate) => candidate.id !== example.id,
                      );
                    }, "Exclude example")
                  }
                >
                  Exclude example
                </button>
              </div>
            </div>
          ))}
          <DiagnosticMessages
            diagnostics={atField("values.example")}
            id={errorId("values.example")}
          />
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
      {issues.map((issue) => (
        <Fragment
          key={issue.id}
          issue={issue}
          draft={draft}
          folder={folder}
          change={change}
          source={source}
        />
      ))}
      <details className="qi-advanced">
        <summary>Advanced</summary>
        {advanced.map(renderField)}
        {folder.config.metadata_schema.fields
          .filter((field) => field.active)
          .map((field) => {
            const path = `metadata.${field.key}`;
            const id = `qi-${card.id}-metadata-${field.key}`;
            return (
              <div className="qi-field" key={field.key}>
                <label htmlFor={id}>
                  {fieldTitle(field)}
                  {field.required ? " *" : ""}
                </label>
                <input
                  id={id}
                  aria-label={`Metadata ${field.key}`}
                  {...accessibility(path)}
                  value={card.metadata[field.key] ?? ""}
                  onChange={(e) =>
                    mutate(
                      (c) => {
                        c.metadata[field.key] = e.target.value;
                      },
                      `Edit metadata ${field.key}`,
                      path,
                    )
                  }
                  onBlur={endGroup}
                />
                <DiagnosticMessages
                  diagnostics={atField(path)}
                  id={errorId(path)}
                />
              </div>
            );
          })}
        <label className="qi-field">
          Difficulty
          <select
            aria-label="Card difficulty"
            {...accessibility("difficulty")}
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
        <DiagnosticMessages
          diagnostics={atField("difficulty")}
          id={errorId("difficulty")}
        />
        {folder.template_key === "interview_questions" && (
          <fieldset className="qi-profile">
            <legend>Interview profile</legend>
            {(
              [
                ["frequency", "Frequency"],
                ["difficulty", "Difficulty"],
                ["level_min", "Minimum level"],
                ["level_max", "Maximum level"],
              ] as const
            ).map(([key, label]) => (
              <div className="qi-field" key={key}>
                <label htmlFor={`qi-profile-${card.id}-${key}`}>{label}</label>
                <input
                  id={`qi-profile-${card.id}-${key}`}
                  type="number"
                  aria-label={`Profile ${key}`}
                  {...accessibility(`profile.${key}`)}
                  min={1}
                  max={key === "frequency" ? 10 : 5}
                  value={card.profile?.[key] ?? ""}
                  placeholder="Default"
                  onChange={(e) =>
                    mutate(
                      (c) => {
                        c.profile = {
                          ...c.profile,
                          [key]: e.target.value
                            ? Number(e.target.value)
                            : undefined,
                        };
                      },
                      `Edit profile ${key}`,
                      `profile.${key}`,
                    )
                  }
                  onBlur={endGroup}
                />
                <DiagnosticMessages
                  diagnostics={atField(`profile.${key}`)}
                  id={errorId(`profile.${key}`)}
                />
              </div>
            ))}
            <div className="qi-field">
              <label htmlFor={`qi-concepts-${card.id}`}>
                Concepts (one per line)
              </label>
              <textarea
                id={`qi-concepts-${card.id}`}
                aria-label="Profile concepts"
                {...accessibility("profile.concepts")}
                value={conceptInput}
                onChange={(e) => {
                  setConceptInput(e.target.value);
                  mutate(
                    (c) => {
                      c.profile = {
                        ...c.profile,
                        concepts: e.target.value
                          .split("\n")
                          .filter((line) => line.trim()),
                      };
                    },
                    "Edit concepts",
                    "profile.concepts",
                  );
                }}
                onBlur={endGroup}
              />
              <DiagnosticMessages
                diagnostics={atField("profile.concepts")}
                id={errorId("profile.concepts")}
              />
            </div>
            <p className="muted">
              Questions without an answer are saved as drafts. Answered
              questions receive a ready profile; default concepts come from the
              topic.
            </p>
          </fieldset>
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
          <p className="muted">
            This repeats{" "}
            {title(
              draft.cards.find(
                (candidate) => candidate.id === card.duplicateOf,
              ),
            )}
            . Translations stay separate unless you merge the cards.
          </p>
        )}
        <label className="qi-field">
          Target card for merge or example reassignment
          <select
            aria-label="Target card"
            value={hasTarget ? target : ""}
            onChange={(e) => setTarget(e.target.value)}
          >
            <option value="">Choose card</option>
            {draft.cards
              .filter(
                (candidate) => candidate.id !== card.id && !candidate.excluded,
              )
              .map((candidate) => (
                <option key={candidate.id} value={candidate.id}>
                  {title(candidate)}
                </option>
              ))}
          </select>
        </label>
        {!!card.examples.length && (
          <section aria-label="Organize examples">
            <h4>Organize examples</h4>
            {card.examples.map((example, index) => (
              <div className="qi-example" key={example.id}>
                <p>
                  {index + 1}. {example.text || "Empty example"}
                </p>
                <div className="button-row">
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
                    disabled={!hasTarget}
                    onClick={() =>
                      change((d) => moveExample(d, card.id, example.id, target))
                    }
                  >
                    Move to target
                  </button>
                  <button
                    className="button small"
                    disabled={!hasTarget}
                    onClick={() =>
                      change((d) =>
                        moveExample(d, card.id, example.id, target, true),
                      )
                    }
                  >
                    Copy to target
                  </button>
                </div>
              </div>
            ))}
          </section>
        )}
        <div className="button-row">
          <button
            className="button small"
            disabled={!hasTarget}
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
          Split adds a blank card beside this one. Edit both cards to choose
          which text belongs to each.
        </p>
        <details className="qi-technical">
          <summary>Automatic interpretation</summary>
          <pre>
            {JSON.stringify(
              draft.automatic.cards.find(
                (candidate) => candidate.id === card.id,
              ) ?? "Manually added card",
              null,
              2,
            )}
          </pre>
        </details>
      </details>
    </section>
  );
}
