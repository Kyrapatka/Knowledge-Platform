import { useState, type FormEvent } from "react";
import type { Folder } from "../types";
import { Modal } from "../ui";
import { FolderIcon } from "./appearance";
import { colors, icons, type FolderAppearance } from "./preferences";
import { useLibraryPreferences } from "./preferences-context";
export function CustomizeDialog({
  folder,
  onClose,
}: {
  folder: Folder;
  onClose: () => void;
}) {
  const { preferences, dispatch } = useLibraryPreferences();
  const [draft, setDraft] = useState<FolderAppearance>(
    () => preferences.folders[folder.id] || {},
  );
  const patch = (value: FolderAppearance) =>
    setDraft((p) => ({ ...p, ...value }));
  function save(e: FormEvent) {
    e.preventDefault();
    dispatch({ type: "folder", id: folder.id, patch: draft });
    onClose();
  }
  return (
    <Modal title="Customize folder" subtitle={folder.title} onClose={onClose}>
      <form className="form-body" onSubmit={save}>
        <label>
          Section
          <select
            aria-label="Folder section"
            value={draft.categoryId || ""}
            onChange={(e) => patch({ categoryId: e.target.value || null })}
          >
            <option value="">Uncategorized</option>
            {preferences.categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
        <fieldset className="appearance-options">
          <legend>Color</legend>
          <div className="option-grid">
            {colors.map((color) => (
              <button
                key={color}
                type="button"
                className={`color-option folder-color-${color}`}
                aria-label={`Color ${color}`}
                aria-pressed={(draft.color || "default") === color}
                onClick={() => patch({ color })}
              >
                <span />
                {color}
              </button>
            ))}
          </div>
        </fieldset>
        <fieldset className="appearance-options">
          <legend>Icon</legend>
          <div className="option-grid">
            {icons.map((icon) => (
              <button
                key={icon}
                type="button"
                className="icon-option"
                aria-label={`Icon ${icon}`}
                aria-pressed={(draft.icon || "default") === icon}
                onClick={() => patch({ icon })}
              >
                <FolderIcon kind={folder.template_key} appearance={{ icon }} />
                {icon}
              </button>
            ))}
          </div>
        </fieldset>
        <label className="check-label">
          <input
            type="checkbox"
            checked={!!draft.favorite}
            onChange={(e) => patch({ favorite: e.target.checked })}
          />
          Favorite folder
        </label>
        <p className="muted">
          Organization and appearance are saved in this browser for your
          account.
        </p>
        <div className="dialog-actions">
          <button className="button" type="button" onClick={onClose}>
            Cancel
          </button>
          <button className="button primary" type="submit">
            Save appearance
          </button>
        </div>
      </form>
    </Modal>
  );
}
