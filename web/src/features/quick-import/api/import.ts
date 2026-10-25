import { post } from "../../../api";
import type { ImportCommand, ImportResult } from "../model/types";
const path = (id: string) =>
  `/folders/${encodeURIComponent(id)}/materials/import`;
export const validateImport = (folderId: string, command: ImportCommand) =>
  post<ImportResult>(path(folderId) + "/validate", command);
export const commitImport = (folderId: string, command: ImportCommand) =>
  post<ImportResult>(path(folderId), command);
