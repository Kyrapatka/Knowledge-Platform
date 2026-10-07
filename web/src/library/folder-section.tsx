import { ChevronRight, MoreHorizontal, Star } from "lucide-react";
import type { Folder } from "../types";
import { FolderCard } from "./folder-card";
import type { Category } from "./preferences";
import { useLibraryPreferences } from "./preferences-context";
export function FolderSection({
  title,
  category,
  folders,
  selected,
  toggle,
  onEdit,
}: {
  title: string;
  category?: Category;
  folders: Folder[];
  selected: string[];
  toggle: (id: string) => void;
  onEdit?: () => void;
}) {
  const { dispatch } = useLibraryPreferences();
  const heading = (
    <>
      <span className="category-name" title={title}>
        {title}
      </span>
      <span className="count-pill">{folders.length}</span>
    </>
  );
  return (
    <section
      className={`category-section ${title === "Favorites" && !category ? "favorites-section" : ""}`}
      aria-label={title}
    >
      <div className="category-heading">
        {category ? (
          <button
            className="text-button category-toggle"
            aria-expanded={!category.collapsed}
            onClick={() =>
              dispatch({
                type: "category",
                category: { ...category, collapsed: !category.collapsed },
              })
            }
          >
            <ChevronRight
              className={category.collapsed ? "" : "expanded"}
              size={18}
            />
            {heading}
          </button>
        ) : (
          <h3>
            {title === "Favorites" && <Star size={17} />}
            {heading}
          </h3>
        )}
        {onEdit && (
          <button
            className="icon-button category-actions"
            aria-label={`Edit category ${title}`}
            title="Category actions"
            onClick={onEdit}
          >
            <MoreHorizontal size={20} />
          </button>
        )}
      </div>
      {!category?.collapsed && (
        <div className="folder-grid">
          {folders.map((folder) => (
            <FolderCard
              key={folder.id}
              folder={folder}
              selected={selected.includes(folder.id)}
              toggle={() => toggle(folder.id)}
            />
          ))}
          {!folders.length && (
            <p className="muted">
              No matching folders in this category. Use Customize on a folder to
              assign it.
            </p>
          )}
        </div>
      )}
    </section>
  );
}
