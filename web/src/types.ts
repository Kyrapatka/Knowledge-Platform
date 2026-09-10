export type Field = {
  key: string;
  label: string;
  required: boolean;
  active: boolean;
};
export type FolderConfig = {
  schema: { fields: Field[] };
  metadata_schema: { fields: Field[] };
  card: { question_fields: string[]; answer_fields: string[] };
};
export type PlanOption = {
  id: string;
  algorithm_key: string;
  algorithm_version: number;
  track: string;
  status: string;
  horizon_days: number;
  pool_size: number;
  created_at: string;
};
export type Folder = {
  id: string;
  title: string;
  description: string;
  template_key: string;
  config: FolderConfig;
  config_version: number;
  training_config: { default_algorithm_key: string; pool_size: number };
  training_config_version: number;
  created_at: string;
  updated_at: string;
  material_count: number;
  due_count: number;
  learning_count: number;
  completed_count: number;
  topics: Topic[];
  selected_plan: PlanOption | null;
};
export type Topic = { name: string; count: number };
export type Progress = {
  stage: number;
  version: number;
  algorithm_key: string;
  track: string;
  learning_started_at: string;
  target_at: string | null;
  completed_at: string | null;
  stage_last_review_at: string | null;
  stage_review_at: string | null;
  rehab_active: boolean;
  rehab_review_at: string | null;
  rehab_step: number;
  extra_review_at: string | null;
  next_review_at: string | null;
  can_start_final: boolean;
};
export type Material = {
  id: string;
  folder_id: string;
  values: Record<string, string | null>;
  metadata: Record<string, string | null>;
  difficulty: "easy" | "medium" | "hard";
  created_at: string;
  updated_at: string;
  topic?: string;
  progress?: Progress | null;
};
export type Library = {
  folders: Folder[];
  totals: {
    folder_count: number;
    material_count: number;
    due_count: number;
    learning_count: number;
    completed_count: number;
  };
};
export type MaterialPage = {
  items: Material[];
  total: number;
  limit: number;
  offset: number;
  topics: Topic[];
  selected_plan: PlanOption | null;
  plans: PlanOption[];
};
export type Plan = {
  id: string;
  version: number;
  algorithm_key: string;
  algorithm_version: number;
  track: string;
  status: string;
  config: { pool_size: number; horizon_days: number };
  source_folder_ids: string[];
  created_at: string;
};
export type CardField = { key: string; label: string; value: string };
export type Presentation = {
  direction?: "foreign" | "native";
  example?: string;
  foreign_word?: string;
  id: string;
  material_id: string;
  folder_id: string;
  kind: string;
  stage: number;
  progress_version: number;
  consecutive_correct: number;
  rehab_consecutive_correct: number;
  required_correct: number;
  difficulty: string;
  question: CardField[];
  answer: CardField[];
  final_review: boolean;
  practice_mode?: string;
  exercise_id?: string;
};
export type Summary = {
  correct: number;
  wrong: number;
  materials_reviewed: number;
  stage_promotions: number;
};
export type SessionView = {
  session: { id: string; plan_id: string; status: string };
  current: Presentation | null;
  summary: Summary;
  pool_size: number;
};
export type CombinedView = {
  undo_actions?: string[];
  sessions: (SessionView & { plan: Plan })[];
  current: {
    session_id: string;
    plan_id: string;
    algorithm_key: string;
    presentation: Presentation;
  } | null;
  summary: Summary;
  next_review_at: string | null;
  empty_reason: string;
};
export type Source = {
  folder_id: string;
  topics?: string[];
  plan_id?: string;
  algorithm_key?: string;
  horizon_days?: number;
  pool_size?: number;
};
export type Exercise = {
  id: string;
  material_id: string;
  version: number;
  problem: string;
  answer: string;
  solution: string;
  hint: string;
};
export type Statistics = {
  days: number;
  timezone: string;
  totals: {
    answers: number;
    correct: number;
    wrong: number;
    materials_reviewed: number;
    sessions: number;
    stage_promotions: number;
    active_days: number;
  };
  daily: {
    stage_promotions: number;
    date: string;
    answers: number;
    correct: number;
    wrong: number;
    materials_reviewed: number;
  }[];
};
export const templateNames: Record<string, string> = {
  english_words: "Languages",
  interview_questions: "Interview prep",
  formulas: "Formulas",
};
export const algorithmNames: Record<string, string> = {
  english_basic: "English · Basic",
  english_adaptive: "English · Adaptive",
  interview_long_term: "Interview · Long-term",
  interview_cram: "Interview · CRAM",
  formula_adaptive: "Formula · Adaptive",
};
export const materialTitle = (m: Material, f?: Folder) =>
  (f?.config.card.question_fields || ["question", "foreign", "name"])
    .map((key) => m.values[key])
    .filter(Boolean)
    .join(" · ") ||
  Object.values(m.values).find(Boolean) ||
  "Untitled material";
export const topicOf = (m: Material) =>
  m.topic ?? m.metadata.topic ?? m.metadata.category ?? "";
