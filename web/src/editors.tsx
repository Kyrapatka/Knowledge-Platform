import { useEffect, useState, type FormEvent } from "react";
import { ArrowRight, Check, Edit3, Plus, Trash2 } from "lucide-react";
import { api, errorText, post } from "./api";
import { useLibrary, TypeIcon } from "./App";
import type { Exercise, Folder, Material, SessionView, Topic } from "./types";
import { algorithmNames, materialTitle, templateNames, topicOf } from "./types";
import { DateValue, ErrorBox, Markdown, Modal, Spinner } from "./ui";
import { CardFields, initialConfig } from "./fields";

export function FolderEditor({
  folder,
  onClose,
  onSaved,
  onDeleted,
}: {
  folder?: Folder;
  onClose: () => void;
  onSaved: () => void;
  onDeleted?: () => void;
}) {
  const [title, setTitle] = useState(folder?.title || "");
  const [description, setDescription] = useState(folder?.description || "");
  const [kind, setKind] = useState(folder?.template_key || "english_words");
  const [config, setConfig] = useState(() =>
    structuredClone(folder?.config || initialConfig("english_words")),
  );
  const [savedFolder, setSavedFolder] = useState(folder);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const dirty =
    title !== (folder?.title || "") ||
    description !== (folder?.description || "") ||
    kind !== (folder?.template_key || "english_words") ||
    JSON.stringify(config) !==
      JSON.stringify(folder?.config || initialConfig(kind));
  function close() {
    if (
      !busy &&
      (!dirty || window.confirm("Discard your unsaved folder changes?"))
    )
      onClose();
  }
  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const enabled = new Set(
        config.schema.fields.filter((f) => f.active).map((f) => f.key),
      );
      const nextConfig = {
        ...config,
        card: {
          question_fields: config.card.question_fields.filter((k) =>
            enabled.has(k),
          ),
          answer_fields: config.card.answer_fields.filter((k) =>
            enabled.has(k),
          ),
        },
      };
      if (
        !nextConfig.card.question_fields.length ||
        !nextConfig.card.answer_fields.length
      )
        throw new Error(
          "Choose at least one active question field and one active answer field.",
        );
      const updated = await api<Folder>(
        `/folders${savedFolder ? `/${savedFolder.id}` : ""}`,
        {
          method: savedFolder ? "PATCH" : "POST",
          body: {
            title: title.trim(),
            description: description.trim(),
            ...(!savedFolder ? { template_key: kind } : {}),
          },
        },
      );
      setSavedFolder(updated);
      await api(`/folders/${updated.id}/workshop`, {
        method: "PATCH",
        body: { expected_version: updated.config_version, config: nextConfig },
      });
      onSaved();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }
  async function remove() {
    if (!folder) return;
    setBusy(true);
    setError("");
    try {
      await api(`/folders/${folder.id}`, { method: "DELETE" });
      onDeleted?.();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title={folder ? "Edit folder" : "A new place to learn"}
      subtitle={
        folder
          ? "Keep your collection feeling like you."
          : "Choose what you’re collecting. We’ll take care of the structure."
      }
      onClose={close}
      wide
    >
      <form onSubmit={save} className="form-body">
        {!savedFolder && (
          <div className="template-options">
            {Object.entries(templateNames).map(([key, label]) => (
              <button
                key={key}
                type="button"
                className={`template-option ${key === kind ? "selected" : ""}`}
                onClick={() => {
                  setKind(key);
                  setConfig(initialConfig(key));
                }}
              >
                <TypeIcon kind={key} />
                <strong>{label}</strong>
                <span>
                  {key === "english_words"
                    ? "Words & vocabulary"
                    : key === "formulas"
                      ? "Practice & understanding"
                      : "Questions & answers"}
                </span>
              </button>
            ))}
          </div>
        )}
        <label>
          Folder name
          <input
            autoFocus
            required
            maxLength={200}
            placeholder="e.g. Everyday English"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
        </label>
        <label>
          Description <span className="optional">optional</span>
          <textarea
            rows={3}
            maxLength={2000}
            placeholder="What would you like to remember?"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
        <CardFields value={config} onChange={setConfig} />
        {error && <ErrorBox error={error} />}
        {deleting && (
          <div className="delete-confirm">
            <strong>Delete “{folder?.title}”?</strong>
            <p>
              This removes the folder and its {folder?.material_count} materials
              from your library and training. Past training history is retained.
            </p>
            <button
              type="button"
              className="button danger"
              disabled={busy}
              onClick={remove}
            >
              Delete folder
            </button>
            <button
              type="button"
              className="text-button"
              onClick={() => setDeleting(false)}
            >
              Keep folder
            </button>
          </div>
        )}
        <div className="dialog-actions">
          {folder && (
            <button
              type="button"
              className="icon-button danger-text"
              aria-label="Delete folder"
              onClick={() => setDeleting(true)}
            >
              <Trash2 size={18} />
            </button>
          )}
          <span className="flex-spacer" />
          <button type="button" className="button" onClick={close}>
            Cancel
          </button>
          <button className="button primary" disabled={busy || !title.trim()}>
            {busy ? "Saving…" : folder ? "Save changes" : "Create folder"}
            <ArrowRight size={16} />
          </button>
        </div>
      </form>
    </Modal>
  );
}
export function MaterialEditor({
  folder,
  material,
  topics,
  onClose,
  onSaved,
  inTraining = false,
}: {
  folder: Folder;
  material?: Material;
  topics: Topic[];
  onClose: () => void;
  onSaved: () => void;
  inTraining?: boolean;
}) {
  const [values, setValues] = useState<Record<string, string | null>>({
    ...material?.values,
  });
  const [metadata, setMetadata] = useState<Record<string, string | null>>({
    ...material?.metadata,
    topic: material ? topicOf(material) : "",
  });
  const [difficulty, setDifficulty] = useState(
    material?.difficulty || "medium",
  );
  const [preview, setPreview] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [dirty, setDirty] = useState(false);
  function close() {
    if (
      !busy &&
      (!dirty || window.confirm("Discard your unsaved material changes?"))
    )
      onClose();
  }
  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const activeValues = Object.fromEntries(
        folder.config.schema.fields
          .filter((f) => f.active)
          .map((f) => [f.key, values[f.key]?.trim() || null]),
      );
      const activeMetadata = Object.fromEntries(
        folder.config.metadata_schema.fields
          .filter((f) => f.active)
          .map((f) => [f.key, metadata[f.key]?.trim() || null]),
      );
      await api(
        `/folders/${folder.id}/materials${material ? `/${material.id}` : ""}`,
        {
          method: material ? "PATCH" : "POST",
          body: { values: activeValues, metadata: activeMetadata, difficulty },
        },
      );
      onSaved();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title={material ? "Edit material" : "Add a little knowledge"}
      subtitle={`${folder.title} · Markdown, code and math supported`}
      onClose={close}
      drawer
    >
      <form onSubmit={save} className="form-body">
        <div className="editor-tabs tabs">
          <button
            type="button"
            className={!preview ? "active" : ""}
            onClick={() => setPreview(false)}
          >
            Write
          </button>
          <button
            type="button"
            className={preview ? "active" : ""}
            onClick={() => setPreview(true)}
          >
            Preview
          </button>
        </div>
        {inTraining && (
          <p className="info-note">
            Changes will appear on the next presentation. Your current card
            stays the same.
          </p>
        )}
        {folder.config.schema.fields
          .filter((f) => f.active)
          .map((field) => (
            <label key={field.key}>
              {field.label}
              {!field.required && <span className="optional">optional</span>}
              {preview ? (
                <div className="field-preview">
                  {values[field.key] ? (
                    <Markdown value={values[field.key]!} field={field.key} />
                  ) : (
                    <span className="muted">Nothing here yet.</span>
                  )}
                </div>
              ) : (
                <textarea
                  required={field.required}
                  rows={field.required ? 3 : 2}
                  maxLength={64000}
                  value={values[field.key] || ""}
                  onChange={(e) => {
                    setValues((v) => ({ ...v, [field.key]: e.target.value }));
                    setDirty(true);
                  }}
                />
              )}
            </label>
          ))}
        <div className="form-divider" />
        <div className="form-columns">
          {folder.config.metadata_schema.fields.some(
            (f) => f.key === "topic" && f.active,
          ) && (
            <label>
              {folder.config.metadata_schema.fields.find(
                (f) => f.key === "topic",
              )?.label || "Topic"}
              <input
                required={
                  folder.config.metadata_schema.fields.find(
                    (f) => f.key === "topic",
                  )?.required
                }
                list="material-topics"
                placeholder="e.g. SQL"
                maxLength={200}
                value={metadata.topic || ""}
                onChange={(e) => {
                  setMetadata((m) => ({ ...m, topic: e.target.value }));
                  setDirty(true);
                }}
              />
              <datalist id="material-topics">
                {topics
                  .filter((t) => t.name)
                  .map((t) => (
                    <option key={t.name} value={t.name} />
                  ))}
              </datalist>
            </label>
          )}
          <label>
            Difficulty
            <select
              value={difficulty}
              onChange={(e) => {
                setDifficulty(e.target.value as Material["difficulty"]);
                setDirty(true);
              }}
            >
              <option value="easy">Easy</option>
              <option value="medium">Medium</option>
              <option value="hard">Hard</option>
            </select>
          </label>
        </div>
        {folder.config.metadata_schema.fields
          .filter((f) => f.active && f.key !== "topic")
          .map((field) => (
            <label key={field.key}>
              {field.label}
              <input
                required={field.required}
                value={metadata[field.key] || ""}
                onChange={(e) => {
                  setMetadata((m) => ({ ...m, [field.key]: e.target.value }));
                  setDirty(true);
                }}
              />
            </label>
          ))}
        {error && <ErrorBox error={error} />}
        <div className="dialog-actions">
          <button type="button" className="button" onClick={close}>
            Cancel
          </button>
          <button className="button primary" disabled={busy}>
            {busy ? "Saving…" : "Save material"}
            <Check size={16} />
          </button>
        </div>
      </form>
    </Modal>
  );
}
export function MaterialDetails({
  folder,
  material,
  planId,
  onClose,
  onEdit,
  onChanged,
}: {
  folder: Folder;
  material: Material;
  planId?: string;
  onClose: () => void;
  onEdit: () => void;
  onChanged: () => void;
}) {
  const { notify } = useLibrary();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [deleting, setDeleting] = useState(false);
  const p = material.progress;
  async function progressAction(action: string) {
    if (!planId || !p) return;
    setBusy(true);
    setError("");
    try {
      const session = await post<SessionView>(
        `/training/plans/${planId}/sessions`,
      );
      await post(
        `/training/sessions/${session.session.id}/materials/${material.id}/${action}`,
        { command_id: crypto.randomUUID(), expected_version: p.version },
      );
      notify(
        action === "start-final"
          ? "Final review is ready. Open training to begin."
          : "Rehab skipped. Your main review date is unchanged.",
      );
      onChanged();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }
  async function remove() {
    setBusy(true);
    try {
      await api(`/folders/${folder.id}/materials/${material.id}`, {
        method: "DELETE",
      });
      notify("Material removed from your library.");
      onChanged();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title="Material details"
      subtitle={folder.title}
      onClose={onClose}
      drawer
    >
      <div className="form-body">
        <div className="details-heading">
          <span className="subtle-tag">{topicOf(material) || "No topic"}</span>
          <span className={`difficulty-label ${material.difficulty}`}>
            {material.difficulty}
          </span>
          <button className="button small" onClick={onEdit}>
            <Edit3 size={15} />
            Edit material
          </button>
        </div>
        {folder.config.schema.fields
          .filter((f) => f.active && material.values[f.key])
          .map((field) => (
            <section className="detail-field" key={field.key}>
              <span className="eyebrow">{field.label}</span>
              <Markdown value={material.values[field.key]!} field={field.key} />
            </section>
          ))}
        <div className="progress-panel">
          <h3>Learning progress</h3>
          {p ? (
            <>
              <div className="detail-pair">
                <span>Algorithm</span>
                <strong>
                  {algorithmNames[p.algorithm_key] || p.algorithm_key}
                </strong>
              </div>
              <div className="detail-pair">
                <span>Stage</span>
                <strong>{p.completed_at ? "Completed" : p.stage}</strong>
              </div>
              <div className="detail-pair">
                <span>Main review</span>
                <DateValue value={p.stage_review_at} />
              </div>
              {p.rehab_active && (
                <div className="detail-pair">
                  <span>Rehab · day {p.rehab_step + 1}</span>
                  <DateValue value={p.rehab_review_at} />
                </div>
              )}
              {p.extra_review_at && (
                <div className="detail-pair">
                  <span>Extra review</span>
                  <DateValue value={p.extra_review_at} />
                </div>
              )}
              {p.target_at && (
                <div className="detail-pair">
                  <span>Final review target</span>
                  <DateValue value={p.target_at} />
                </div>
              )}
              {p.can_start_final && (
                <button
                  className="button small"
                  disabled={busy}
                  onClick={() => progressAction("start-final")}
                >
                  Start final review early
                  <ArrowRight size={15} />
                </button>
              )}
              {p.rehab_active && (
                <button
                  className="text-button"
                  disabled={busy}
                  onClick={() => progressAction("skip-rehab")}
                >
                  Skip rehab
                </button>
              )}
            </>
          ) : (
            <p className="muted">
              Not started yet. This material will enter its learning schedule
              when it joins the training pool.
            </p>
          )}
        </div>
        {folder.template_key === "formulas" && (
          <ExerciseManager materialID={material.id} />
        )}
        {error && <ErrorBox error={error} />}
        {deleting ? (
          <div className="delete-confirm">
            <p>
              Remove “{materialTitle(material, folder)}” from your library and
              future training?
            </p>
            <button className="button danger" disabled={busy} onClick={remove}>
              Delete material
            </button>
            <button className="text-button" onClick={() => setDeleting(false)}>
              Keep material
            </button>
          </div>
        ) : (
          <button
            className="text-button danger-text"
            onClick={() => setDeleting(true)}
          >
            <Trash2 size={15} />
            Delete material
          </button>
        )}
      </div>
    </Modal>
  );
}
function ExerciseManager({ materialID }: { materialID: string }) {
  const [items, setItems] = useState<Exercise[]>([]);
  const [editing, setEditing] = useState<Exercise | "new" | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    let alive = true;
    void api<Exercise[]>(`/materials/${materialID}/exercises?limit=100`)
      .then((x) => {
        if (alive) setItems(x || []);
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
  }, [materialID, revision]);
  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    setBusy(true);
    setError("");
    try {
      const fields = Object.fromEntries(
        ["problem", "answer", "solution", "hint"].map((key) => [
          key,
          String(form.get(key) || ""),
        ]),
      );
      await api(
        `/materials/${materialID}/exercises${editing && editing !== "new" ? `/${editing.id}` : ""}`,
        {
          method: editing === "new" ? "POST" : "PUT",
          body: {
            ...fields,
            ...(editing && editing !== "new"
              ? { expected_version: editing.version }
              : {}),
          },
        },
      );
      setEditing(null);
      setRevision((n) => n + 1);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }
  async function remove(exercise: Exercise) {
    if (
      !window.confirm(
        "Delete this exercise? Already displayed cards will keep their current content.",
      )
    )
      return;
    setBusy(true);
    try {
      await api(
        `/materials/${materialID}/exercises/${exercise.id}?expected_version=${exercise.version}`,
        { method: "DELETE" },
      );
      setRevision((n) => n + 1);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="exercises">
      <div className="section-top">
        <h3>Practice exercises</h3>
        <button className="button small" onClick={() => setEditing("new")}>
          <Plus size={14} />
          Add exercise
        </button>
      </div>
      <p className="muted">
        Add at least one exercise to include this formula in training.
      </p>
      {loading ? (
        <Spinner label="Loading exercises…" />
      ) : (
        items.map((ex) => (
          <div className="exercise-item" key={ex.id}>
            <p>{ex.problem}</p>
            <div className="button-row">
              <button className="text-button" onClick={() => setEditing(ex)}>
                Edit
              </button>
              <button
                className="icon-button danger-text"
                aria-label="Delete exercise"
                disabled={busy}
                onClick={() => remove(ex)}
              >
                <Trash2 size={14} />
              </button>
            </div>
          </div>
        ))
      )}
      {editing && (
        <form
          key={editing === "new" ? "new" : editing.id}
          className="exercise-form"
          onSubmit={save}
        >
          {(["problem", "answer", "solution", "hint"] as const).map((key) => (
            <label key={key}>
              {key[0].toUpperCase() + key.slice(1)}
              {key === "hint" && <span className="optional">optional</span>}
              <textarea
                name={key}
                required={key !== "hint"}
                rows={key === "solution" ? 4 : 2}
                maxLength={key === "hint" ? 16000 : 64000}
                defaultValue={editing === "new" ? "" : editing[key]}
              />
            </label>
          ))}
          <div className="button-row">
            <button className="button primary small" disabled={busy}>
              {busy ? "Saving…" : "Save exercise"}
            </button>
            <button
              className="button small"
              type="button"
              onClick={() => setEditing(null)}
            >
              Cancel
            </button>
          </div>
        </form>
      )}
      {error && <ErrorBox error={error} />}
    </section>
  );
}
