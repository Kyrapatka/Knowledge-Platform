export type Range = { start: number; end: number; line: number };
export type SchemaField = {
  key: string;
  active: boolean;
  required?: boolean;
  label?: string;
};
export type ImportFolder = {
  id: string;
  template_key: string;
  config: {
    schema: { fields: SchemaField[] };
    metadata_schema: { fields: SchemaField[] };
  };
};
export type Alternative = {
  cardId?: string;
  kind: "example" | "boundary";
  score: number;
  reasons: string[];
};
export type Issue = {
  id: string;
  code: string;
  severity: "error" | "warning";
  message: string;
  fragment: string;
  range: Range;
  path?: string;
  cardId?: string;
  resolved?: "excluded" | "assigned" | "accepted";
  blocking: boolean;
  actions: string[];
  alternatives?: Alternative[];
};
export type Example = { id: string; text: string; range: Range };
export type ProfileInput = {
  frequency?: number;
  difficulty?: number;
  level_min?: number;
  level_max?: number;
  topic?: string;
  subtopic?: string;
  concepts?: string[];
};
export type DraftCard = {
  id: string;
  values: Record<string, string>;
  metadata: Record<string, string>;
  difficulty: "easy" | "medium" | "hard";
  profile?: ProfileInput;
  examples: Example[];
  ranges: Record<string, Range>;
  range: Range;
  excluded: boolean;
  status: "ready" | "needs_review" | "invalid" | "excluded";
  duplicatePolicy?: "skip" | "allow" | "conflict";
  validation: string[];
  duplicateOf?: string;
};
export type ImportItem = {
  client_id: string;
  values: Record<string, string>;
  metadata: Record<string, string>;
  difficulty: string;
  profile?: ProfileInput;
  duplicate_policy?: string;
};
export type ImportCommand = {
  command_id: string;
  duplicate_policy: "skip" | "allow" | "conflict";
  items: ImportItem[];
};
export type ImportResult = {
  valid: boolean;
  created: number;
  skipped: number;
  failed: number;
  issues: {
    client_id?: string;
    path: string;
    code: string;
    severity: string;
    message: string;
  }[];
  items: {
    client_id: string;
    status: string;
    material_id?: string;
    duplicate_of?: string;
    duplicate_kind?: string;
  }[];
};
export type ImportDraft = {
  version: 1;
  draftId: string;
  folderId: string;
  kind: "text" | "json";
  source: string;
  cards: DraftCard[];
  issues: Issue[];
  automatic: { cards: DraftCard[]; issues: Issue[] };
  history: { at: number; action: string }[];
  updatedAt: number;
  pending?: ImportCommand;
};
