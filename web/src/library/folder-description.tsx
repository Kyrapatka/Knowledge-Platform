import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
export function FolderDescription({
  value,
  compact = false,
}: {
  value: string;
  compact?: boolean;
}) {
  return (
    <div className={`folder-description-text ${compact ? "compact" : ""}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        components={{
          a: ({ href, children }) =>
            href && /^https?:\/\//i.test(href) ? (
              <a href={href} target="_blank" rel="noopener noreferrer">
                {children}
              </a>
            ) : (
              <span>{children}</span>
            ),
          img: ({ alt }) => <span>{alt || ""}</span>,
        }}
      >
        {value}
      </ReactMarkdown>
    </div>
  );
}
