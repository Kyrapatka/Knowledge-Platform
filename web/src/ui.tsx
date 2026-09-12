import { useEffect, useRef, type ReactNode } from "react";
import { AlertCircle, ArrowRight, Loader2, X } from "lucide-react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";

export function Logo({ compact = false }: { compact?: boolean }) {
  return (
    <span className="brand">
      <span className="brand-mark">
        <svg viewBox="0 0 28 28" aria-hidden="true">
          <path d="M8 5v18M11 14l10-9M11 14l10 9" />
        </svg>
      </span>
      {!compact && (
        <span>
          knowledge<span className="brand-dot">.</span>
        </span>
      )}
    </span>
  );
}
export function Spinner({ label = "Loading your space…" }: { label?: string }) {
  return (
    <div className="loading" role="status">
      <Loader2 className="spin" size={22} />
      <span>{label}</span>
    </div>
  );
}
export function ErrorBox({
  error,
  retry,
}: {
  error: string;
  retry?: () => void;
}) {
  return (
    <div className="error-box" role="alert">
      <AlertCircle size={18} />
      <span>{error}</span>
      {retry && (
        <button className="text-button" onClick={retry}>
          Try again
        </button>
      )}
    </div>
  );
}
export function Empty({
  icon,
  title,
  children,
  action,
}: {
  icon?: ReactNode;
  title: string;
  children: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="empty">
      <div className="empty-icon">{icon || <ArrowRight />}</div>
      <h2>{title}</h2>
      <p>{children}</p>
      {action}
    </div>
  );
}
export function Modal({
  title,
  subtitle,
  children,
  onClose,
  wide = false,
  drawer = false,
}: {
  title: string;
  subtitle?: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
  drawer?: boolean;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current!;
    const previous = document.activeElement as HTMLElement | null;
    dialog.showModal();
    const original = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const viewport = window.visualViewport;
    let frame = 0;
    const keepFieldVisible = () => {
      const content = dialog.querySelector<HTMLElement>(".dialog-content");
      const active = document.activeElement;
      if (
        !content ||
        !(active instanceof HTMLElement) ||
        !content.contains(active) ||
        !active.matches("input, textarea, select, [contenteditable='true']")
      )
        return;
      const bounds = content.getBoundingClientRect();
      const actions = content.querySelector<HTMLElement>(".dialog-actions");
      const top = bounds.top + (actions?.offsetHeight || 0) + 18;
      const bottom = bounds.bottom - 18;
      const field = active.getBoundingClientRect();
      if (field.bottom > bottom)
        content.scrollTop += Math.min(field.bottom - bottom, field.top - top);
      else if (field.top < top) content.scrollTop += field.top - top;
    };
    const fitViewport = () => {
      const height = viewport?.height ?? window.innerHeight;
      dialog.style.setProperty("--dialog-viewport-height", `${height}px`);
      dialog.style.setProperty(
        "--dialog-viewport-offset",
        `${viewport?.offsetTop || 0}px`,
      );
      dialog.classList.toggle(
        "keyboard-open",
        window.innerHeight - height > 120,
      );
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(keepFieldVisible);
    };
    fitViewport();
    viewport?.addEventListener("resize", fitViewport);
    viewport?.addEventListener("scroll", fitViewport);
    window.addEventListener("resize", fitViewport);
    dialog.addEventListener("focusin", fitViewport);
    return () => {
      cancelAnimationFrame(frame);
      viewport?.removeEventListener("resize", fitViewport);
      viewport?.removeEventListener("scroll", fitViewport);
      window.removeEventListener("resize", fitViewport);
      dialog.removeEventListener("focusin", fitViewport);
      dialog.close();
      document.body.style.overflow = original;
      previous?.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      className={`dialog keyboard-aware ${wide ? "wide" : ""} ${drawer ? "drawer" : ""}`}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      onClick={(e) => {
        if (e.target === ref.current) {
          const r = ref.current.getBoundingClientRect();
          if (
            e.clientX < r.left ||
            e.clientX > r.right ||
            e.clientY < r.top ||
            e.clientY > r.bottom
          )
            onClose();
        }
      }}
      aria-label={title}
    >
      <div className="dialog-header">
        <div>
          <h2>{title}</h2>
          {subtitle && <p>{subtitle}</p>}
        </div>
        <button
          className="icon-button"
          aria-label="Close dialog"
          onClick={onClose}
        >
          <X size={20} />
        </button>
      </div>
      <div className="dialog-content">{children}</div>
    </dialog>
  );
}
export function Markdown({ value, field }: { value: string; field?: string }) {
  const text =
    field === "code_example" && !value.includes("```")
      ? "```\n" + value + "\n```"
      : field === "formula" && !value.includes("$")
        ? "$$\n" + value + "\n$$"
        : value;
  return (
    <div className="markdown">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[[rehypeKatex, { trust: false, strict: "ignore" }]]}
        skipHtml
        components={{
          a: ({ children, ...props }) => (
            <a {...props} target="_blank" rel="noopener noreferrer">
              {children}
            </a>
          ),
          img: ({ alt }) => (
            <span className="muted">[Image: {alt || "external image"}]</span>
          ),
        }}
      >
        {text}
      </ReactMarkdown>
    </div>
  );
}
export function dateLabel(value?: string | null) {
  if (!value) return "Not scheduled";
  const date = new Date(value);
  const now = new Date();
  if (date <= now) return "Ready now";
  const moscowDay = (d: Date) =>
    new Intl.DateTimeFormat("en-CA", {
      timeZone: "Europe/Moscow",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(d);
  const day = moscowDay(date);
  const today = moscowDay(now);
  const time =
    date.toLocaleTimeString("en-GB", {
      timeZone: "Europe/Moscow",
      hour: "2-digit",
      minute: "2-digit",
    }) + " MSK";
  if (day === today) return `Today, ${time}`;
  if (day === moscowDay(new Date(now.getTime() + 86400000)))
    return `Tomorrow, ${time}`;
  return (
    date.toLocaleDateString("en-GB", {
      timeZone: "Europe/Moscow",
      month: "short",
      day: "numeric",
      year: "numeric",
    }) + `, ${time}`
  );
}
export function DateValue({ value }: { value?: string | null }) {
  return (
    <span
      title={
        value
          ? new Date(value).toLocaleString("en-GB", {
              timeZone: "Europe/Moscow",
            }) + " MSK"
          : undefined
      }
      className={value && new Date(value) <= new Date() ? "text-accent" : ""}
    >
      {dateLabel(value)}
    </span>
  );
}
