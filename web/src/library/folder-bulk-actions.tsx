import { useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError, errorText } from "../api";
import type { Folder } from "../types";
import { ErrorBox, Modal } from "../ui";
import { useLibrary } from "./context";
import { colors, icons, type FolderAppearance } from "./preferences";
import { useLibraryPreferences } from "./preferences-context";
type Action = "move" | "customize" | "delete";
export function FolderBulkActions({
  selected,
  setSelected,
  shown,
}: {
  selected: string[];
  setSelected: (ids: string[]) => void;
  shown: Folder[];
}) {
  const { data, launch, reload, notify } = useLibrary();
  const { preferences, dispatch } = useLibraryPreferences();
  const navigate = useNavigate();
  const [action, setAction] = useState<Action | null>(null);
  const [section, setSection] = useState("");
  const [color, setColor] = useState("");
  const [icon, setIcon] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const guard = useRef(false);
  const ids = selected.filter((id) => data?.folders.some((f) => f.id === id));
  const mockIds = ids.filter((id) =>
    data?.folders.some(
      (f) => f.id === id && f.template_key === "interview_questions",
    ),
  );
  function patch(value: FolderAppearance) {
    dispatch({ type: "folders", ids, patch: value });
    setSelected([]);
    setAction(null);
  }
  function open(value: Action) {
    setError("");
    setAction(value);
    setColor("");
    setIcon("");
    setSection("");
  }
  async function remove() {
    if (guard.current) return;
    guard.current = true;
    setBusy(true);
    setError("");
    const removed: string[] = [];
    try {
      for (const id of ids) {
        try {
          await api(`/folders/${id}`, { method: "DELETE" });
        } catch (e) {
          if (!(e instanceof ApiError && e.status === 404)) throw e;
        }
        removed.push(id);
      }
      setAction(null);
      notify(`Deleted ${removed.length} folders.`);
    } catch (e) {
      setError(`${removed.length} folders deleted. ${errorText(e)}`);
    } finally {
      setSelected(ids.filter((id) => !removed.includes(id)));
      await reload();
      guard.current = false;
      setBusy(false);
    }
  }
  function mock() {
    navigate("/interview", {
      state: { mockSources: mockIds.map((folder_id) => ({ folder_id })) },
    });
  }
  const choose = (value: string) => {
    if (value === "favorite") patch({ favorite: true });
    else if (value === "unfavorite") patch({ favorite: false });
    else if (value === "mock") mock();
    else if (value) open(value as Action);
  };
  return (
    <>
      <div className="bulk-selection-controls">
        <button
          className="text-button"
          disabled={!shown.length}
          onClick={() => setSelected(shown.map((f) => f.id))}
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
          aria-label="Folder bulk actions"
        >
          <strong>{ids.length} selected</strong>
          <button className="button primary" onClick={() => launch(ids)}>
            Train
          </button>
          <button className="button" onClick={() => open("move")}>
            Move to section
          </button>
          <div className="bulk-desktop-actions">
            <button
              className="button"
              disabled={!mockIds.length}
              onClick={mock}
            >
              Start Mock
            </button>
            <button
              className="button"
              onClick={() => patch({ favorite: true })}
            >
              Favorite
            </button>
            <button
              className="button"
              onClick={() => patch({ favorite: false })}
            >
              Unfavorite
            </button>
            <button className="button" onClick={() => open("customize")}>
              Customize
            </button>
            <button
              className="button danger-text"
              onClick={() => open("delete")}
            >
              Delete
            </button>
          </div>
          <select
            className="bulk-mobile-more"
            aria-label="More folder actions"
            value=""
            onChange={(e) => choose(e.target.value)}
          >
            <option value="">More</option>
            <option value="customize">Customize</option>
            <option value="favorite">Favorite</option>
            <option value="unfavorite">Unfavorite</option>
            <option value="delete">Delete</option>
            <option value="mock" disabled={!mockIds.length}>
              Start Mock
            </option>
          </select>
        </div>
      )}
      {action && (
        <Modal
          title={
            action === "move"
              ? "Move folders to section"
              : action === "delete"
                ? "Delete selected folders"
                : "Customize selected folders"
          }
          onClose={() => {
            if (!busy) setAction(null);
          }}
        >
          <form
            className="form-body"
            onSubmit={(e) => {
              e.preventDefault();
              if (action === "delete") void remove();
              else if (action === "move")
                patch({ categoryId: section || null });
              else
                patch({
                  ...(color
                    ? { color: color as FolderAppearance["color"] }
                    : {}),
                  ...(icon ? { icon: icon as FolderAppearance["icon"] } : {}),
                });
            }}
          >
            {error && <ErrorBox error={error} />}
            <p>{ids.length} folders selected.</p>
            {action === "delete" ? (
              <p>
                The folders and their materials will be removed from your
                library. Confirm to continue.
              </p>
            ) : action === "move" ? (
              <label>
                Section
                <select
                  aria-label="Target section"
                  value={section}
                  onChange={(e) => setSection(e.target.value)}
                >
                  <option value="">Uncategorized</option>
                  {preferences.categories.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </label>
            ) : (
              <>
                <label>
                  Color
                  <select
                    aria-label="Bulk folder color"
                    value={color}
                    onChange={(e) => setColor(e.target.value)}
                  >
                    <option value="">Keep current</option>
                    {colors.map((v) => (
                      <option key={v} value={v}>
                        {v}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Icon
                  <select
                    aria-label="Bulk folder icon"
                    value={icon}
                    onChange={(e) => setIcon(e.target.value)}
                  >
                    <option value="">Keep current</option>
                    {icons.map((v) => (
                      <option key={v} value={v}>
                        {v}
                      </option>
                    ))}
                  </select>
                </label>
              </>
            )}
            <div className="dialog-actions">
              <button
                type="button"
                className="button"
                disabled={busy}
                onClick={() => setAction(null)}
              >
                Cancel
              </button>
              <button
                className="button primary"
                disabled={
                  busy ||
                  !ids.length ||
                  (action === "customize" && !color && !icon)
                }
              >
                {busy
                  ? "Saving..."
                  : action === "delete"
                    ? "Confirm delete folders"
                    : "Apply to selected"}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </>
  );
}
