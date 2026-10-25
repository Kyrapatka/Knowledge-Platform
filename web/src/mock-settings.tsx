import { useState } from "react";
import { Settings2, ChevronDown } from "lucide-react";
import {
  interviewProfiles,
  interviewLevels,
  type InterviewGraphConfig,
} from "./interview-types";
export function MockSettings({
  config,
  setConfig,
  disabled = false,
  availableTopicKeys = [
    "go",
    "sql",
    "http",
    "architecture",
    "messaging",
    "ops",
    "other",
  ],
}: {
  config: InterviewGraphConfig;
  setConfig: React.Dispatch<React.SetStateAction<InterviewGraphConfig>>;
  disabled?: boolean;
  availableTopicKeys?: string[];
}) {
  const [advanced, setAdvanced] = useState(false);
  return (
    <>
      <label className="interview-label">
        Interview mode
        <select
          value={config.interview_mode}
          disabled={disabled}
          onChange={(e) =>
            setConfig({
              ...config,
              interview_mode: e.target
                .value as InterviewGraphConfig["interview_mode"],
              custom_weights: config.custom_weights || {
                go: 5,
                sql: 3,
                http: 2,
                architecture: 1,
                messaging: 1,
                ops: 1,
                other: 1,
              },
            })
          }
        >
          <option value="real">Real Interview</option>
          <option value="balanced">Balanced</option>
          <option value="custom">Custom</option>
          <option value="deep">Deep Interview</option>
        </select>
      </label>
      <p className="interview-mode-help">
        {
          {
            real: "A Go backend preset, adjusted to your selected subjects. Broad coverage with short branches.",
            balanced:
              "Give every available subject an equal share of the interview.",
            custom:
              "Set relative weights. For example, 5 / 3 / 2 means 50% / 30% / 20%.",
            deep: "Fewer starting topics, longer connected branches and more challenging follow-ups.",
          }[config.interview_mode]
        }
      </p>
      {config.interview_mode === "deep" && (
        <label className="interview-label">
          Depth level
          <select
            value={config.depth_level}
            disabled={disabled}
            onChange={(e) =>
              setConfig({ ...config, depth_level: +e.target.value })
            }
          >
            <option value={1}>1 — Focused</option>
            <option value={2}>2 — In depth</option>
            <option value={3}>3 — Expert deep dive</option>
          </select>
        </label>
      )}
      {config.interview_mode === "custom" && (
        <fieldset className="interview-weights" disabled={disabled}>
          <legend>Relative topic weights</legend>
          {Object.entries({
            go: "Go",
            sql: "SQL",
            http: "HTTP / Networks",
            architecture: "Architecture",
            messaging: "Messaging",
            ops: "Testing / Ops",
            other: "Other",
          })
            .filter(([key]) => availableTopicKeys.includes(key))
            .map(([key, label]) => (
              <label key={key}>
                {label}
                <input
                  aria-label={`${label} weight`}
                  type="number"
                  min={0}
                  max={1000000}
                  step="any"
                  value={config.custom_weights?.[key] ?? 0}
                  onChange={(e) =>
                    setConfig({
                      ...config,
                      custom_weights: {
                        ...config.custom_weights,
                        [key]: +e.target.value,
                      },
                    })
                  }
                />
              </label>
            ))}
          <small>
            Only topics with eligible questions participate. Zero excludes a
            topic.
          </small>
        </fieldset>
      )}
      <label className="interview-label">
        Questions
        <input
          type="number"
          min={1}
          max={100}
          value={config.question_limit}
          disabled={disabled}
          onChange={(e) =>
            setConfig((value) => ({
              ...value,
              question_limit: +e.target.value,
            }))
          }
        />
      </label>
      <label className="interview-label">
        Interview profile
        <select
          value={config.profile}
          disabled={disabled}
          onChange={(e) => setConfig({ ...config, profile: e.target.value })}
        >
          {interviewProfiles.map(([slug, label]) => (
            <option value={slug} key={slug}>
              {label}
            </option>
          ))}
        </select>
      </label>
      <label className="interview-label">
        Interview level
        <select
          value={config.level}
          disabled={disabled}
          onChange={(e) => setConfig({ ...config, level: +e.target.value })}
        >
          {interviewLevels.map((label, i) => (
            <option value={i + 1} key={label}>
              {label}
            </option>
          ))}
        </select>
      </label>
      <label className="interview-bank-mode">
        <input
          type="checkbox"
          checked={config.include_draft}
          disabled={disabled}
          onChange={(e) =>
            setConfig({ ...config, include_draft: e.target.checked })
          }
        />
        <span>
          <strong>Bank testing mode / Include drafts</strong>
          <small>
            Allows draft questions for graph testing. Practice only; does not
            modify SRS.
          </small>
        </span>
      </label>
      <button
        className="interview-advanced-toggle"
        type="button"
        onClick={() => setAdvanced(!advanced)}
      >
        <Settings2 size={16} /> Session options <ChevronDown size={15} />
      </button>
      {advanced && (
        <div className="interview-advanced">
          {(
            [
              ["max_depth_per_branch", "Maximum depth", 1, 20],
              ["max_forks_per_root", "Branches per topic", 0, 10],
            ] as const
          ).map(([key, label, min, max]) => (
            <label className="interview-label" key={key}>
              {label}
              <input
                type="number"
                min={min}
                max={max}
                disabled={disabled}
                value={config[key]}
                onChange={(e) =>
                  setConfig((value) => ({
                    ...value,
                    [key]: +e.target.value,
                  }))
                }
              />
            </label>
          ))}
        </div>
      )}
    </>
  );
}
