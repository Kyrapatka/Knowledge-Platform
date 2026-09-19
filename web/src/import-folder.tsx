import { useRef, useState, type DragEvent } from "react";
import {
  CheckCircle2,
  Copy,
  Download,
  FileJson,
  Sparkles,
  Upload,
} from "lucide-react";
import { ApiError, errorText, upload } from "./api";
import {
  exampleFileNames,
  exampleJSON,
  importPrompts,
  type ImportTemplate,
} from "./import-spec";
import { ErrorBox, Modal } from "./ui";

type ImportIssue = { item?: number; field: string; message: string };
type ImportPreview = {
  valid: boolean;
  format_version: number;
  template: ImportTemplate;
  folder_name: string;
  items_count: number;
  warnings: string[];
  errors?: ImportIssue[];
};
type ImportResult = {
  folder_id: string;
  folder_name: string;
  template: ImportTemplate;
  items_created: number;
  warnings: string[];
};

const labels: Record<ImportTemplate, string> = {
  interview_questions: "Interview questions",
  english_words: "English words",
};

export function ImportFolder({
  onClose,
  onImported,
}: {
  onClose: () => void;
  onImported: (result: ImportResult) => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [issues, setIssues] = useState<ImportIssue[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [template, setTemplate] =
    useState<ImportTemplate>("interview_questions");
  const [copied, setCopied] = useState(false);

  async function validate(next: File) {
    setFile(next);
    setPreview(null);
    setIssues([]);
    setError("");
    if (!next.name.toLowerCase().endsWith(".json")) {
      setError("Only .json files are supported.");
      return;
    }
    if (next.size > 10 * 1024 * 1024) {
      setError("The file is larger than 10 MB.");
      return;
    }
    setBusy(true);
    try {
      const result = await upload<ImportPreview>(
        "/folders/import/validate",
        next,
      );
      setPreview(result);
      setTemplate(result.template);
    } catch (cause) {
      const details = importErrors(cause);
      setIssues(details);
      setError(details.length ? "Cannot import this file." : errorText(cause));
    } finally {
      setBusy(false);
    }
  }

  function dropped(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    setDragging(false);
    const next = event.dataTransfer.files[0];
    if (next) void validate(next);
  }

  async function submit() {
    if (!file || !preview?.valid) return;
    setBusy(true);
    setError("");
    setIssues([]);
    try {
      onImported(await upload<ImportResult>("/folders/import", file));
    } catch (cause) {
      const details = importErrors(cause);
      setIssues(details);
      setError(details.length ? "Cannot import this file." : errorText(cause));
    } finally {
      setBusy(false);
    }
  }

  async function copyPrompt() {
    try {
      await navigator.clipboard.writeText(importPrompts[template]);
    } catch {
      const area = document.createElement("textarea");
      area.value = importPrompts[template];
      area.style.position = "fixed";
      area.style.opacity = "0";
      document.body.append(area);
      area.select();
      document.execCommand("copy");
      area.remove();
    }
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1800);
  }

  function downloadExample() {
    const blob = new Blob([exampleJSON(template) + "\n"], {
      type: "application/json;charset=utf-8",
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = exampleFileNames[template];
    link.click();
    URL.revokeObjectURL(url);
  }

  const countLabel =
    preview?.template === "english_words" ? "words" : "questions";
  return (
    <Modal
      title="Import folder"
      subtitle="Upload a prepared JSON file, check it, and start learning."
      onClose={() => {
        if (!busy) onClose();
      }}
      wide
    >
      <div className="form-body import-folder">
        <div
          className={`import-dropzone ${dragging ? "dragging" : ""}`}
          onDragEnter={(event) => {
            event.preventDefault();
            setDragging(true);
          }}
          onDragOver={(event) => event.preventDefault()}
          onDragLeave={() => setDragging(false)}
          onDrop={dropped}
        >
          <FileJson size={30} />
          <strong>Drop JSON file here</strong>
          <span>Supported format: .json · maximum 10 MB</span>
          <input
            ref={input}
            className="sr-only"
            type="file"
            accept=".json,application/json"
            aria-label="Choose JSON file"
            onChange={(event) => {
              const next = event.target.files?.[0];
              if (next) void validate(next);
            }}
          />
          <button
            type="button"
            className="button"
            disabled={busy}
            onClick={() => input.current?.click()}
          >
            <Upload size={16} />
            {busy ? "Validating…" : "Choose file"}
          </button>
          {file && <small className="import-file-name">{file.name}</small>}
        </div>

        {preview?.valid && (
          <section className="import-preview" aria-label="Import preview">
            <CheckCircle2 size={21} />
            <div>
              <span>Ready to import</span>
              <h3>{preview.folder_name}</h3>
              <dl>
                <div>
                  <dt>Template</dt>
                  <dd>{labels[preview.template]}</dd>
                </div>
                <div>
                  <dt>Items</dt>
                  <dd>
                    {preview.items_count} {countLabel}
                  </dd>
                </div>
                <div>
                  <dt>File</dt>
                  <dd>{file?.name}</dd>
                </div>
                <div>
                  <dt>Validation</dt>
                  <dd>Valid</dd>
                </div>
              </dl>
              {preview.warnings?.map((warning) => (
                <p className="import-warning" key={warning}>
                  {warning}
                </p>
              ))}
            </div>
          </section>
        )}

        {error && <ErrorBox error={error} />}
        {!!issues.length && (
          <ul className="import-errors" aria-label="Validation errors">
            {issues.map((issue, index) => (
              <li key={`${issue.item}-${issue.field}-${index}`}>
                <strong>{issue.item ? `Item ${issue.item}` : issue.field}</strong>
                <span>{issue.message}</span>
              </li>
            ))}
          </ul>
        )}

        <details className="import-guide" open={!file}>
          <summary>How to prepare a file</summary>
          <div className="import-guide-body">
            <ol>
              <li>Your file must be valid UTF-8 JSON.</li>
              <li>It must contain one folder and a non-empty “items” array.</li>
              <li>
                Choose a supported template: Interview questions or English
                words.
              </li>
              <li>Upload the file and check the preview before importing.</li>
            </ol>
            <div className="tabs import-tabs" aria-label="Example type">
              {(Object.keys(labels) as ImportTemplate[]).map((key) => (
                <button
                  type="button"
                  key={key}
                  className={template === key ? "active" : ""}
                  onClick={() => {
                    setTemplate(key);
                    setCopied(false);
                  }}
                >
                  {labels[key]}
                </button>
              ))}
            </div>
            <div className="import-section-heading">
              <div>
                <h3>JSON example</h3>
                <p>
                  Required fields: {template === "interview_questions" ? "question and short_answer" : "word and translation"}.
                </p>
              </div>
              <button type="button" className="button" onClick={downloadExample}>
                <Download size={15} /> Download example
              </button>
            </div>
            <pre className="import-code" tabIndex={0}>
              <code>{exampleJSON(template)}</code>
            </pre>
            <div className="import-ai">
              <div className="import-section-heading">
                <div>
                  <h3>
                    <Sparkles size={16} /> Create this file with AI
                  </h3>
                  <p>
                    Give this prompt to ChatGPT or another AI. Replace the text
                    in brackets with your topic.
                  </p>
                </div>
                <button
                  type="button"
                  className="button"
                  onClick={() => void copyPrompt()}
                >
                  {copied ? <CheckCircle2 size={15} /> : <Copy size={15} />}
                  {copied ? "Copied" : "Copy prompt"}
                </button>
              </div>
              <pre className="import-code import-prompt" tabIndex={0}>
                <code>{importPrompts[template]}</code>
              </pre>
            </div>
          </div>
        </details>

        <div className="dialog-actions import-actions">
          <button type="button" className="button" disabled={busy} onClick={onClose}>
            Cancel
          </button>
          <button
            type="button"
            className="button primary"
            disabled={busy || !preview?.valid}
            onClick={() => void submit()}
          >
            <Upload size={16} />
            {busy && preview ? "Importing…" : "Import folder"}
          </button>
        </div>
      </div>
    </Modal>
  );
}

function importErrors(error: unknown): ImportIssue[] {
  if (!(error instanceof ApiError)) return [];
  const direct = error.data?.errors;
  if (Array.isArray(direct)) return direct as ImportIssue[];
  const validation = error.data?.validation as
    | { errors?: ImportIssue[] }
    | undefined;
  return Array.isArray(validation?.errors) ? validation.errors : [];
}
