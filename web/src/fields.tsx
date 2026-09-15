import type { Field, FolderConfig } from "./types";
import { Plus } from "lucide-react";

export function initialConfig(kind: string): FolderConfig {
  const keys =
    kind === "interview_questions"
      ? [
          "question",
          "answer",
          "short_answer",
          "explanation",
          "code_example",
          "common_mistakes",
          "note",
          "sources",
        ]
      : kind === "formulas"
        ? [
            "name",
            "formula",
            "explanation",
            "variables",
            "units",
            "example",
            "conditions",
            "note",
            "sources",
          ]
        : [
            "foreign",
            "native",
            "transcription",
            "definition",
            "example",
            "example_translation",
            "note",
            "sources",
          ];
  const field = (key: string, i: number): Field => ({
    key,
    label:
      key === "short_answer"
        ? "Short Answer"
        : key === "answer"
          ? "Detailed Answer"
          : key === "sources" && kind === "interview_questions" ? "Source"
          : key
              .split("_")
              .map((s) => s[0].toUpperCase() + s.slice(1))
              .join(" "),
    active: true,
    required: kind === "interview_questions" ? i === 0 : i < 2,
  });
  return {
    schema: { fields: keys.map(field) },
    metadata_schema: {
      fields: (kind === "interview_questions"
        ? ["topic", "company", "level", "category"]
        : ["topic"]
      ).map((key) => ({ ...field(key, 2), active: key !== "category" })),
    },
    card: {
      question_fields: [keys[0]],
      answer_fields:
        kind === "interview_questions"
          ? ["short_answer", "answer", ...keys.slice(3)]
          : keys.slice(1),
    },
  };
}

export function CardFields({
  value,
  onChange,
}: {
  value: FolderConfig;
  onChange: (v: FolderConfig) => void;
}) {
  function update(
    group: "schema" | "metadata_schema",
    key: string,
    patch: Partial<Field>,
  ) {
    onChange({
      ...value,
      [group]: {
        fields: value[group].fields.map((f) =>
          f.key === key ? { ...f, ...patch } : f,
        ),
      },
    });
  }
  function role(key: string, side: string) {
    onChange({
      ...value,
      card: {
        question_fields: [
          ...value.card.question_fields.filter((k) => k !== key),
          ...(side === "question" ? [key] : []),
        ],
        answer_fields: [
          ...value.card.answer_fields.filter((k) => k !== key),
          ...(side === "answer" ? [key] : []),
        ],
      },
    });
  }
  function add(group: "schema" | "metadata_schema") {
    const key =
      "custom_" + crypto.randomUUID().replaceAll("-", "").slice(0, 12);
    onChange({
      ...value,
      [group]: {
        fields: [
          ...value[group].fields,
          { key, label: "New field", active: true, required: false },
        ],
      },
    });
  }
  return (
    <section className="folder-fields">
      <h3>Card fields</h3>
      <p className="muted small-text">
        Choose exactly what appears when you add a card. Disabled fields keep
        their existing content. Assign at least one question and one answer
        field.
      </p>
      {(["schema", "metadata_schema"] as const).map((group) => (
        <div key={group}>
          <h4>
            {group === "schema"
              ? "Content & presentation"
              : "Organization fields"}
          </h4>
          <div className="field-settings-list">
            {value[group].fields.map((f) => (
              <div
                className={`field-settings-row ${f.active ? "" : "inactive"}`}
                key={f.key}
              >
                <label className="field-toggle">
                  <input
                    type="checkbox"
                    aria-label={`Enable ${f.label}`}
                    checked={f.active}
                    onChange={(e) =>
                      update(group, f.key, { active: e.target.checked })
                    }
                  />
                  <span className="sr-only">Enable {f.label}</span>
                </label>
                <input
                  aria-label={`Label for ${f.key}`}
                  required
                  maxLength={100}
                  value={f.label}
                  onChange={(e) =>
                    update(group, f.key, { label: e.target.value })
                  }
                />
                {group === "schema" && (
                  <select
                    aria-label={`Show ${f.label} on`}
                    disabled={!f.active}
                    value={
                      value.card.question_fields.includes(f.key)
                        ? "question"
                        : value.card.answer_fields.includes(f.key)
                          ? "answer"
                          : "editor"
                    }
                    onChange={(e) => role(f.key, e.target.value)}
                  >
                    <option value="question">Question</option>
                    <option value="answer">Answer</option>
                    <option value="editor">Editor only</option>
                  </select>
                )}
                <label className="field-required">
                  <input
                    type="checkbox"
                    disabled={!f.active}
                    checked={f.required}
                    onChange={(e) =>
                      update(group, f.key, { required: e.target.checked })
                    }
                  />
                  Required
                </label>
              </div>
            ))}
          </div>
          <button
            type="button"
            className="text-button"
            onClick={() => add(group)}
          >
            <Plus size={16} />
            {group === "schema"
              ? "Add content field"
              : "Add organization field"}
          </button>
        </div>
      ))}
    </section>
  );
}
