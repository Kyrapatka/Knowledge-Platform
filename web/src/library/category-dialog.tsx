import { useState, type FormEvent } from "react";
import { errorText } from "../api";
import { ErrorBox, Modal } from "../ui";
import { createClientUUID } from "../uuid";
import type { Category } from "./preferences";
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
  const [deleting, setDeleting] = useState(false);
  function save(e: FormEvent) {
    e.preventDefault();
    if (!category && preferences.categories.length >= 100) {
      setError("You can create up to 100 categories.");
      return;
    }
    if (!name.trim()) {
      setError("Enter a category name.");
      return;
    }
    if (
      preferences.categories.some(
        (c) =>
          c.id !== category?.id &&
          c.name.toLocaleLowerCase() === name.trim().toLocaleLowerCase(),
      )
    ) {
      setError("This category name already exists.");
      return;
    }
    try {
      dispatch({
        type: "category",
        category: {
          id: category?.id || createClientUUID(),
          name: name.trim(),
          collapsed: category?.collapsed || false,
        },
      });
      onClose();
    } catch (err) {
      setError(errorText(err));
    }
  }
  return (
    <Modal
      title={category ? "Edit category" : "New category"}
      onClose={onClose}
    >
      <form className="form-body" onSubmit={save}>
        {error && <ErrorBox error={error} />}
        <label>
          Category name
          <input
            aria-label="Category name"
            maxLength={64}
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </label>
        {deleting && (
          <div className="error-box" role="alert">
            Delete this category? Its folders will remain in Uncategorized.
          </div>
        )}
        <div className="dialog-actions">
          <button type="button" className="button" onClick={onClose}>
            Cancel
          </button>
          <button className="button primary" type="submit">
            Save category
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
              {deleting ? "Confirm delete category" : "Delete category"}
            </button>
          </div>
        )}
      </form>
    </Modal>
  );
}
