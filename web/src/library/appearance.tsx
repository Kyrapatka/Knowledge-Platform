import {
  BookOpen,
  Brain,
  Code2,
  Database,
  Folder,
  GraduationCap,
  Languages,
  Rocket,
  Sigma,
} from "lucide-react";
import type { FolderAppearance } from "./preferences";
import { TypeIcon } from "./presentation";
const iconMap = {
  folder: Folder,
  code: Code2,
  brain: Brain,
  book: BookOpen,
  language: Languages,
  database: Database,
  math: Sigma,
  graduation: GraduationCap,
  rocket: Rocket,
};
export function FolderIcon({
  kind,
  appearance,
  size = 23,
}: {
  kind: string;
  appearance?: FolderAppearance;
  size?: number;
}) {
  const key = appearance?.icon;
  const Icon = key && key !== "default" ? iconMap[key] : undefined;
  return Icon ? <Icon size={size} /> : <TypeIcon kind={kind} size={size} />;
}
