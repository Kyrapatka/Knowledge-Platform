import { createContext, useContext } from "react";
import type { Library } from "../types";
export type LibraryContextValue = {
  data: Library | null;
  loading: boolean;
  error: string;
  reload: () => Promise<void>;
  notify: (message: string) => void;
  launch: (ids: string[], topics?: string[], planId?: string) => void;
};
export const LibraryContext = createContext<LibraryContextValue>(null!);
export const useLibrary = () => useContext(LibraryContext);
