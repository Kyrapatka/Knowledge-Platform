import { useEffect, useRef, useState, type FormEvent } from "react";
import { api, ApiError, errorText, post } from "./api";
import { createClientUUID } from "./uuid";
import type { Folder, Source } from "./types";
import {
  defaultGraphConfig,
  type InterviewGraphConfig,
  type InterviewPlan,
  type InterviewSession,
} from "./interview-types";
import { ErrorBox, Modal, Spinner } from "./ui";
import { MockSettings } from "./mock-settings";
import { SectionFolderSelector } from "./library/section-folder-selector";
import { useLibraryPreferences } from "./library/preferences-context";
import { InterviewBankTools } from "./interview-editor";
export type MockPreset = {
  id: string;
  name: string;
  folder_ids: string[];
  settings: InterviewGraphConfig | null;
  version: number;
  created_at: string;
  updated_at: string;
  last_used_at: string | null;
  unavailable_folder_ids: string[];
};
const endpoint = "/training/mock-presets";
const equal = (a: InterviewGraphConfig, b: InterviewGraphConfig) =>
  JSON.stringify(a) === JSON.stringify(b);
export function MockSetup({
  folders,
  onStarted,
  initialSources,
}: {
  folders: Folder[];
  onStarted: (view: InterviewSession) => void;
  initialSources?: Source[];
}) {
  const { markUsed } = useLibraryPreferences();
  const [presets, setPresets] = useState<MockPreset[]>([]);
  const [global, setGlobal] = useState(defaultGraphConfig);
  const [config, setConfig] = useState(defaultGraphConfig);
  const [selected, setSelected] = useState<string[]>([]);
  const [manual, setManual] = useState(!!initialSources?.length);
  const [manualSources, setManualSources] = useState<Source[]>(
    initialSources || [],
  );
  const [step, setStep] = useState<"sources" | "settings">("sources");
  const [editor, setEditor] = useState<MockPreset | "new" | null>(null);
  const [sort, setSort] = useState("used");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [canResume, setCanResume] = useState(false);
  const [preview, setPreview] = useState<InterviewPlan | null>(null);
  const [previewError, setPreviewError] = useState("");
  const [previewBusy, setPreviewBusy] = useState(false);
  const [topicKeys, setTopicKeys] = useState<string[]>([]);
  const explicit = useRef(false);
  const starting = useRef(false);
  const command = useRef<{
    command_id: string;
    config: InterviewGraphConfig;
    sources?: Source[];
    preset_ids?: string[];
  } | null>(null);
  async function load() {
    setLoading(true);
    setError("");
    try {
      const [p, c] = await Promise.all([
        api<MockPreset[]>(endpoint),
        api<InterviewGraphConfig>("/training/interview-graph/config"),
      ]);
      setPresets(p);
      setGlobal(c);
      setConfig(c);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void load();
  }, []);
  useEffect(() => {
    if (initialSources?.length) {
      setManual(true);
      setManualSources(initialSources);
      setSelected([]);
      setStep("sources");
    }
  }, [initialSources]);
  const chosen = presets.filter((p) => selected.includes(p.id));
  const single = !manual && chosen.length === 1 ? chosen[0] : null;
  const baseline = single?.settings || global;
  const dirty = !equal(config, baseline);
  const sources = manual
    ? manualSources
    : [...new Set(chosen.flatMap((p) => p.folder_ids))].map((folder_id) => ({
        folder_id,
      }));
  const exact = manualSources.some((s) => s.material_ids !== undefined);
  const selectionBody = manual ? { sources } : { preset_ids: selected };
  const locked = busy || !!command.current;
  const previewKey = JSON.stringify({
    selectionBody,
    config,
    folders: folders.map((f) => [f.id, f.material_count, f.updated_at]),
  });
  useEffect(() => {
    let alive = true;
    setPreview(null);
    setPreviewError("");
    setPreviewBusy(!!sources.length);
    if (!sources.length) {
      setTopicKeys([]);
      return;
    }
    const timer = setTimeout(() => {
      void post<InterviewPlan>("/training/mock-interviews/preview", {
        ...selectionBody,
        config,
      })
        .then((p) => {
          if (alive) {
            setPreview(p);
            setTopicKeys(p.topics.map((t) => t.key));
          }
        })
        .catch(async (e) => {
          if (alive) setPreviewError(errorText(e));
          if (config.interview_mode === "custom")
            try {
              const p = await post<InterviewPlan>(
                "/training/mock-interviews/preview",
                {
                  ...selectionBody,
                  config: {
                    ...config,
                    interview_mode: "balanced",
                    custom_weights: undefined,
                  },
                },
              );
              if (alive) setTopicKeys(p.topics.map((t) => t.key));
            } catch {
              /* retain original error */
            }
        })
        .finally(() => {
          if (alive) setPreviewBusy(false);
        });
    }, 200);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [previewKey]);
  function choose(ids: string[], continueToSettings = false) {
    setSelected(ids);
    setManual(false);
    setError("");
    if (ids.length === 1) {
      setConfig(
        structuredClone(
          presets.find((p) => p.id === ids[0])?.settings || global,
        ),
      );
      explicit.current = false;
    } else if (!explicit.current) setConfig(structuredClone(global));
    if (continueToSettings) setStep("settings");
  }
  const changeConfig: React.Dispatch<
    React.SetStateAction<InterviewGraphConfig>
  > = (value) => {
    explicit.current = true;
    setConfig(value);
  };
  async function start(e: FormEvent) {
    e.preventDefault();
    if (starting.current || !sources.length) return;
    starting.current = true;
    setBusy(true);
    setError("");
    setCanResume(false);
    try {
      if (!command.current)
        command.current = {
          command_id: createClientUUID(),
          config: structuredClone(config),
          ...structuredClone(selectionBody),
        };
      const view = await post<InterviewSession>(
        "/training/mock-interviews",
        command.current,
      );
      markUsed(sources.map((s) => s.folder_id));
      command.current = null;
      onStarted(view);
    } catch (e) {
      if (e instanceof ApiError && e.status >= 400 && e.status < 500)
        command.current = null;
      setError(errorText(e));
      if (e instanceof ApiError && e.code === "active_mock_interview")
        setCanResume(true);
    } finally {
      starting.current = false;
      setBusy(false);
    }
  }
  function updated(p: MockPreset) {
    setPresets((values) =>
      values.some((v) => v.id === p.id)
        ? values.map((v) => (v.id === p.id ? p : v))
        : [p, ...values],
    );
  }
  async function saveSettings() {
    if (!single || locked) return;
    setBusy(true);
    setError("");
    try {
      const p = await api<MockPreset>(`${endpoint}/${single.id}`, {
        method: "PUT",
        body: {
          name: single.name,
          folder_ids: single.folder_ids,
          settings: config,
          expected_version: single.version,
        },
      });
      updated(p);
      explicit.current = false;
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  async function action(p: MockPreset, kind: string) {
    if (kind === "edit" || kind === "rename") {
      setEditor(p);
      return;
    }
    if (
      kind === "delete" &&
      !window.confirm(
        `Delete preset “${p.name}”? Folders and questions are retained.`,
      )
    )
      return;
    setBusy(true);
    setError("");
    try {
      if (kind === "duplicate") {
        updated(await post<MockPreset>(`${endpoint}/${p.id}/duplicate`, {}));
      } else if (kind === "delete") {
        await api(`${endpoint}/${p.id}`, { method: "DELETE" });
        setPresets((values) => values.filter((v) => v.id !== p.id));
        choose(selected.filter((id) => id !== p.id));
      }
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  const sorted = [...presets].sort((a, b) =>
    sort === "name"
      ? a.name.localeCompare(b.name)
      : sort === "created"
        ? b.created_at.localeCompare(a.created_at)
        : (b.last_used_at || "").localeCompare(a.last_used_at || "") ||
          b.created_at.localeCompare(a.created_at),
  );
  if (loading) return <Spinner label="Loading Mock presets…" />;
  return (
    <>
      <header className="interview-heading">
        <div>
          <span className="eyebrow">Practice a conversation</span>
          <h1>
            Mock interview<span>.</span>
          </h1>
          <p>Choose a saved preset or folders for a one-time interview.</p>
        </div>
      </header>
      <InterviewBankTools />
      {error && (
        <ErrorBox
          error={error}
          retry={!presets.length ? () => void load() : undefined}
        />
      )}
      <div className={`interview-setup mock-setup step-${step}`}>
        <section className="interview-setup-main mock-source-panel">
          <div className="button-row">
            <button
              type="button"
              className="button primary"
              disabled={locked}
              onClick={() => setEditor("new")}
            >
              + Create preset
            </button>
            <button
              type="button"
              className="button"
              disabled={locked}
              onClick={() => {
                setManual(true);
                setManualSources([]);
                setSelected([]);
                setConfig(structuredClone(global));
                explicit.current = false;
                setStep("sources");
              }}
            >
              Choose folders manually
            </button>
          </div>
          {manual ? (
            <>
              <h2>{exact ? "Selected questions" : "One-time Mock"}</h2>
              {exact ? (
                <p>
                  Only the{" "}
                  {manualSources.reduce(
                    (n, s) => n + (s.material_ids?.length || 0),
                    0,
                  )}{" "}
                  selected questions will participate.
                </p>
              ) : (
                <SectionFolderSelector
                  folders={folders}
                  value={manualSources.map((s) => s.folder_id)}
                  onChange={(ids) =>
                    setManualSources(
                      ids.map(
                        (folder_id) =>
                          manualSources.find(
                            (s) => s.folder_id === folder_id,
                          ) || { folder_id },
                      ),
                    )
                  }
                  disabled={locked}
                />
              )}
              {!exact &&
                manualSources.map((source) => {
                  const f = folders.find((f) => f.id === source.folder_id);
                  return (
                    !!f?.topics?.length && (
                      <details key={source.folder_id}>
                        <summary>Topics · {f.title}</summary>
                        <div className="topic-choices">
                          {f.topics
                            .filter((t) => t.name)
                            .map((t) => (
                              <button
                                key={t.name}
                                type="button"
                                disabled={locked}
                                className={
                                  source.topics?.includes(t.name)
                                    ? "selected"
                                    : ""
                                }
                                onClick={() =>
                                  setManualSources((all) =>
                                    all.map((s) =>
                                      s.folder_id === source.folder_id
                                        ? {
                                            ...s,
                                            topics: s.topics?.includes(t.name)
                                              ? s.topics.filter(
                                                  (x) => x !== t.name,
                                                )
                                              : [...(s.topics || []), t.name],
                                          }
                                        : s,
                                    ),
                                  )
                                }
                              >
                                {t.name}
                              </button>
                            ))}
                        </div>
                      </details>
                    )
                  );
                })}
              <p className="info-note">Nothing is saved as a preset.</p>
              <button
                type="button"
                className="button"
                disabled={locked}
                onClick={() => choose([])}
              >
                Back to presets
              </button>
            </>
          ) : (
            <>
              <div className="preset-list-heading">
                <h2>Your presets</h2>
                <label>
                  Sort presets
                  <select
                    value={sort}
                    onChange={(e) => setSort(e.target.value)}
                  >
                    <option value="used">Recently used</option>
                    <option value="created">Recently created</option>
                    <option value="name">Name</option>
                  </select>
                </label>
              </div>
              {!presets.length && (
                <p>
                  Create a preset to keep your usual interview folders together.
                </p>
              )}
              <div className="mock-presets">
                {sorted.map((p) => (
                  <article
                    key={p.id}
                    className={`mock-preset-card ${selected.includes(p.id) ? "selected" : ""}`}
                  >
                    <label className="preset-choice">
                      <input
                        type="checkbox"
                        checked={selected.includes(p.id)}
                        disabled={locked}
                        onChange={() =>
                          choose(
                            selected.includes(p.id)
                              ? selected.filter((id) => id !== p.id)
                              : [...selected, p.id],
                          )
                        }
                      />
                      <strong>{p.name}</strong>
                    </label>
                    <p>
                      {p.folder_ids.length} folders ·{" "}
                      {p.folder_ids.reduce(
                        (n, id) =>
                          n +
                          (folders.find((f) => f.id === id)?.material_count ||
                            0),
                        0,
                      )}{" "}
                      questions in folders
                    </p>
                    <small>
                      {p.settings
                        ? `${p.settings.interview_mode} · ${p.settings.question_limit} questions${p.settings.interview_mode === "deep" ? ` · Depth ${p.settings.depth_level}` : ""}`
                        : "Global defaults"}
                    </small>
                    <small>
                      {p.last_used_at
                        ? `Last used ${new Date(p.last_used_at).toLocaleDateString()}`
                        : "Not used yet"}
                    </small>
                    {!!p.unavailable_folder_ids?.length && (
                      <p role="status">
                        {p.unavailable_folder_ids.length} unavailable folders
                        ignored.
                      </p>
                    )}
                    {!p.folder_ids.length && (
                      <p>Add an available interview folder to start.</p>
                    )}
                    <div className="button-row">
                      <button
                        type="button"
                        className="button"
                        disabled={locked || !p.folder_ids.length}
                        onClick={() => choose([p.id], true)}
                      >
                        Start
                      </button>
                      <select
                        aria-label={`Actions for ${p.name}`}
                        value=""
                        disabled={locked}
                        onChange={(e) => void action(p, e.target.value)}
                      >
                        <option value="" disabled>
                          Actions…
                        </option>
                        <option value="edit">Edit</option>
                        <option value="rename">Rename</option>
                        <option value="duplicate">Duplicate</option>
                        <option value="delete">Delete</option>
                      </select>
                    </div>
                  </article>
                ))}
              </div>
            </>
          )}
          <p>{sources.length} folders selected</p>
          <button
            type="button"
            className="button primary mock-continue"
            disabled={locked || !sources.length}
            onClick={() => setStep("settings")}
          >
            Continue →
          </button>
        </section>
        <form
          className="interview-session-options mock-settings-panel"
          onSubmit={start}
        >
          <button
            type="button"
            className="button mock-back"
            disabled={locked}
            onClick={() => setStep("sources")}
          >
            ← Select sources
          </button>
          <h2>Session Settings</h2>
          <p>
            {single
              ? single.name
              : manual
                ? "One-time Mock"
                : `${selected.length} presets selected`}
          </p>
          {chosen.length > 1 && !manual && (
            <p className="info-note">
              Multiple presets selected. Saved preset settings are not combined.
              Settings below apply only to this session.
            </p>
          )}
          {dirty && <p role="status">Unsaved session changes</p>}
          <MockSettings
            config={config}
            setConfig={changeConfig}
            disabled={locked}
            availableTopicKeys={topicKeys}
          />
          <div className="preset-settings-actions">
            <button
              type="button"
              className="button"
              disabled={locked || !dirty}
              onClick={() => {
                setConfig(structuredClone(baseline));
                explicit.current = false;
              }}
            >
              {single ? "Reset to preset defaults" : "Reset to global defaults"}
            </button>
            {single && (
              <button
                type="button"
                className="button"
                disabled={locked || !dirty}
                onClick={() => void saveSettings()}
              >
                Save changes to preset
              </button>
            )}
          </div>
          {previewBusy && <p role="status">Checking available questions…</p>}
          {preview && (
            <div
              className="interview-distribution"
              aria-label="Interview distribution"
            >
              <strong>
                {preview.strategy.target_roots} root branches planned
              </strong>
              {preview.topics.map((t) => (
                <div key={t.key}>
                  <span>{t.label}</span>
                  <b>{(t.weight * 100).toFixed(1)}%</b>
                  <small>
                    {t.roots} roots · {t.available} available areas
                  </small>
                </div>
              ))}
            </div>
          )}
          {previewError && <ErrorBox error={previewError} />}
          {canResume && (
            <button
              type="button"
              className="button"
              disabled={busy}
              onClick={() => {
                setBusy(true);
                void api<InterviewSession>("/training/mock-interviews/active")
                  .then(onStarted)
                  .catch((e) => setError(errorText(e)))
                  .finally(() => setBusy(false));
              }}
            >
              Resume existing interview
            </button>
          )}
          <button
            className="button primary interview-start"
            disabled={
              busy ||
              !sources.length ||
              previewBusy ||
              !!previewError ||
              !preview
            }
          >
            {busy
              ? "Preparing…"
              : command.current
                ? "Retry start"
                : "Start Mock"}
          </button>
          <small>
            Session changes do not update presets. Mock practice leaves your SRS
            schedule unchanged.
          </small>
        </form>
      </div>
      {editor && (
        <PresetEditor
          folders={folders}
          preset={editor === "new" ? null : editor}
          defaults={global}
          onClose={() => setEditor(null)}
          onSaved={(p) => {
            updated(p);
            setEditor(null);
            if (
              selected.length === 1 &&
              selected[0] === p.id &&
              !explicit.current
            )
              setConfig(structuredClone(p.settings || global));
          }}
        />
      )}
    </>
  );
}
function PresetEditor({
  folders,
  preset,
  defaults,
  onClose,
  onSaved,
}: {
  folders: Folder[];
  preset: MockPreset | null;
  defaults: InterviewGraphConfig;
  onClose: () => void;
  onSaved: (p: MockPreset) => void;
}) {
  const [name, setName] = useState(preset?.name || "");
  const [ids, setIDs] = useState(preset?.folder_ids || []);
  const [custom, setCustom] = useState(!!preset?.settings);
  const [config, setConfig] = useState(() =>
    structuredClone(preset?.settings || defaults),
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const saving = useRef(false);
  async function save(e: FormEvent) {
    e.preventDefault();
    if (saving.current) return;
    saving.current = true;
    setBusy(true);
    setError("");
    try {
      const p = await api<MockPreset>(
        preset ? `${endpoint}/${preset.id}` : endpoint,
        {
          method: preset ? "PUT" : "POST",
          body: {
            name,
            folder_ids: ids,
            settings: custom ? config : null,
            expected_version: preset?.version,
          },
        },
      );
      onSaved(p);
    } catch (e) {
      setError(errorText(e));
    } finally {
      saving.current = false;
      setBusy(false);
    }
  }
  return (
    <Modal
      title={preset ? "Edit preset" : "Create preset"}
      wide
      onClose={() => {
        if (!busy) onClose();
      }}
    >
      <form className="form-body" onSubmit={save}>
        <label>
          Name
          <input
            autoFocus
            required
            maxLength={120}
            value={name}
            disabled={busy}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        {!!preset?.unavailable_folder_ids?.length && (
          <p>Unavailable folders will be removed when you save.</p>
        )}
        <SectionFolderSelector
          folders={folders}
          value={ids}
          onChange={setIDs}
          disabled={busy}
        />
        <label className="preset-choice">
          <input
            type="checkbox"
            checked={custom}
            disabled={busy}
            onChange={(e) => setCustom(e.target.checked)}
          />
          Save Mock default settings
        </label>
        {custom ? (
          <MockSettings config={config} setConfig={setConfig} disabled={busy} />
        ) : (
          <p>Use global defaults when this preset is selected.</p>
        )}
        {error && <ErrorBox error={error} />}
        <div className="dialog-actions">
          <button
            type="button"
            className="button"
            disabled={busy}
            onClick={onClose}
          >
            Cancel
          </button>
          <button className="button primary" disabled={busy || !name.trim()}>
            {busy ? "Saving…" : "Save preset"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
