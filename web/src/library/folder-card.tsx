import { ArrowRight, ChevronRight, Palette, Star } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import type { Folder } from "../types";
import { templateNames } from "../types";
import { FolderIcon } from "./appearance";
import { useLibrary } from "./context";
import { CustomizeDialog } from "./customize-dialog";
import { FolderDescription } from "./folder-description";
import { useLibraryPreferences } from "./preferences-context";
export function FolderCard({
  folder,
  selected,
  toggle,
}: {
  folder: Folder;
  selected: boolean;
  toggle: () => void;
}) {
  const { launch } = useLibrary();
  const { preferences, dispatch } = useLibraryPreferences();
  const [customize, setCustomize] = useState(false);
  const appearance = preferences.folders[folder.id];
  return (
    <>
      <article
        className={`folder-card ${folder.template_key} folder-color-${appearance?.color || "default"} ${selected ? "selected" : ""}`}
      >
        <div className="folder-card-top">
          <span className="folder-icon">
            <FolderIcon kind={folder.template_key} appearance={appearance} />
          </span>
          <div className="folder-card-actions">
            <button
              className="icon-button"
              aria-label={`${appearance?.favorite ? "Unfavorite" : "Favorite"} ${folder.title}`}
              aria-pressed={!!appearance?.favorite}
              onClick={() =>
                dispatch({
                  type: "folder",
                  id: folder.id,
                  patch: { favorite: !appearance?.favorite },
                })
              }
            >
              <Star
                size={17}
                fill={appearance?.favorite ? "currentColor" : "none"}
              />
            </button>
            <button
              className="icon-button"
              aria-label={`Customize ${folder.title}`}
              onClick={() => setCustomize(true)}
            >
              <Palette size={17} />
            </button>
            <input
              type="checkbox"
              className="folder-checkbox"
              aria-label={`Select ${folder.title}`}
              checked={selected}
              onChange={toggle}
            />
          </div>
        </div>
        <Link className="folder-title-link" to={`/folders/${folder.id}`}>
          <h3>{folder.title}</h3>
        </Link>
        <FolderDescription
          compact
          value={folder.description || "A little knowledge worth keeping."}
        />
        {appearance?.lastUsedAt && (
          <small className="folder-last-used">
            Last used {new Date(appearance.lastUsedAt).toLocaleDateString()}
          </small>
        )}
        <div className="folder-tags">
          <span className="type-tag">
            {templateNames[folder.template_key] || "Collection"}
          </span>
          {folder.topics
            ?.filter((t) => t.name)
            .slice(0, 2)
            .map((t) => (
              <span className="subtle-tag" key={t.name}>
                {t.name}
              </span>
            ))}
        </div>
        <div className="folder-card-meta">
          <span>{folder.material_count} materials</span>
          <span className={folder.due_count ? "text-accent" : "muted"}>
            <span className="status-dot" />
            {folder.due_count
              ? `${folder.due_count} ready`
              : folder.learning_count
                ? "On schedule"
                : "Ready to begin"}
          </span>
        </div>
        <div className="folder-card-foot">
          <Link to={`/folders/${folder.id}`}>
            Open collection
            <ChevronRight size={15} />
          </Link>
          <button
            className="icon-button train-folder"
            title={`Train ${folder.title}`}
            aria-label={`Train ${folder.title}`}
            onClick={() => launch([folder.id])}
          >
            <ArrowRight size={17} />
          </button>
        </div>
      </article>
      {customize && (
        <CustomizeDialog folder={folder} onClose={() => setCustomize(false)} />
      )}
    </>
  );
}
