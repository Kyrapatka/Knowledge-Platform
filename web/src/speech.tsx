import { useEffect, useId, useRef, useState } from "react";
import { Volume2, Square } from "lucide-react";
import "./speech.css";

export function PronounceButton({
  text,
  lang = "en-US",
  label = "Listen",
  className = "",
}: {
  text: string;
  lang?: string;
  label?: string;
  className?: string;
}) {
  const [speaking, setSpeaking] = useState(false);
  const [error, setError] = useState("");
  const utterance = useRef<SpeechSynthesisUtterance | null>(null);
  const timeout = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const statusID = useId();
  useEffect(() => {
    setSpeaking(false);
    setError("");
    return () => {
      clearTimeout(timeout.current);
      if (utterance.current) {
        utterance.current.onend = null;
        utterance.current.onerror = null;
        utterance.current = null;
        window.speechSynthesis?.cancel();
      }
    };
  }, [text, lang]);

  function pronounce() {
    if (
      !window.speechSynthesis ||
      typeof window.SpeechSynthesisUtterance !== "function"
    ) {
      setError("Pronunciation is not available in this browser.");
      return;
    }
    const synth = window.speechSynthesis;
    if (utterance.current) {
      utterance.current.onend = null;
      utterance.current.onerror = null;
      utterance.current = null;
      clearTimeout(timeout.current);
      synth.cancel();
      setSpeaking(false);
      return;
    }
    setError("");
    synth.cancel();
    const next = new SpeechSynthesisUtterance(text.trim());
    next.lang = lang;
    next.rate = 0.88;
    const voices = synth.getVoices();
    const voice =
      voices.find((v) => v.lang.toLowerCase() === lang.toLowerCase()) ||
      voices.find((v) =>
        v.lang.toLowerCase().startsWith(lang.split("-")[0].toLowerCase()),
      );
    if (voice) next.voice = voice;
    const finish = (message = "") => {
      if (utterance.current !== next) return;
      clearTimeout(timeout.current);
      utterance.current = null;
      setSpeaking(false);
      if (message) setError(message);
    };
    next.onend = () => finish();
    next.onerror = (event) =>
      finish(
        event.error === "interrupted" || event.error === "canceled"
          ? ""
          : "Could not play pronunciation. Check your device’s voice settings and try again.",
      );
    utterance.current = next;
    setSpeaking(true);
    try {
      synth.speak(next);
      timeout.current = setTimeout(() => {
        if (utterance.current !== next) return;
        finish("Pronunciation did not finish. Please try again.");
        synth.cancel();
      }, 30000);
    } catch {
      finish("Could not play pronunciation. Please try again.");
    }
  }

  return (
    <span className={`pronunciation ${className}`}>
      <button
        type="button"
        className={`pronounce-button ${speaking ? "is-speaking" : ""}`}
        disabled={!text.trim()}
        onClick={(event) => {
          event.stopPropagation();
          pronounce();
        }}
        aria-label={speaking ? "Stop pronunciation" : "Listen to pronunciation"}
        aria-describedby={error ? statusID : undefined}
        aria-pressed={speaking}
      >
        {speaking ? <Square size={15} /> : <Volume2 size={18} />}
        <span>{speaking ? "Playing" : label}</span>
      </button>
      <span id={statusID} role="status" className="pronunciation-status">
        {error}
      </span>
    </span>
  );
}
