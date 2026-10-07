import { useLayoutEffect } from "react";
import { useLocation, useNavigationType } from "react-router-dom";
import { useLibraryPreferences } from "./preferences-context";
export function useLibraryScroll(ready: boolean) {
  const { pathname, search } = useLocation();
  const type = useNavigationType();
  const { scrollPositions } = useLibraryPreferences();
  const key = pathname + search;
  useLayoutEffect(() => {
    if (!ready) return;
    window.scrollTo({
      top: type === "POP" ? scrollPositions.current[key] || 0 : 0,
      behavior: "instant",
    });
    const remember = () => {
      scrollPositions.current[key] = window.scrollY;
    };
    window.addEventListener("scroll", remember, { passive: true });
    return () => window.removeEventListener("scroll", remember);
  }, [ready, key, type, scrollPositions]);
}
