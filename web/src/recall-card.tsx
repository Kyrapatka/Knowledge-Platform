import { Check, RotateCcw, Rotate3D } from "lucide-react";
import type { Presentation } from "./types";
import { algorithmNames } from "./types";
import { Markdown } from "./ui";
import { WordExample } from "./example";

export function RecallCard({
  card,
  algorithm,
  shown,
  flip,
  answer,
  busy,
  swipe,
  skipRehab,
}: {
  card: Presentation;
  algorithm: string;
  shown: boolean;
  flip: () => void;
  answer: (action: string) => void;
  busy: boolean;
  swipe: string;
  skipRehab: () => void;
}) {
  const english = algorithm.startsWith("english_");
  // Older unanswered snapshots can still carry their example in answer fields.
  const example =
    card.example ||
    (english ? card.answer.find((f) => f.key === "example")?.value : "");
  const word =
    card.foreign_word ||
    [...card.question, ...card.answer].find((f) => f.key === "foreign")
      ?.value ||
    "";
  const answerFields = card.answer.filter(
    (f) => !english || f.key !== "example",
  );
  const direction =
    card.direction ||
    (card.question[0]?.key === "native" ? "native" : "foreign");
  return (
    <article
      className={`training-card recall-card ${english ? "word-card" : ""} ${swipe ? `swipe-${swipe}` : ""}`}
    >
      <div className="recall-top">
        <span className="recall-language">
          {english
            ? shown
              ? direction === "foreign"
                ? "Native"
                : "Foreign"
              : direction === "foreign"
                ? "Foreign"
                : "Native"
            : shown
              ? "Answer"
              : "Question"}
        </span>
        <span className="recall-flip-hint">
          <Rotate3D size={15} /> Click to flip
        </span>
      </div>
      <div className={`flip-scene ${shown ? "is-flipped" : ""}`}>
        <div className="flip-inner">
          {[false, true].map((back) => (
            <div
              key={String(back)}
              className={`flip-face ${back ? "flip-back answer-fields" : "flip-front question-fields"}`}
              role="button"
              aria-label={back ? "Flip to question" : "Flip to answer"}
              aria-hidden={shown !== back}
              inert={shown !== back}
              tabIndex={busy || shown !== back ? -1 : 0}
              onClick={(e) => {
                if (
                  !busy &&
                  !(e.target as HTMLElement).closest(
                    "a, button, input, summary",
                  )
                )
                  flip();
              }}
              onKeyDown={(e) => {
                if (
                  e.target === e.currentTarget &&
                  (e.key === "Enter" || e.code === "Space")
                ) {
                  e.preventDefault();
                  e.stopPropagation();
                  if (!busy) flip();
                }
              }}
            >
              {(back ? answerFields : card.question).map((field, index) => (
                <div
                  key={`${field.key}-${index}`}
                  className={`${back ? "answer-field" : "question-field"} ${index === 0 ? "recall-lead" : "recall-detail"} ${!back && index === 0 ? "lead-question" : ""}`}
                >
                  {index > 0 && (
                    <span className="eyebrow">
                      {field.key === "answer" &&
                      answerFields.some((f) => f.key === "short_answer")
                        ? "Detailed answer"
                        : field.label}
                    </span>
                  )}
                  <Markdown value={field.value} field={field.key} />
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
      {example && <WordExample example={example} word={word} />}
      <div className="recall-meta">
        <span>{algorithmNames[algorithm]}</span>
        <span>
          {card.kind === "rehab"
            ? card.rehab_consecutive_correct
            : card.consecutive_correct}{" "}
          / {card.required_correct} correct
        </span>
      </div>
      <div className="answer-buttons">
        <button
          className="button answer-wrong"
          disabled={busy}
          onClick={() => answer("wrong")}
        >
          <RotateCcw size={22} />
          Wrong
        </button>
        <button
          className="button answer-correct"
          disabled={busy}
          onClick={() => answer("correct")}
        >
          <Check size={24} />
          Correct
        </button>
      </div>
      {(card.kind === "rehab" || card.kind === "extra") && (
        <button
          className="text-button recall-skip"
          disabled={busy}
          onClick={skipRehab}
        >
          Skip rehab
        </button>
      )}
    </article>
  );
}
