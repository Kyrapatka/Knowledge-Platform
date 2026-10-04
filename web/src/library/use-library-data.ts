import { useCallback, useEffect, useRef, useState } from "react";
import { api, errorText } from "../api";
import type { Library } from "../types";
export function useLibraryData() {
  const [data, setData] = useState<Library | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const current = useRef<AbortController | null>(null);
  const reload = useCallback(async () => {
    current.current?.abort();
    const request = new AbortController();
    current.current = request;
    setError("");
    try {
      const next = await api<Library>("/library", { signal: request.signal });
      if (!request.signal.aborted) setData(next);
    } catch (e) {
      if (!request.signal.aborted) setError(errorText(e));
    } finally {
      if (!request.signal.aborted) setLoading(false);
    }
  }, []);
  useEffect(() => {
    void reload();
    return () => current.current?.abort();
  }, [reload]);
  return { data, loading, error, reload };
}
