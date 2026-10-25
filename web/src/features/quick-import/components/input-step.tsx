import type { RefObject } from "react";
export function InputStep({
  input,
  kind,
  busy,
  hasDraft,
  changed,
  original,
  template,
  textRef,
  onInput,
  onKind,
  onUpload,
  onParse,
  onReview,
  onEdited,
}: {
  input: string;
  kind: "text" | "json";
  busy: boolean;
  hasDraft: boolean;
  changed: boolean;
  original: boolean;
  template: string;
  textRef: RefObject<HTMLTextAreaElement | null>;
  onInput: (s: string) => void;
  onKind: (s: "text" | "json") => void;
  onUpload: (f?: File) => void;
  onParse: () => void;
  onReview: () => void;
  onEdited: () => void;
}) {
  return (
    <fieldset disabled={busy} className="qi-form qi-input">
      <div className="button-row" role="group" aria-label="Input format">
        <button
          className="button"
          aria-pressed={kind === "text"}
          disabled={original}
          onClick={() => onKind("text")}
        >
          Paste text
        </button>
        <button
          className="button"
          aria-pressed={kind === "json"}
          disabled={original}
          onClick={() => onKind("json")}
        >
          Upload JSON
        </button>
      </div>
      {kind === "json" && !original && (
        <label className="qi-field">
          JSON file
          <input
            type="file"
            accept=".json,application/json"
            onChange={(e) => onUpload(e.target.files?.[0])}
          />
        </label>
      )}
      {original && (
        <p>
          Original source. Your edited input and reviewed cards are kept
          separately.{" "}
          <button className="qi-source-link" onClick={onEdited}>
            Return to edited source
          </button>
        </p>
      )}
      <label className="qi-field">
        {original
          ? "Original source"
          : kind === "text"
            ? "Paste your materials"
            : "JSON source"}
        <textarea
          ref={textRef}
          className="qi-source"
          aria-label="Import source"
          rows={12}
          value={input}
          readOnly={original}
          onChange={(e) => onInput(e.target.value)}
          placeholder={
            template === "english_words"
              ? "subtle - тонкий\nThere is a subtle difference."
              : "Question: What is a goroutine?\nShort answer: A lightweight concurrent function."
          }
        />
      </label>
      <small className="muted">
        {kind === "text" ? "Text up to 1 MiB" : "UTF-8 JSON up to 10 MiB"} ·
        Review before saving.
      </small>
      {changed && hasDraft && (
        <p className="muted">
          The source has changed. Parsing it again will replace reviewed edits
          after confirmation.
        </p>
      )}
      <div className="button-row qi-input-actions">
        <button
          className="button primary"
          disabled={!input.trim() || original}
          onClick={onParse}
        >
          {busy
            ? "Parsing…"
            : hasDraft && changed
              ? "Reparse source"
              : "Review import"}
        </button>
        {hasDraft && (
          <button className="button" onClick={onReview}>
            Back to review
          </button>
        )}
      </div>
    </fieldset>
  );
}
