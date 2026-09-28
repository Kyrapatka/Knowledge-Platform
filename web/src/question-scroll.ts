import { useLayoutEffect, useRef } from "react";

// Revealing an answer or retrying a command must not interrupt reading.
export function useQuestionScroll(id: string | undefined) {
  const container = useRef<HTMLDivElement>(null);
  const previous = useRef(id);
  useLayoutEffect(() => {
    const changed = !!previous.current && !!id && previous.current !== id;
    previous.current = id;
    if (!changed) return;
    const frame = requestAnimationFrame(() => {
      const element = container.current;
      if (!element) return;
      const top = element.getBoundingClientRect().top;
      const inset = parseFloat(getComputedStyle(element).scrollMarginTop) || 0;
      if (top < inset || top > window.innerHeight - 64)
        element.scrollIntoView({
          block: "start",
          behavior: window.matchMedia("(prefers-reduced-motion: reduce)")
            .matches
            ? "instant"
            : "smooth",
        });
    });
    return () => cancelAnimationFrame(frame);
  }, [id]);
  return container;
}
