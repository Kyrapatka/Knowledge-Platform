import type { ImportCommand, ImportResult } from "../model/types";
export type PreparedImport = {
  body: ImportCommand;
  preview: ImportResult;
  excluded: number;
  fragments: number;
  revision: number;
};
export function ImportFooter({
  ready,
  total,
  excluded,
  fragments,
  fatal,
  busy,
  pending,
  prepared,
  saved,
  onPreview,
  onCommit,
  onCancel,
}: {
  ready: number;
  total: number;
  excluded: number;
  fragments: number;
  fatal: boolean;
  busy: boolean;
  pending: boolean;
  prepared: PreparedImport | null;
  saved: boolean;
  onPreview: () => void;
  onCommit: () => void;
  onCancel: () => void;
}) {
  return (
    <footer
      className="qi-footer"
      data-testid="import-footer"
      aria-label="Import actions"
    >
      {pending ? (
        <>
          <div role="status">
            <strong>
              {busy ? "Importing…" : "Import result is unconfirmed"}
            </strong>
            <p>
              The saved operation is protected. Retry safely to retrieve its
              result.
            </p>
          </div>
          <button className="button primary" disabled={busy} onClick={onCommit}>
            Retry same import
          </button>
        </>
      ) : prepared ? (
        <>
          <section aria-label="Confirm import" className="qi-confirm">
            <strong>Confirm import</strong>
            <p>
              {prepared.preview.created} will be created ·{" "}
              {prepared.preview.skipped} duplicates skipped ·{" "}
              {prepared.excluded} cards excluded · {prepared.fragments}{" "}
              unresolved fragments excluded.
            </p>
          </section>
          <div className="button-row">
            <button className="button" disabled={busy} onClick={onCancel}>
              Keep reviewing
            </button>
            <button
              className="button primary"
              disabled={busy}
              onClick={onCommit}
            >
              Confirm import
            </button>
          </div>
        </>
      ) : (
        <>
          <div aria-live="polite">
            <strong>
              {ready
                ? `${ready} ready${total - ready - excluded > 0 ? ` · ${total - ready - excluded} need attention` : ""}`
                : "No cards are ready yet"}
            </strong>
            {fragments > 0 && (
              <p>
                {fragments} source{" "}
                {fragments === 1 ? "fragment needs" : "fragments need"} a
                decision.
              </p>
            )}
            {fatal && <p>Correct the source before importing.</p>}
            {saved && (
              <small className="muted">Draft saved on this device</small>
            )}
          </div>
          <button
            className="button primary"
            disabled={busy || fatal || !ready}
            onClick={onPreview}
          >
            {busy
              ? "Checking…"
              : `Import ${ready}${ready + excluded < total || fragments ? " ready" : ""} ${ready === 1 ? "card" : "cards"}`}
          </button>
        </>
      )}
    </footer>
  );
}
