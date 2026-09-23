import type { Presentation, SessionView } from "./types";

export type InterviewGraphConfig = {
	interview_mode: "real" | "balanced" | "custom" | "deep";
	depth_level: number;
	custom_weights?: Record<string, number>;
  max_roots: number;
  max_depth_per_branch: number;
  max_forks_per_root: number;
  question_limit: number;
  temperature: number;
  max_detected_concepts: number;
  cross_topic_penalty: number;
  early_review_policy: "no_credit";
  store_raw_answer: boolean;
  include_draft: boolean;
  profile: string;
  level: number;
  max_frontier_size: number;
};

export const defaultGraphConfig: InterviewGraphConfig = {
  interview_mode: "real",
  depth_level: 1,
  max_roots: 6,
  max_depth_per_branch: 20,
  max_forks_per_root: 2,
  question_limit: 24,
  temperature: 0.85,
  max_detected_concepts: 6,
  cross_topic_penalty: 0.8,
  early_review_policy: "no_credit",
  store_raw_answer: false,
  include_draft: false,
  profile: "all",
  level: 3,
  max_frontier_size: 12,
};

export const interviewProfiles = [
  ["all", "All profiles"], ["go_core", "Go core"], ["go_middle", "Go middle"],
  ["go_strong_middle", "Go strong middle"], ["go_postgres", "Go + PostgreSQL"],
  ["backend_core", "Backend core"], ["backend_full", "Backend full"],
  ["distributed_backend", "Distributed backend"], ["infrastructure", "Infrastructure"],
  ["system_design", "System design"],
] as const;
export const interviewLevels = ["Junior", "Junior+", "Middle", "Strong middle", "Senior"];

export type GraphMatch = { slug: string; strength: number; source?: string };
export type InterviewPlan = {
  mode: string;
  depth_level: number;
  strategy: { target_roots: number; target_branch: number };
  topics: { key: string; label: string; available: number; weight: number; roots: number }[];
  slots: string[];
};
export type GraphCandidate = {
  material_id: string;
  seed_key?: string;
  score: number;
  question?: string;
  reason?: string;
  components?: Record<string, number>;
};
export type GraphSelection = {
  selection_event_id?: string;
  root_index: number;
  depth: number;
  probe: boolean;
  review_credit: boolean;
  bank_verification?: boolean;
  answer_incomplete?: boolean;
  warning?: string;
  reason?: string;
  detected_concepts?: GraphMatch[];
  candidates?: GraphCandidate[];
};
export type InterviewSession = Omit<SessionView, "current"> & {
  current: (Presentation & { interview_graph?: GraphSelection }) | null;
  undo_actions?: string[];
  graph?: {
    state: {
      practice_only?: boolean;
      answered_questions?: number;
      shown_root_ids?: string[];
      skipped_root_ids?: string[];
      completed_root_ids?: string[];
      interview_plan?: InterviewPlan;
      current_root: number;
      current_depth: number;
      roots_used: number;
      questions_asked: number;
      config: InterviewGraphConfig;
      recent_concepts?: string[];
      stop_reason?: string;
      current_branch?: string;
      random_index?: number;
      frontier?: { material_id: string; root_index: number; source_depth: number; status: string }[];
    };
    selection?: {
      detected_concepts: GraphMatch[];
      candidates: GraphCandidate[];
      selection_reason: string;
      root_index: number;
      depth_after: number;
      selected_question?: { seed_key?: string; topic?: string; subtopic?: string; status?: string };
    };
    statistics: {
      scheduled_reviews: number;
      interview_probes: number;
      correct: number;
      wrong: number;
      max_depth: number;
      average_depth: number;
      cross_domain_transitions: number;
      concept_coverage: {
        slug: string;
        asked: number;
        correct: number;
        wrong: number;
      }[];
    };
  };
};

export type InterviewProfile = {
  material_id?: string;
  seed_key?: string;
  frequency: number;
  frequency_confidence: number;
  interview_difficulty: number;
  specificity: number;
  root_weight: number;
  followup_weight: number;
  level_min: number;
  level_max: number;
  topic?: string;
  subtopic?: string;
  interview_profiles?: string[];
  status: "draft" | "ready" | "archived";
  profile_version?: number;
  domain?: string;
  concepts: {
    slug: string;
    role: "primary" | "tested" | "answer" | "hook" | "prerequisite" | "wrong_fallback";
    weight?: number;
    ordinal?: number;
  }[];
};
