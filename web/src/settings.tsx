import { useEffect, useState, type FormEvent } from "react";
import { Check } from "lucide-react";
import { api, errorText, post, readStored, saveStored } from "./api";
import { useAuth } from "./auth";
import type { SavedTraining } from "./training";
import type { Folder, Plan } from "./types";
import { algorithmNames } from "./types";
import { ErrorBox, Modal, Spinner } from "./ui";

export const algorithmsFor = (folder: Folder) =>
  folder.template_key === "interview_questions"
    ? ["interview_long_term", "interview_cram"]
    : folder.template_key === "formulas"
      ? ["formula_adaptive"]
      : ["english_basic", "english_adaptive"];
export function PlanSettings({
  folder,
  planId,
  onClose,
  onSaved,
}: {
  folder: Folder;
  planId?: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { user } = useAuth();
  const [plan, setPlan] = useState<Plan | null>(null);
  const [key, setKey] = useState(folder.training_config.default_algorithm_key);
  const [pool, setPool] = useState(folder.training_config.pool_size);
  const [horizon, setHorizon] = useState(150);
  const [mode, setMode] = useState("keep");
  const [loading, setLoading] = useState(!!planId);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    if (planId)
      void api<Plan>(`/training/plans/${planId}`)
        .then((p) => {
          setPlan(p);
          setKey(p.algorithm_key);
          setPool(p.config.pool_size);
          setHorizon(p.config.horizon_days || 150);
        })
        .catch((e) => setError(errorText(e)))
        .finally(() => setLoading(false));
  }, [planId]);
  const isInterview = key.startsWith("interview");
  const trackChanged =
    !!plan &&
    (key === "interview_cram") !== (plan.algorithm_key === "interview_cram");
  async function save(e: FormEvent) {
    e.preventDefault();
    if (
      mode === "reset" &&
      !window.confirm(
        "Reset the current plan’s material progress to stage 1? Past training history will be retained.",
      )
    )
      return;
    setBusy(true);
    setError("");
    try {
      let target = plan?.status === "active" && !trackChanged ? plan : null;
      if (!target) {
        for (let offset = 0; ; offset += 100) {
          const candidates = await api<Plan[]>(
            `/training/plans?limit=100&offset=${offset}`,
          );
          target =
            candidates.find(
              (p) =>
                p.status === "active" &&
                p.algorithm_key === key &&
                p.source_folder_ids.includes(folder.id),
            ) || null;
          if (target || candidates.length < 100) break;
        }
      }
      let updated: Plan;
      if (target) {
        const result = await post<{ plan: Plan }>(
          `/training/plans/${target.id}/algorithm`,
          {
            command_id: crypto.randomUUID(),
            expected_version: target.version,
            algorithm_key: key,
            algorithm_version: 1,
            mode,
            horizon_days: isInterview ? horizon : 0,
            pool_size: pool,
            end_active_session: true,
          },
        );
        updated = result.plan;
      } else {
        updated = await post<Plan>("/training/plans", {
          source_folder_ids: [folder.id],
          algorithm_key: key,
          pool_size: pool,
          horizon_days: isInterview ? horizon : 0,
        });
      }
      setPlan(updated);
      const storageKey = `knowledge:training:${user.id}`;
      const saved = readStored<SavedTraining>(storageKey);
      if (saved)
        saveStored(storageKey, {
          sources: saved.sources.map((s) =>
            s.folder_id === folder.id
              ? { folder_id: folder.id, topics: s.topics, plan_id: updated.id }
              : s,
          ),
          session_ids: [],
        });
      await api(`/folders/${folder.id}/training-config`, {
        method: "PATCH",
        body: {
          version: folder.training_config_version,
          training_config: { default_algorithm_key: key, pool_size: pool },
        },
      });
      onSaved();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }
  function changeKey(value: string) {
    setKey(value);
    if (value === "interview_cram") setHorizon(5);
    else if (value === "interview_long_term") setHorizon(150);
  }
  return (
    <Modal
      title="Learning settings"
      subtitle={folder.title}
      onClose={() => {
        if (!busy) onClose();
      }}
    >
      <form className="form-body" onSubmit={save}>
        {loading ? (
          <Spinner />
        ) : (
          <>
            <label>
              Learning algorithm
              <select value={key} onChange={(e) => changeKey(e.target.value)}>
                {algorithmsFor(folder).map((k) => (
                  <option key={k} value={k}>
                    {algorithmNames[k]}
                  </option>
                ))}
              </select>
            </label>
            <div className="form-columns">
              <label>
                Pool size
                <input
                  type="number"
                  min={1}
                  max={50}
                  required
                  value={pool}
                  onChange={(e) => setPool(Number(e.target.value))}
                />
              </label>
              {isInterview && (
                <label>
                  Learning horizon · days
                  <input
                    type="number"
                    min={key === "interview_cram" ? 1 : 7}
                    max={key === "interview_cram" ? 7 : 365}
                    required
                    value={horizon}
                    onChange={(e) => setHorizon(Number(e.target.value))}
                  />
                </label>
              )}
            </div>
            <p className="info-note">
              The pool is the number of materials in active practice. Each
              material follows its own schedule.
              {isInterview &&
                " The learning horizon starts individually when a material begins learning."}
            </p>
            {plan?.status === "active" && !trackChanged ? (
              <label>
                Existing progress
                <select value={mode} onChange={(e) => setMode(e.target.value)}>
                  <option value="keep">
                    Keep progress and apply the updated plan
                  </option>
                  <option value="reset">Reset progress to stage 1</option>
                </select>
              </label>
            ) : (
              <p className="info-note">
                {trackChanged
                  ? "This switches learning tracks, reusing an existing plan when available. Progress in your previous plan will be retained."
                  : "These settings will create your learning plan."}
              </p>
            )}
            {!!plan && plan.source_folder_ids.length > 1 && (
              <p className="info-note">
                This plan is shared by {plan.source_folder_ids.length} folders.
                Updating it applies to all its materials.
              </p>
            )}
            {error && <ErrorBox error={error} />}
            <div className="dialog-actions">
              <button
                type="button"
                className="button"
                onClick={onClose}
                disabled={busy}
              >
                Cancel
              </button>
              <button className="button primary" disabled={busy}>
                {busy ? "Saving…" : "Save settings"}
                <Check size={16} />
              </button>
            </div>
          </>
        )}
      </form>
    </Modal>
  );
}
