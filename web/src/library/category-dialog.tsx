import { useState, type FormEvent } from "react";
import { errorText } from "../api";
import { ErrorBox, Modal } from "../ui";
import { createClientUUID } from "../uuid";
import { colors, icons, type Category } from "./preferences";
import { useLibraryPreferences } from "./preferences-context";
export function CategoryDialog({
  category,
  onClose,
}: {
  category?: Category;
  onClose: () => void;
}) {
  const { preferences, dispatch } = useLibraryPreferences();
  const [name, setName] = useState(category?.name || "");
  const [error, setError] = useState("");
  const [icon, setIcon] = useState<Category["icon"]>(
    category?.icon || "folder",
  );
  const [color, setColor] = useState<Category["color"]>(
    category?.color || "default",
  );
  const [deleting, setDeleting] = useState(false);
  function save(e: FormEvent) {
    e.preventDefault();
    if (!category && preferences.categories.length >= 100) {
      setError("You can create up to 100 sections.");
      return;
    }
    if (!name.trim()) {
      setError("Enter a section name.");
      return;
    }
    if (
      preferences.categories.some(
        (c) =>
          c.id !== category?.id &&
          c.name.toLocaleLowerCase() === name.trim().toLocaleLowerCase(),
      )
    ) {
      setError("This section name already exists.");
      return;
    }
    try {
      dispatch({
        type: "category",
        category: {
          id: category?.id || createClientUUID(),
          name: name.trim(),
          collapsed: category?.collapsed || false,
          icon,
          color,
        },
      });
      onClose();
    } catch (err) {
      setError(errorText(err));
    }
  }
  return (
    <Modal title={category ? "Edit section" : "New section"} onClose={onClose}>
      <form className="form-body" onSubmit={save}>
        {error && <ErrorBox error={error} />}
        <label>
          Section name
          <input
            aria-label="Section name"
            maxLength={64}
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </label>
        <label>
          Section icon
          <select
            aria-label="Section icon"
            value={icon}
            onChange={(e) => setIcon(e.target.value as Category["icon"])}
          >
            {icons.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </label>
        <label>
          Section color
          <select
            aria-label="Section color"
            value={color}
            onChange={(e) => setColor(e.target.value as Category["color"])}
          >
            {colors.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </label>
        {deleting && (
          <div className="error-box" role="alert">
            Delete this section? Its folders will remain in Uncategorized.
          </div>
        )}
        <div className="dialog-actions">
          <button type="button" className="button" onClick={onClose}>
            Cancel
          </button>
          <button className="button primary" type="submit">
            Save section
          </button>
        </div>
        {category && (
          <div className="category-danger">
            <button
              type="button"
              className="button danger-text"
              onClick={() => {
                if (!deleting) setDeleting(true);
                else {
                  dispatch({ type: "remove-category", id: category.id });
                  onClose();
                }
              }}
            >
              {deleting ? "Confirm delete section" : "Delete section"}
            </button>
          </div>
        )}
      </form>
    </Modal>
  );
}
