import { useEffect, useState, type FormEvent } from "react";
import {
  BookOpen,
  Check,
  Download,
  GitBranch,
  Plus,
  Trash2,
} from "lucide-react";
import { api, errorText, post } from "./api";
import { useLibrary } from "./App";
import { ErrorBox, Modal, Spinner } from "./ui";
import { interviewProfiles, interviewLevels, type InterviewProfile } from "./interview-types";
import "./interview.css";

type Alias = {
  alias: string;
  language: string;
  weight: number;
  whole_word: boolean;
  constraints?: { requires_any?: string[]; requires_domain?: string };
};
type Concept = {
  slug: string;
  display_name: string;
  domain: string;
  topic: string;
  aliases: Alias[];
  version?: number;
};
type SeedInfo = {
  version: string;
  question_count: number;
  domains: { slug: string; name: string; question_count: number }[];
};

export function defaultInterviewProfile(
  answerExists = false,
): InterviewProfile {
  return {
    frequency: 5,
    frequency_confidence: 0.5,
    interview_difficulty: 2,
    specificity: 2,
    root_weight: 5,
    followup_weight: 5,
    level_min: 1,
    level_max: 5,
    interview_profiles: [],
    status: answerExists ? "ready" : "draft",
    profile_version: 0,
    domain: "",
    concepts: [],
  };
}

export function InterviewProfileFields({
  value,
  onChange,
  disabled = false,
}: {
  value: InterviewProfile;
  onChange: (profile: InterviewProfile) => void;
  disabled?: boolean;
}) {
  const [concepts, setConcepts] = useState<Concept[]>([]);
  useEffect(() => {
    let alive = true;
    void api<{ concepts: Concept[] }>("/interview/concepts")
      .then((result) => {
        if (alive) setConcepts(result.concepts || []);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  const roles = [
    ["primary", "Primary concept"],
    ["tested", "Tested concepts"],
    ["answer", "Answer concepts — route after Correct"],
    ["hook", "Expected hooks"],
    ["prerequisite", "Prerequisites"],
    ["wrong_fallback", "Fallback concepts — route after Wrong"],
  ] as const;
  function replaceRole(
    role: InterviewProfile["concepts"][number]["role"],
    text: string,
  ) {
    const slugs = [
      ...new Set(
        text
          .split(",")
          .map((v) => v.trim())
          .filter(Boolean),
      ),
    ];
    onChange({
      ...value,
      concepts: [
        ...value.concepts.filter((c) => c.role !== role),
        ...slugs.map((slug, ordinal) => ({
          slug,
          role,
          ordinal,
          weight:
            value.concepts.find((c) => c.slug === slug && c.role === role)
              ?.weight || 1,
        })),
      ],
    });
  }
  return (
    <fieldset className="interview-profile-fields" disabled={disabled}>
      <legend>
        <GitBranch size={18} /> Interview Graph
      </legend>
      <p className="interview-field-help">
        Drafts stay in your library. Mark a question ready when its answer is
        complete.
      </p>
      {value.seed_key && <span className="interview-seed-key">{value.seed_key}</span>}
      <label>
        Status
        <select
          value={value.status}
          onChange={(e) =>
            onChange({
              ...value,
              status: e.target.value as InterviewProfile["status"],
            })
          }
        >
          <option value="draft">Draft</option>
          <option value="ready">Ready to train</option>
          <option value="archived">Archived</option>
        </select>
      </label>
      <div className="interview-profile-numbers">
        {(
          [
            ["frequency", "Frequency", 1, 10],
            ["interview_difficulty", "Interview difficulty", 1, 5],
            ["specificity", "Specificity", 1, 5],
            ["root_weight", "Starting question weight", 0, 10],
            ["followup_weight", "Follow-up weight", 0, 10],
          ] as const
        ).map(([key, label, min, max]) => (
          <label key={key}>
            {label}
            <span className="interview-range">
              <input
                type="range"
                min={min}
                max={max}
                value={value[key]}
                onChange={(e) => onChange({ ...value, [key]: +e.target.value })}
              />
              <output>{value[key]}</output>
            </span>
          </label>
        ))}
      </div>
      <div className="interview-profile-numbers">
        {(["level_min", "level_max"] as const).map(key => <label key={key}>
          {key === "level_min" ? "Minimum level" : "Maximum level"}
          <select value={value[key]} onChange={e => onChange({...value, [key]: +e.target.value})}>
            {interviewLevels.map((label, i) => <option key={label} value={i+1}>{label}</option>)}
          </select>
        </label>)}
      </div>
      <div className="interview-profile-memberships">
        <span>Interview profiles</span>
        {interviewProfiles.filter(([slug]) => slug !== "all").map(([slug, label]) => <label key={slug}>
          <input type="checkbox" checked={(value.interview_profiles || []).includes(slug)} onChange={e => onChange({...value, interview_profiles: e.target.checked ? [...(value.interview_profiles || []), slug] : (value.interview_profiles || []).filter(v => v !== slug)})} />
          {label}
        </label>)}
      </div>
      <label>
        Frequency confidence
        <input
          type="number"
          min={0}
          max={1}
          step={0.05}
          value={value.frequency_confidence}
          onChange={(e) =>
            onChange({ ...value, frequency_confidence: +e.target.value })
          }
        />
      </label>
      <label>
        Domain
        <input
          value={value.domain || ""}
          placeholder="e.g. go"
          maxLength={96}
          onChange={(e) => onChange({ ...value, domain: e.target.value })}
        />
      </label>
      <label>Subtopic<input value={value.subtopic || ""} maxLength={200} onChange={e => onChange({...value, subtopic: e.target.value})} /></label>
      {roles.map(([role, label]) => (
        <ConceptInput
          key={role}
          label={label}
          value={value.concepts
            .filter((c) => c.role === role)
            .map((c) => c.slug)
            .join(", ")}
          onChange={(text) => replaceRole(role, text)}
          listID={
            role === "primary" ? "interview-concept-suggestions" : undefined
          }
        />
      ))}
      <datalist id="interview-concept-suggestions">
        {concepts.map((concept) => (
          <option key={concept.slug} value={concept.slug}>
            {concept.display_name}
          </option>
        ))}
      </datalist>
      <p className="interview-field-help">
        Separate concept slugs with commas. Manage their names and aliases in
        Mock interview → Concept dictionary.
      </p>
    </fieldset>
  );
}

function ConceptInput({
  value,
  onChange,
  label,
  listID,
}: {
  value: string;
  onChange: (value: string) => void;
  label: string;
  listID?: string;
}) {
  const [draft, setDraft] = useState(value);
  const [editing, setEditing] = useState(false);
  useEffect(() => {
    if (!editing) setDraft(value);
  }, [value, editing]);
  return (
    <label>
      {label}
      <input
        value={draft}
        list={listID}
        placeholder="e.g. goroutine, os_thread"
        onFocus={() => setEditing(true)}
        onChange={(e) => {
          setDraft(e.target.value);
          onChange(e.target.value);
        }}
        onBlur={() => setEditing(false)}
        maxLength={2000}
      />
    </label>
  );
}

export function InterviewBankTools() {
  const [open, setOpen] = useState<"import" | "concepts" | null>(null);
  return (
    <>
      <div className="interview-bank-tools">
        <button className="button" onClick={() => setOpen("import")}>
          <Download size={17} /> Import question bank
        </button>
        <button className="button" onClick={() => setOpen("concepts")}>
          <BookOpen size={17} /> Concept dictionary
        </button>
      </div>
      {open === "import" && (
        <ImportQuestionBank onClose={() => setOpen(null)} />
      )}
      {open === "concepts" && (
        <ConceptDictionary onClose={() => setOpen(null)} />
      )}
    </>
  );
}

function ImportQuestionBank({ onClose }: { onClose: () => void }) {
  const { reload, notify } = useLibrary();
  const [seed, setSeed] = useState<SeedInfo | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    let alive = true;
    void api<SeedInfo>("/interview/seed")
      .then((data) => {
        if (alive) {
          setSeed(data);
          setSelected(data.domains.map((d) => d.slug));
        }
      })
      .catch((e) => {
        if (alive) setError(errorText(e));
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);
  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const result = await post<{
        created: number;
        updated: number;
        skipped: number;
      }>("/interview/seed/import", { domains: selected });
      await reload();
      notify(
        `${result.created} questions added, ${result.updated} updated, ${result.skipped} unchanged. Imported questions start as drafts.`,
      );
      onClose();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title="Your backend interview bank"
      subtitle="Choose subjects to add to your library."
      onClose={() => {
        if (!busy) onClose();
      }}
      wide
    >
      {loading ? (
        <Spinner label="Loading question bank…" />
      ) : (
        <form className="form-body" onSubmit={submit}>
          <p className="interview-field-help">
            {seed?.question_count || 0} questions across{" "}
            {seed?.domains.length || 0} subjects. They arrive as drafts, ready
            for you to write and refine the answers. Importing again does not
            create duplicates.
          </p>
          <div className="interview-seed-domains">
            {seed?.domains.map((domain) => (
              <label key={domain.slug}>
                <input
                  type="checkbox"
                  disabled={busy}
                  checked={selected.includes(domain.slug)}
                  onChange={() =>
                    setSelected((values) =>
                      values.includes(domain.slug)
                        ? values.filter((value) => value !== domain.slug)
                        : [...values, domain.slug],
                    )
                  }
                />
                <span>
                  {domain.name}
                  <small>{domain.question_count} questions</small>
                </span>
              </label>
            ))}
          </div>
          {error && <ErrorBox error={error} />}
          <div className="dialog-actions">
            <button
              type="button"
              className="button"
              disabled={busy}
              onClick={onClose}
            >
              Cancel
            </button>
            <button
              className="button primary"
              disabled={busy || !selected.length}
            >
              {busy ? "Importing…" : "Import selected subjects"}
              <Download size={16} />
            </button>
          </div>
        </form>
      )}
    </Modal>
  );
}

function ConceptDictionary({ onClose }: { onClose: () => void }) {
  const [concepts, setConcepts] = useState<Concept[]>([]);
  const [draft, setDraft] = useState<Concept | null>(null);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  useEffect(() => {
    let alive = true;
    void api<{ concepts: Concept[] }>("/interview/concepts")
      .then((data) => {
        if (alive) setConcepts(data.concepts || []);
      })
      .catch((e) => {
        if (alive) setError(errorText(e));
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);
  function update(value: Concept) {
    setDraft(value);
    setDirty(true);
    setSaved(false);
  }
  function close() {
    if (
      !busy &&
      (!dirty || window.confirm("Discard your unsaved concept changes?"))
    )
      onClose();
  }
  async function save(e: FormEvent) {
    e.preventDefault();
    if (!draft) return;
    setBusy(true);
    setError("");
    try {
      const value = await api<Concept>(
        `/interview/concepts/${encodeURIComponent(draft.slug)}`,
        {
          method: "PUT",
          body: {
            ...draft,
            ...(draft.version ? { expected_version: draft.version } : {}),
          },
        },
      );
      const updated = value?.slug ? value : draft;
      setConcepts((items) =>
        items.map((item) => (item.slug === updated.slug ? updated : item)),
      );
      setDraft(updated);
      setDirty(false);
      setSaved(true);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  function alias(index: number, patch: Partial<Alias>) {
    if (draft)
      update({
        ...draft,
        aliases: draft.aliases.map((value, i) =>
          i === index ? { ...value, ...patch } : value,
        ),
      });
  }
  return (
    <Modal
      title="Concept dictionary"
      subtitle="Names and aliases used to follow the ideas in your answers."
      onClose={close}
      wide
    >
      {loading ? (
        <Spinner label="Loading concepts…" />
      ) : (
        <form className="form-body" onSubmit={save}>
          <label>
            Find a concept
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search name, slug, or domain"
            />
          </label>
          <label>
            Concept
            <select
              value={draft?.slug || ""}
              onChange={(e) => {
                if (
                  !dirty ||
                  window.confirm("Discard your unsaved concept changes?")
                ) {
                  setDraft(
                    structuredClone(
                      concepts.find((c) => c.slug === e.target.value) || null,
                    ),
                  );
                  setDirty(false);
                  setSaved(false);
                }
              }}
            >
              <option value="">Choose a concept</option>
              {concepts
                .filter((c) =>
                  `${c.slug} ${c.display_name} ${c.domain}`
                    .toLowerCase()
                    .includes(search.toLowerCase()),
                )
                .map((concept) => (
                  <option value={concept.slug} key={concept.slug}>
                    {concept.display_name} · {concept.domain}
                  </option>
                ))}
            </select>
          </label>
          {!concepts.length && (
            <p className="interview-field-help">
              Import a question bank to add its concept dictionary.
            </p>
          )}
          {draft && (
            <>
              <label>
                Display name
                <input
                  required
                  maxLength={200}
                  value={draft.display_name}
                  onChange={(e) =>
                    update({ ...draft, display_name: e.target.value })
                  }
                />
              </label>
              <div className="form-columns">
                <label>
                  Domain
                  <input
                    required
                    value={draft.domain}
                    onChange={(e) =>
                      update({ ...draft, domain: e.target.value })
                    }
                  />
                </label>
                <label>
                  Topic
                  <input
                    value={draft.topic}
                    onChange={(e) =>
                      update({ ...draft, topic: e.target.value })
                    }
                  />
                </label>
              </div>
              <div className="interview-aliases">
                <h3>Aliases</h3>
                {draft.aliases.map((value, index) => (
                  <fieldset key={index} className="interview-alias">
                    <legend>Alias {index + 1}</legend>
                    <label>
                      Text
                      <input
                        required
                        value={value.alias}
                        onChange={(e) =>
                          alias(index, { alias: e.target.value })
                        }
                      />
                    </label>
                    <div className="form-columns">
                      <label>
                        Language
                        <select
                          value={value.language}
                          onChange={(e) =>
                            alias(index, { language: e.target.value })
                          }
                        >
                          <option value="any">Any</option>
                          <option value="en">English</option>
                          <option value="ru">Russian</option>
                        </select>
                      </label>
                      <label>
                        Weight
                        <input
                          type="number"
                          min={0.01}
                          max={2}
                          step={0.05}
                          value={value.weight}
                          onChange={(e) =>
                            alias(index, { weight: +e.target.value })
                          }
                        />
                      </label>
                    </div>
                    <label className="interview-checkbox">
                      <input
                        type="checkbox"
                        checked={value.whole_word}
                        onChange={(e) =>
                          alias(index, { whole_word: e.target.checked })
                        }
                      />{" "}
                      Match whole words
                    </label>
                    <label>
                      Required domain
                      <input
                        value={value.constraints?.requires_domain || ""}
                        onChange={(e) =>
                          alias(index, {
                            constraints: {
                              ...value.constraints,
                              requires_domain: e.target.value || undefined,
                            },
                          })
                        }
                      />
                    </label>
                    <label>
                      Context words
                      <ConceptContextInput
                        value={value.constraints?.requires_any || []}
                        onChange={(words) =>
                          alias(index, {
                            constraints: {
                              ...value.constraints,
                              requires_any: words,
                            },
                          })
                        }
                      />
                    </label>
                    <button
                      type="button"
                      className="text-button danger-text"
                      onClick={() =>
                        update({
                          ...draft,
                          aliases: draft.aliases.filter((_, i) => i !== index),
                        })
                      }
                    >
                      <Trash2 size={15} /> Remove alias
                    </button>
                  </fieldset>
                ))}
                <button
                  type="button"
                  className="button"
                  onClick={() =>
                    update({
                      ...draft,
                      aliases: [
                        ...draft.aliases,
                        {
                          alias: "",
                          language: "any",
                          weight: 1,
                          whole_word: true,
                          constraints: {},
                        },
                      ],
                    })
                  }
                >
                  <Plus size={16} />
                  Add alias
                </button>
              </div>
            </>
          )}
          {error && <ErrorBox error={error} />}
          {saved && (
            <p className="interview-saved" role="status">
              <Check size={16} />
              Concept saved
            </p>
          )}
          <div className="dialog-actions">
            <button
              type="button"
              className="button"
              disabled={busy}
              onClick={close}
            >
              Close
            </button>
            <button
              className="button primary"
              disabled={busy || !draft || !dirty}
            >
              {busy ? "Saving…" : "Save concept"}
              <Check size={16} />
            </button>
          </div>
        </form>
      )}
    </Modal>
  );
}

function ConceptContextInput({
  value,
  onChange,
}: {
  value: string[];
  onChange: (words: string[]) => void;
}) {
  const [text, setText] = useState(value.join(", "));
  useEffect(() => {
    setText(value.join(", "));
  }, [JSON.stringify(value)]);
  return (
    <input
      value={text}
      placeholder="e.g. offset, consumer"
      onChange={(e) => setText(e.target.value)}
      onBlur={() =>
        onChange(
          text
            .split(",")
            .map((word) => word.trim())
            .filter(Boolean),
        )
      }
    />
  );
}
