import { BookOpen } from "lucide-react";
import type { ReactNode } from "react";
export function TypeIcon({ kind, size = 23 }: { kind: string; size?: number }) {
  return kind === "interview_questions" ? (
    <span className="type-glyph">&lt;/&gt;</span>
  ) : kind === "formulas" ? (
    <span className="type-glyph formula-glyph">ƒ</span>
  ) : (
    <BookOpen size={size} />
  );
}
export function Metric({
  label,
  value,
  icon,
  accent,
}: {
  label: string;
  value: number | string;
  icon?: ReactNode;
  accent?: boolean;
}) {
  return (
    <div className={`metric ${accent ? "accent" : ""}`}>
      <span className="metric-label">
        {icon}
        {label}
      </span>
      <strong>
        {typeof value === "number" ? value.toLocaleString("en") : value}
      </strong>
    </div>
  );
}
