import { useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ApiError, errorText, post } from "../api";
import type { Folder, Material } from "../types";
import { createClientUUID } from "../uuid";
import { ErrorBox, Modal } from "../ui";
import { useLibrary } from "./context";
import { useLibraryPreferences } from "./preferences-context";
type Action = "copy" | "delete" | "metadata";
type Request = {
  command_id: string;
  action: Action;
  material_ids: string[];
  target_folder_id?: string;
  allow_duplicate?: boolean;
  difficulty?: string;
  metadata?: Record<string, string | null>;
  profile?: Record<string, string | number>;
};
export function MaterialBulkActions({
  folder,
  shown,
  selected,
  setSelected,
  onSaved,
}: {
  folder: Folder;
  shown: Material[];
  selected: string[];
  setSelected: (ids: string[]) => void;
  onSaved: () => void;
}) {
  const { data, launch } = useLibrary();
  const navigate = useNavigate();
  const [action, setAction] = useState<Action | null>(null);
  const ids = shown.filter((m) => selected.includes(m.id)).map((m) => m.id);
  return (
    <>
      <div className="bulk-selection-controls">
        <button
          className="text-button"
          onClick={() => setSelected(shown.map((m) => m.id))}
        >
          Select all {shown.length} shown
        </button>
        {!!ids.length && (
          <button className="text-button" onClick={() => setSelected([])}>
            Clear selection
          </button>
        )}
      </div>
      {!!ids.length && (
        <div
          className="bulk-toolbar"
          role="region"
          aria-label="Material bulk actions"
        >
          <strong>{ids.length} selected</strong>
          <button
            className="button primary"
            onClick={() => launch([folder.id], undefined, undefined, ids)}
          >
            Train
          </button>
          <button className="button" onClick={() => setAction("copy")}>
            Copy
          </button>
          <div className="bulk-desktop-actions">
            <button
              className="button"
              disabled={folder.template_key !== "interview_questions"}
              onClick={() =>
                navigate("/interview", {
                  state: {
                    mockSources: [{ folder_id: folder.id, material_ids: ids }],
                  },
                })
              }
            >
              Start Mock
            </button>
            <button className="button" onClick={() => setAction("metadata")}>
              Edit metadata
            </button>
            <button
              className="button danger-text"
              onClick={() => setAction("delete")}
            >
              Delete
            </button>
          </div>
          <select
            className="bulk-mobile-more"
            value=""
            aria-label="More material actions"
            onChange={(e) => {
              if (e.target.value === "mock")
                navigate("/interview", {
                  state: {
                    mockSources: [{ folder_id: folder.id, material_ids: ids }],
                  },
                });
              else if (e.target.value) setAction(e.target.value as Action);
            }}
          >
            <option value="">More</option>
            <option value="metadata">Edit metadata</option>
            <option value="delete">Delete</option>
            <option
              value="mock"
              disabled={folder.template_key !== "interview_questions"}
            >
              Start Mock
            </option>
          </select>
        </div>
      )}
      {action && (
        <MaterialBulkDialog
          folder={folder}
          ids={ids}
          action={action}
          targets={(data?.folders || []).filter(
            (f) => f.id !== folder.id && f.template_key === folder.template_key,
          )}
          onClose={() => setAction(null)}
          onSaved={() => {
            setAction(null);
            setSelected([]);
            onSaved();
          }}
        />
      )}
    </>
  );
}
function MaterialBulkDialog({
  folder,
  ids,
  action,
  targets,
  onClose,
  onSaved,
}: {
  folder: Folder;
  ids: string[];
  action: Action;
  targets: Folder[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const navigate = useNavigate();
  const { notify } = useLibrary();
  const { preferences, dispatch } = useLibraryPreferences();
  const [target, setTarget] = useState(
    targets.some((f) => f.id === preferences.defaultCopyFolderId)
      ? preferences.defaultCopyFolderId!
      : "",
  );
  const [query, setQuery] = useState("");
  const [makeDefault, setMakeDefault] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const [enabled, setEnabled] = useState<Record<string, boolean>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [duplicate, setDuplicate] = useState<{
    id: string;
    folder: string;
  } | null>(null);
  const guard = useRef(false);
  const command = useRef<Request | null>(null);
  const fields = folder.config.metadata_schema.fields
    .filter(
      (f) =>
        f.active && ["topic", "subtopic", "company", "level"].includes(f.key),
    )
    .map((f) => ({
      key: f.key,
      label: f.label,
      kind: "metadata",
      min: 0,
      max: 0,
    }));
  fields.push({
    key: "difficulty",
    label: "Difficulty",
    kind: "difficulty",
    min: 0,
    max: 0,
  });
  if (folder.template_key === "interview_questions")
    fields.push(
      ...[
        {
          key: "frequency",
          label: "Interview frequency",
          kind: "profile",
          min: 1,
          max: 10,
        },
        {
          key: "subtopic",
          label: "Interview subtopic",
          kind: "profile",
          min: 0,
          max: 0,
        },
        {
          key: "level_min",
          label: "Minimum interview level",
          kind: "profile",
          min: 1,
          max: 5,
        },
        {
          key: "level_max",
          label: "Maximum interview level",
          kind: "profile",
          min: 1,
          max: 5,
        },
      ],
    );
  async function apply(force = false) {
    if (guard.current) return;
    guard.current = true;
    setBusy(true);
    setError("");
    try {
      if (!command.current) {
        const request: Request = {
          command_id: createClientUUID(),
          action,
          material_ids: [...ids],
        };
        if (action === "copy") {
          request.target_folder_id = target;
          request.allow_duplicate = force;
        }
        if (action === "metadata")
          for (const f of fields) {
            const k = f.kind + ":" + f.key;
            if (!enabled[k]) continue;
            const value = values[k] || "";
            if (f.kind === "difficulty") request.difficulty = value;
            else if (f.kind === "metadata")
              (request.metadata ||= {})[f.key] = value || null;
            else
              (request.profile ||= {})[f.key] = f.max ? Number(value) : value;
          }
        command.current = request;
      }
      const result = await post<{ affected: number }>(
        `/folders/${folder.id}/materials/bulk`,
        command.current,
      );
      if (action === "copy" && folder.template_key === "interview_questions")
        dispatch({ type: "copied", id: target, makeDefault });
      command.current = null;
      notify(
        `${result.affected} materials ${action === "copy" ? "copied" : action === "delete" ? "deleted" : "updated"}.`,
      );
      onSaved();
    } catch (e) {
      if (e instanceof ApiError && e.status >= 400 && e.status < 500)
        command.current = null;
      if (
        e instanceof ApiError &&
        e.code === "duplicate_question" &&
        typeof e.data?.existing_material_id === "string"
      )
        setDuplicate({ id: e.data.existing_material_id, folder: target });
      else setError(errorText(e));
    } finally {
      guard.current = false;
      setBusy(false);
    }
  }
  const locked = busy || !!command.current;
  return (
    <Modal
      title={
        action === "copy"
          ? "Copy selected materials"
          : action === "delete"
            ? "Delete selected materials"
            : "Edit selected metadata"
      }
      onClose={() => {
        if (!busy) onClose();
      }}
    >
      <form
        className="form-body"
        onSubmit={(e) => {
          e.preventDefault();
          void apply();
        }}
      >
        <p>{ids.length} materials selected.</p>
        {error && <ErrorBox error={error} />}
        {command.current && !busy && (
          <p>
            Retry uses the same command so confirmed changes are not applied
            twice.
          </p>
        )}
        <fieldset className="bulk-fields" disabled={locked}>
          {action === "copy" && (
            <>
              <p>
                Copies start with independent learning progress. Originals
                remain here.
              </p>
              <label>
                Search folders
                <input
                  aria-label="Search bulk copy folders"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
              </label>
              <label>
                Target folder
                <select
                  required
                  aria-label="Bulk copy target"
                  value={target}
                  onChange={(e) => {
                    setTarget(e.target.value);
                    setDuplicate(null);
                  }}
                >
                  <option value="">Choose a folder</option>
                  {targets
                    .filter(
                      (f) =>
                        f.id === target ||
                        f.title
                          .toLocaleLowerCase()
                          .includes(query.toLocaleLowerCase()),
                    )
                    .map((f) => (
                      <option key={f.id} value={f.id}>
                        {f.title}
                      </option>
                    ))}
                </select>
              </label>
              {folder.template_key === "interview_questions" && (
                <label className="check-label">
                  <input
                    type="checkbox"
                    checked={makeDefault}
                    onChange={(e) => setMakeDefault(e.target.checked)}
                  />
                  Make default copy folder
                </label>
              )}
            </>
          )}
          {action === "delete" && (
            <p>Delete these materials from this folder? Confirm to continue.</p>
          )}
          {action === "metadata" && (
            <>
              <p>
                Only enabled fields change. Blank metadata clears that field.
                Interview profile fields require existing question profiles.
              </p>
              {fields.map((f) => {
                const k = f.kind + ":" + f.key;
                return (
                  <div key={k} className="bulk-field">
                    <label className="check-label">
                      <input
                        type="checkbox"
                        checked={!!enabled[k]}
                        onChange={(e) =>
                          setEnabled({ ...enabled, [k]: e.target.checked })
                        }
                      />
                      Change {f.label}
                    </label>
                    {f.kind === "difficulty" ? (
                      <select
                        aria-label={`Bulk ${f.label}`}
                        disabled={!enabled[k]}
                        required={!!enabled[k]}
                        value={values[k] || ""}
                        onChange={(e) =>
                          setValues({ ...values, [k]: e.target.value })
                        }
                      >
                        <option value="">Keep current</option>
                        {["easy", "medium", "hard"].map((v) => (
                          <option key={v}>{v}</option>
                        ))}
                      </select>
                    ) : (
                      <input
                        aria-label={`Bulk ${f.label}`}
                        placeholder="Keep current"
                        disabled={!enabled[k]}
                        type={f.max ? "number" : "text"}
                        min={f.min}
                        max={f.max || undefined}
                        maxLength={200}
                        required={!!enabled[k] && !!f.max}
                        value={values[k] || ""}
                        onChange={(e) =>
                          setValues({ ...values, [k]: e.target.value })
                        }
                      />
                    )}
                  </div>
                );
              })}
            </>
          )}
        </fieldset>
        {duplicate && (
          <div className="duplicate-warning" role="alert">
            <p>
              A selected question already exists in the target. Nothing from
              this batch was copied.
            </p>
            <button
              type="button"
              className="button"
              disabled={busy}
              onClick={() =>
                navigate(
                  `/folders/${duplicate.folder}?material=${duplicate.id}`,
                )
              }
            >
              Open existing
            </button>
            <button
              type="button"
              className="button"
              disabled={busy}
              onClick={() => void apply(true)}
            >
              Create copies anyway
            </button>
          </div>
        )}
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
            disabled={
              busy ||
              !ids.length ||
              !!duplicate ||
              (action === "copy" && !target) ||
              (action === "metadata" && !Object.values(enabled).some(Boolean))
            }
          >
            {busy
              ? "Saving..."
              : command.current
                ? "Retry"
                : action === "copy"
                  ? "Copy materials"
                  : action === "delete"
                    ? "Confirm delete materials"
                    : "Apply metadata"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
