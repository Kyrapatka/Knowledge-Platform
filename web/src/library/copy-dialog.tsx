import { useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ApiError, errorText, post } from "../api";
import type { Folder, Material } from "../types";
import { ErrorBox, Modal } from "../ui";
import { useLibrary } from "./context";
import { useLibraryPreferences } from "./preferences-context";
type CopyResult = { material_id: string; folder_id: string };
export function CopyDialog({
  folder,
  material,
  onClose,
}: {
  folder: Folder;
  material: Material;
  onClose: () => void;
}) {
  const { data, reload, notify } = useLibrary();
  const { preferences, dispatch } = useLibraryPreferences();
  const navigate = useNavigate();
  const targets = useMemo(
    () =>
      (data?.folders || []).filter(
        (f) => f.template_key === "interview_questions" && f.id !== folder.id,
      ),
    [data?.folders, folder.id],
  );
  const [target, setTarget] = useState(() =>
    targets.some((f) => f.id === preferences.defaultCopyFolderId)
      ? preferences.defaultCopyFolderId!
      : "",
  );
  const [query, setQuery] = useState("");
  const [makeDefault, setMakeDefault] = useState(false);
  const [busy, setBusy] = useState(false);
  const pending = useRef(false);
  const [error, setError] = useState("");
  const [duplicate, setDuplicate] = useState<CopyResult | null>(null);
  const selected = targets.find((f) => f.id === target);
  const filtered = targets.filter((f) =>
    f.title.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()),
  );
  const recent = preferences.recentCopyTargets.flatMap((id) =>
    filtered.filter((f) => f.id === id),
  );
  function choose(id: string) {
    setTarget(id);
    setDuplicate(null);
    setError("");
  }
  function close() {
    if (!pending.current) onClose();
  }
  async function copy(allowDuplicate = false) {
    if (pending.current || !selected) return;
    pending.current = true;
    setBusy(true);
    setError("");
    try {
      await post<CopyResult>(
        `/folders/${folder.id}/interview/questions/${material.id}/copy`,
        { target_folder_id: target, allow_duplicate: allowDuplicate },
      );
      dispatch({ type: "copied", id: target, makeDefault });
      notify(
        `Question copied to ${selected.title}. Learning progress starts independently.`,
      );
      void reload();
      onClose();
    } catch (err) {
      if (
        err instanceof ApiError &&
        err.code === "duplicate_question" &&
        typeof err.data?.existing_material_id === "string"
      )
        setDuplicate({
          material_id: err.data.existing_material_id,
          folder_id: target,
        });
      else {
        setError(errorText(err));
        if (err instanceof ApiError && err.status === 404) void reload();
      }
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  const option = (f: Folder, prefix: string) => (
    <button
      type="button"
      className={`copy-target ${target === f.id ? "selected" : ""}`}
      key={prefix + f.id}
      aria-pressed={target === f.id}
      disabled={busy}
      onClick={() => choose(f.id)}
    >
      {f.title}
      <small>
        {f.material_count} materials
        {f.id === preferences.defaultCopyFolderId ? " � Default" : ""}
      </small>
    </button>
  );
  return (
    <Modal title="Copy question" subtitle={folder.title} onClose={close}>
      <div className="form-body" aria-busy={busy}>
        {error && <ErrorBox error={error} />}
        <label>
          Search folders
          <input
            aria-label="Search copy folders"
            value={query}
            disabled={busy}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search by folder name�"
          />
        </label>
        {recent.length > 0 && (
          <section>
            <h3>Recent targets</h3>
            <div className="copy-target-list">
              {recent.map((f) => option(f, "recent"))}
            </div>
          </section>
        )}
        <section>
          <h3>Interview folders</h3>
          <div className="copy-target-list">
            {filtered.map((f) => option(f, "all"))}
          </div>
          {!filtered.length && (
            <p className="muted">
              {targets.length
                ? "No matching folders."
                : "Create another Interview folder to copy this question."}
            </p>
          )}
        </section>
        <p role="status">
          {selected ? `Selected: ${selected.title}` : "Choose a target folder."}
        </p>
        <label className="check-label">
          <input
            type="checkbox"
            checked={makeDefault}
            disabled={busy}
            onChange={(e) => setMakeDefault(e.target.checked)}
          />
          Make this my default copy folder
        </label>
        {preferences.defaultCopyFolderId && (
          <button
            type="button"
            className="text-button"
            disabled={busy}
            onClick={() => dispatch({ type: "clear-default" })}
          >
            Clear default copy folder
          </button>
        )}
        {duplicate && (
          <section className="duplicate-warning" role="alert">
            <h3>This question already exists in �{selected?.title}�.</h3>
            <p>
              You can open the existing question or explicitly create another
              independent copy.
            </p>
            <button
              className="button"
              disabled={busy}
              onClick={() => {
                navigate(
                  `/folders/${duplicate.folder_id}?material=${duplicate.material_id}`,
                );
                onClose();
              }}
            >
              Open existing
            </button>
          </section>
        )}
        <div className="dialog-actions">
          <button className="button" disabled={busy} onClick={close}>
            Cancel
          </button>
          <button
            className="button primary"
            disabled={busy || !selected}
            onClick={() => void copy(!!duplicate)}
          >
            {busy
              ? "Copying�"
              : duplicate
                ? "Create another copy"
                : "Copy question"}
          </button>
        </div>
      </div>
    </Modal>
  );
}
