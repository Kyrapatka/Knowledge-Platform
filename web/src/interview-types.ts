import type { Presentation, SessionView } from "./types";

export type InterviewGraphConfig = {
  max_roots: number;
  max_depth_per_branch: number;
  max_forks_per_root: number;
  question_limit: number;
  temperature: number;
  max_detected_concepts: number;
  cross_topic_penalty: number;
  early_review_policy: "no_credit";
  store_raw_answer: boolean;
};

export const defaultGraphConfig: InterviewGraphConfig = {
  max_roots: 3,
  max_depth_per_branch: 10,
  max_forks_per_root: 2,
  question_limit: 24,
  temperature: 0.85,
  max_detected_concepts: 6,
  cross_topic_penalty: 0.35,
  early_review_policy: "no_credit",
  store_raw_answer: false,
};

export type GraphMatch = { slug: string; strength: number; source?: string };
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
  reason?: string;
  detected_concepts?: GraphMatch[];
  candidates?: GraphCandidate[];
};
export type InterviewSession = Omit<SessionView, "current"> & {
  current: (Presentation & { interview_graph?: GraphSelection }) | null;
  undo_actions?: string[];
  graph?: {
    state: {
      current_root: number;
      current_depth: number;
      roots_used: number;
      questions_asked: number;
      config: InterviewGraphConfig;
      recent_concepts?: string[];
      stop_reason?: string;
    };
    selection?: {
      detected_concepts: GraphMatch[];
      candidates: GraphCandidate[];
      selection_reason: string;
      root_index: number;
      depth_after: number;
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
  status: "draft" | "ready" | "archived";
  profile_version?: number;
  domain?: string;
  concepts: {
    slug: string;
    role: "primary" | "tested" | "hook" | "prerequisite";
    weight?: number;
  }[];
};
