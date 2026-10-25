/** Standalone deterministic Quick Import prototype. No dependency on the app. */
export type Issue = {
  code: string;
  severity: "warning" | "error";
  line: number;
  fragment: string;
  message: string;
};
export type Card = {
  id: string;
  word: string;
  translation: string;
  examples: string[];
  exampleLines?: number[];
  startLine: number;
  endLine: number;
  issues: Issue[];
};
export type Alternative = {
  cardId: string;
  word: string;
  score: number;
  reasons: string[];
};
export type ReviewCandidate = {
  line: number;
  text: string;
  alternatives: Alternative[];
  reason: string;
};
export type ParseHypothesis = {
  evidenceScore: number;
  assignments: { line: number; cardId?: string }[];
};
export type ParseResult = {
  cards: Card[];
  issues: Issue[];
  unassigned: { line: number; text: string }[];
  reviews: ReviewCandidate[];
  hypotheses: ParseHypothesis[];
  lines: number;
};
type Row = { line: number; text: string; block: number };
const cyr = /[\u0400-\u052f]/u;
const lat = /[A-Za-z]/;
const named =
  /^(word|term|слово|translation|перевод|meaning|example|пример)\s*:\s*(.*)$/i;
const beginsExample = /^(example|пример)\s*:\s*(.*)$/i;
const unsupportedLabel =
  /^(?:part of speech|pos|часть речи|transcription|транскрипция|pronunciation|произношение|note|заметка)\s*:/i;
const brokenPair = /^(?:[-–—]\s*.+|.+\s+[-–—:=]\s*)$/;
const normalize = (x: string) => x.trim().replace(/\s+/g, " ");
const canonical = (x: string) =>
  normalize(x)
    .toLowerCase()
    .replace(/[.!?,;:]+$/, "");
const tokenize = (x: string) =>
  canonical(x)
    .split(/[^\p{L}\p{N}'+-]+/u)
    .filter(Boolean);
const matches = (word: string, sentence: string): boolean => {
  const w = tokenize(word),
    s = tokenize(sentence);
  if (!w.length || !s.length) return false;
  return s.some((_, i) => w.every((part, j) => s[i + j] === part));
};
function parsePair(
  line: string,
): { word: string; translation: string; extra?: string } | null {
  let s = line.replace(/^\s*(?:\d+[.)]|[-*•])\s+/, "").trim();
  if (named.test(s)) return null;
  // EN -> RU boundary is stronger evidence than a punctuation delimiter.
  // Limit the English part to a plausible headword/phrase; do not split slash-pairs.
  const cut = s.search(/[\u0400-\u052f]/u);
  if (cut > 0 && lat.test(s.slice(0, cut))) {
    let left = s
      .slice(0, cut)
      .trim()
      .replace(/\s*[-–—:=,;]\s*$/, "")
      .trim();
    let right = s.slice(cut).trim();
    if (left && right) {
      const extraMatch = right.match(/^(.*?)\s+[-–—]\s+(.+)$/);
      if (extraMatch)
        return {
          word: left,
          translation: normalize(extraMatch[1]),
          extra: normalize(extraMatch[2]),
        };
      return { word: normalize(left), translation: normalize(right) };
    }
  }
  // Explicit delimiters: allow English-only translation but mark separately in future.
  const sep = s.match(/^(.+?)\s+(?:[-–—]|=)\s+(.+)$/);
  if (sep && lat.test(sep[1]) && cyr.test(sep[2]))
    return { word: normalize(sep[1]), translation: normalize(sep[2]) };
  return null;
}
export function parseText(input: string): ParseResult {
  const source = input.replace(/\r\n?/g, "\n");
  const sourceLines = source.split("\n");
  let block = 0;
  const rows: Row[] = source
    .split("\n")
    .map((text, i) => {
      if (!text.trim()) block++;
      return { line: i + 1, text: text.trim(), block };
    })
    .filter((r) => r.text);
  const cards: Card[] = [];
  const issues: Issue[] = [];
  const unknown: Row[] = [];
  let current: Card | undefined;
  let pendingWord: Row | undefined;
  let pendingNamedWord: Row | undefined;
  function add(word: string, translation: string, r: Row) {
    const card: Card = {
      id: `card-${cards.length + 1}`,
      word,
      translation,
      examples: [],
      startLine: pendingWord?.line ?? pendingNamedWord?.line ?? r.line,
      endLine: r.line,
      issues: [],
    };
    cards.push(card);
    current = card;
    pendingWord = undefined;
    pendingNamedWord = undefined;
  }
  function issue(
    code: string,
    severity: Issue["severity"],
    r: Row,
    fragment: string,
    message: string,
    card?: Card,
  ) {
    const o = { code, severity, line: r.line, fragment, message };
    issues.push(o);
    card?.issues.push(o);
  }
  for (let ri = 0; ri < rows.length; ri++) {
    const r = rows[ri];
    const l = r.text;
    const next = rows[ri + 1];
    // A standalone English headword followed immediately by a Russian line.
    if (
      next &&
      next.line === r.line + 1 &&
      lat.test(l) &&
      !cyr.test(l) &&
      !named.test(l) &&
      !/[.!?:]$/.test(l) &&
      tokenize(l).length <= 3 &&
      cyr.test(next.text) &&
      !lat.test(next.text) &&
      !named.test(next.text)
    ) {
      add(normalize(l), normalize(next.text), next);
      current!.startLine = r.line;
      ri++;
      continue;
    }
    const explicit = l.match(named);
    if (explicit) {
      const key = explicit[1].toLowerCase(),
        value = normalize(explicit[2]);
      if (["word", "term", "слово"].includes(key)) {
        if (pendingNamedWord)
          issue(
            "missing_translation",
            "error",
            pendingNamedWord,
            pendingNamedWord.text,
            "Не найден перевод",
          );
        pendingNamedWord = r;
        pendingWord = undefined;
        if (!value) issue("missing_word", "error", r, l, "Пустое слово");
        continue;
      }
      if (["translation", "перевод", "meaning"].includes(key)) {
        if (pendingNamedWord) {
          const w = pendingNamedWord.text.replace(named, "$2").trim();
          if (w && value) add(w, value, r);
          else issue("missing_value", "error", r, l, "Неполная пара");
        } else issue("orphan_translation", "error", r, l, "Перевод без слова");
        continue;
      }
      if (["example", "пример"].includes(key)) {
        if (value) unknown.push({ ...r, text: value });
        else issue("empty_example", "error", r, l, "Пустой пример");
        continue;
      }
    }
    if (unsupportedLabel.test(l)) {
      issue(
        "unsupported_field",
        "error",
        r,
        l,
        "Поле не поддерживается текстовым импортом",
      );
      continue;
    }
    const pair = parsePair(l);
    if (pair) {
      add(pair.word, pair.translation, r);
      if (pair.extra)
        issue(
          "unsupported_fragment",
          "error",
          r,
          pair.extra,
          "Неподдерживаемый фрагмент после перевода",
          current,
        );
      continue;
    }
    if (brokenPair.test(l) && !/^[-–—]\s*.+[.!?]$/.test(l)) {
      issue(
        "incomplete_pair",
        "error",
        r,
        l,
        "В паре отсутствует слово или перевод",
      );
      continue;
    }
    if (pendingNamedWord) {
      issue(
        "incomplete_named_field",
        "error",
        pendingNamedWord,
        pendingNamedWord.text,
        "Не найден перевод к слову",
      );
      pendingNamedWord = undefined;
    }
    // One English line followed by a Russian line is a two-line card.
    if (pendingWord && cyr.test(l) && !lat.test(l)) {
      add(normalize(pendingWord.text), normalize(l), r);
      continue;
    }
    if (pendingWord) {
      unknown.push(pendingWord);
      pendingWord = undefined;
    }
    if (lat.test(l) && !cyr.test(l) && !current) {
      pendingWord = r;
      continue;
    }
    if (cyr.test(l) && !lat.test(l)) {
      issue(
        "unassigned_cyrillic",
        "warning",
        r,
        l,
        "Строка на русском языке без пары",
      );
      continue;
    }
    unknown.push(r);
  }
  if (pendingWord) unknown.push(pendingWord);
  if (pendingNamedWord)
    issue(
      "missing_translation",
      "error",
      pendingNamedWord,
      pendingNamedWord.text,
      "Не найден перевод",
    );
  if (cards.length > 1000)
    return {
      cards: [],
      issues: [
        {
          code: "item_limit",
          severity: "error",
          line: 1,
          fragment: "",
          message: "At most 1000 cards are allowed",
        },
      ],
      unassigned: [],
      reviews: [],
      hypotheses: [],
      lines: sourceLines.length,
    };
  const unassigned: ParseResult["unassigned"] = [];
  const reviews: ReviewCandidate[] = [];
  const lineBlocks = new Map(rows.map((r) => [r.line, r.block]));
  const wordTokens = new Map(cards.map((c) => [c.id, tokenize(c.word)]));
  // Candidate ranking uses lexical overlap, structural locality and sentence shape.
  // Scores are relative evidence points, NOT calibrated probabilities.
  const exactMatches = new Map<number, Card[]>();
  function rank(r: Row): Alternative[] {
    const tokens = tokenize(r.text);
    const matching: Card[] = [];
    exactMatches.set(r.line, matching);
    return cards
      .map((c) => {
        const reasons: string[] = [];
        let score = 0;
        const w = wordTokens.get(c.id)!;
        const exact =
          w.length > 0 &&
          tokens.some((_, i) => w.every((part, j) => tokens[i + j] === part));
        if (exact) matching.push(c);
        if (exact) {
          score += 65;
          reasons.push("exact_phrase_match");
        }
        const shared = wordTokens
          .get(c.id)!
          .filter((w) => tokens.includes(w)).length;
        if (shared && !exact) {
          score += Math.min(18, shared * 6);
          reasons.push("partial_token_overlap");
        }
        const distance = r.line - c.endLine;
        if (distance >= 0 && distance <= 2) {
          score += 22;
          reasons.push("adjacent_to_card");
        } else if (distance >= 0 && distance <= 5) {
          score += 10;
          reasons.push("near_card");
        }
        if (r.block === lineBlocks.get(c.startLine)) {
          score += 8;
          reasons.push("same_block");
        }
        if (tokens.length >= 5) {
          score += 8;
          reasons.push("sentence_length");
        }
        if (/[.!?…]$/.test(r.text)) {
          score += 5;
          reasons.push("sentence_punctuation");
        }
        return { cardId: c.id, word: c.word, score, reasons };
      })
      .sort((a, b) => b.score - a.score || a.cardId.localeCompare(b.cardId));
  }
  const rankings = new Map(unknown.map((r) => [r.line, rank(r).slice(0, 5)]));
  // Bounded beam over possible example assignments. Includes an unassigned path.
  // Automatic attachment remains conservative, to avoid silently corrupting cards.
  type Assignment = { line: number; cardId?: string };
  type Link = { decision: Assignment; previous?: Link };
  type Beam = { score: number; last?: Link };
  let beams: Beam[] = [{ score: 0 }];
  for (const r of unknown) {
    const choices = rankings.get(r.line)!.slice(0, 4);
    const next: Beam[] = [];
    for (const beam of beams) {
      next.push({
        score: beam.score + 22,
        last: { decision: { line: r.line }, previous: beam.last },
      });
      for (const option of choices)
        next.push({
          score: beam.score + option.score,
          last: {
            decision: { line: r.line, cardId: option.cardId },
            previous: beam.last,
          },
        });
    }
    beams = next.sort((a, b) => b.score - a.score).slice(0, 8);
  }
  const decisions = (beam: Beam) => {
    const result: Assignment[] = [];
    for (let link = beam.last; link; link = link.previous)
      result.push(link.decision);
    return result.reverse();
  };
  for (const r of unknown) {
    if (!cards.length) {
      unassigned.push({ line: r.line, text: r.text });
      issue(
        "unassigned",
        "warning",
        r,
        r.text,
        "Не найдена карточка для строки",
      );
      continue;
    }
    const explicitExample = beginsExample.test(
      sourceLines[r.line - 1]?.trim() || "",
    );
    const matched = exactMatches.get(r.line) ?? [];
    const alternatives = rankings.get(r.line)!;
    const local = cards.filter((c) => c.startLine <= r.line).at(-1);
    let destination: Card | undefined;
    // Explicit example labels and close local structure override global lexical matches.
    if (explicitExample && local) destination = local;
    else if (matched.length === 1) destination = matched[0];
    else if (
      matched.length === 0 &&
      local &&
      r.line - local.endLine <= 3 &&
      tokenize(r.text).length >= 5 &&
      /[.!?…]$/.test(r.text)
    )
      destination = local;
    if (destination) {
      destination.examples.push(r.text);
      (destination.exampleLines ??= []).push(r.line);
      destination.endLine = Math.max(destination.endLine, r.line);
    } else {
      unassigned.push({ line: r.line, text: r.text });
      reviews.push({
        line: r.line,
        text: r.text,
        alternatives: alternatives.slice(0, 5),
        reason:
          matched.length > 1
            ? "multiple_lexical_matches"
            : "insufficient_evidence",
      });
      issue(
        matched.length > 1 ? "ambiguous_example" : "unassigned_example",
        "warning",
        r,
        r.text,
        matched.length > 1
          ? "Пример подходит нескольким карточкам"
          : "Не удалось надёжно привязать пример",
      );
    }
  }
  const seen = new Map<string, Card>();
  for (const card of cards) {
    const key = canonical(card.word);
    if (seen.has(key)) {
      issue(
        "duplicate_word",
        "warning",
        { line: card.startLine, text: card.word, block: 0 },
        card.word,
        "Повтор слова: проверьте, объединить ли карточки",
        card,
      );
    } else seen.set(key, card);
  }
  return {
    cards,
    issues,
    unassigned,
    reviews,
    hypotheses: beams
      .slice(0, 3)
      .map((b) => ({ evidenceScore: b.score, assignments: decisions(b) })),
    lines: source.split("\n").length,
  };
}
