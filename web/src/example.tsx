import { useState } from "react";
import { Eye, EyeOff } from "lucide-react";

export function WordExample({
  example,
  word,
}: {
  example: string;
  word: string;
}) {
  const [shown, setShown] = useState(false);
  const [revealed, setRevealed] = useState<Set<number>>(new Set());
  const escaped = word.trim().replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const parts = escaped
    ? example.split(
        new RegExp(
          `(?<![\\p{L}\\p{N}_])(${escaped})(?![\\p{L}\\p{N}_])`,
          "giu",
        ),
      )
    : [example];
  return (
    <div className="word-example">
      <button
        className="text-button example-toggle"
        onClick={() => setShown(!shown)}
        aria-expanded={shown}
      >
        {shown ? <EyeOff size={17} /> : <Eye size={17} />}
        {shown ? "Hide example" : "Show example"}
      </button>
      {shown && (
        <div className="example-content">
          {parts.map((part, i) =>
            i % 2 === 1 && !revealed.has(i) ? (
              <button
                key={i}
                className="masked-word"
                aria-label="Reveal hidden word"
                onClick={() => setRevealed((old) => new Set([...old, i]))}
              >
                <span aria-hidden="true">••••••</span>
              </button>
            ) : (
              <span
                key={i}
                className={i % 2 === 1 ? "revealed-word" : undefined}
              >
                {part}
              </span>
            ),
          )}
        </div>
      )}
    </div>
  );
}
