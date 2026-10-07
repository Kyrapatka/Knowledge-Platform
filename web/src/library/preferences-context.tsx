import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import type { Folder } from "../types";
import {
  libraryPreferencesStorage,
  prunePreferences,
  updatePreferences,
  type LibraryPreferences,
  type PreferenceAction,
} from "./preferences";

type PreferencesContextValue = {
  preferences: LibraryPreferences;
  dispatch: (action: PreferenceAction) => void;
  prune: (folders: Folder[]) => void;
  markUsed: (ids: string[]) => void;
  storageFailed: boolean;
  scrollPositions: { current: Record<string, number> };
};
const Context = createContext<PreferencesContextValue | null>(null);
export const useLibraryPreferences = () => {
  const value = useContext(Context);
  if (!value) throw new Error("Library preferences provider is missing");
  return value;
};
export function LibraryPreferencesProvider({
  userId,
  children,
}: {
  userId: string;
  children: ReactNode;
}) {
  const scrollPositions = useRef<Record<string, number>>({});
  const [preferences, setPreferences] = useState(() =>
    libraryPreferencesStorage.load(userId),
  );
  const [storageFailed, setStorageFailed] = useState(false);
  useEffect(() => {
    setStorageFailed(!libraryPreferencesStorage.save(userId, preferences));
  }, [userId, preferences]);
  const dispatch = useCallback(
    (action: PreferenceAction) =>
      setPreferences((p) => updatePreferences(p, action)),
    [],
  );
  const prune = useCallback(
    (folders: Folder[]) => setPreferences((p) => prunePreferences(p, folders)),
    [],
  );
  const markUsed = useCallback(
    (ids: string[]) => dispatch({ type: "used", ids, at: Date.now() }),
    [dispatch],
  );
  const value = useMemo(
    () => ({
      preferences,
      dispatch,
      prune,
      markUsed,
      storageFailed,
      scrollPositions,
    }),
    [preferences, dispatch, prune, markUsed, storageFailed],
  );
  return <Context.Provider value={value}>{children}</Context.Provider>;
}
