export type ImportTemplate = "interview_questions" | "english_words";

export const importExamples: Record<ImportTemplate, object> = {
  interview_questions: {
    format_version: 1,
    folder: {
      name: "Go Interview",
      description: "Questions for Go interview preparation",
      template: "interview_questions",
    },
    items: [
      {
        question: "Что такое goroutine?",
        short_answer:
          "Goroutine — легковесная единица конкурентного выполнения под управлением runtime Go.",
        full_answer:
          "Goroutine выполняется планировщиком Go поверх ограниченного числа потоков операционной системы.",
        source: ".",
        topic: "Concurrency",
        subtopic: "Goroutines",
        difficulty: 2,
        frequency: 10,
        level: "middle",
        company: "",
        keywords: ["goroutine", "runtime"],
        concepts: ["goroutine", "scheduler"],
      },
    ],
  },
  english_words: {
    format_version: 1,
    folder: {
      name: "English B2",
      description: "English vocabulary for work",
      template: "english_words",
    },
    items: [
      {
        word: "maintain",
        translation: "поддерживать",
        pronunciation: "/meɪnˈteɪn/",
        definition: "To keep something in good condition or continue it.",
        example: "We need to maintain the system.",
        example_translation: "Нам нужно поддерживать систему.",
        notes: "",
        topic: "Work",
        sources: [],
      },
    ],
  },
};

export const exampleJSON = (template: ImportTemplate) =>
  JSON.stringify(importExamples[template], null, 2);

export const exampleFileNames: Record<ImportTemplate, string> = {
  interview_questions: "knowledge-platform-interview-example.json",
  english_words: "knowledge-platform-english-example.json",
};

export const importPrompts: Record<ImportTemplate, string> = {
  interview_questions: `Create a JSON file for Knowledge Platform.

I need an interview-question folder about:

[INSERT TOPIC HERE]

Return ONLY valid JSON.
Do not use Markdown.
Do not wrap the result in \`\`\`json.

Use exactly this top-level structure:

{
  "format_version": 1,
  "folder": {
    "name": "...",
    "description": "...",
    "template": "interview_questions"
  },
  "items": []
}

Each item must follow:

{
  "question": "...",
  "short_answer": "...",
  "full_answer": "...",
  "source": ".",
  "topic": "...",
  "subtopic": "...",
  "difficulty": 1,
  "frequency": 1,
  "level": "...",
  "company": "",
  "keywords": [],
  "concepts": []
}

Rules:

- question and short_answer are required;
- question must be a real interview question;
- short_answer must directly answer the question in 1–3 sentences;
- short_answer must never be empty or ".";
- full_answer should contain a detailed explanation and may be empty;
- source may be "." if no verified source is available;
- difficulty, when present, must be an integer from 1 to 5;
- frequency, when present, must be an integer from 1 to 10;
- keywords must contain important terms from the question;
- concepts must contain canonical concepts and useful synonyms or related terms;
- do not create duplicate questions;
- use correct technical terminology;
- output valid JSON only.

Generate [NUMBER] questions.`,
  english_words: `Create a JSON file for Knowledge Platform.

I want to learn English vocabulary about:

[INSERT TOPIC]

My approximate English level:

[LEVEL]

Generate:

[NUMBER] words.

Return ONLY valid JSON.
Do not use Markdown.
Do not wrap the response in \`\`\`json.

Use exactly this top-level structure:

{
  "format_version": 1,
  "folder": {
    "name": "...",
    "description": "...",
    "template": "english_words"
  },
  "items": []
}

Each item must follow:

{
  "word": "...",
  "translation": "...",
  "pronunciation": "...",
  "definition": "...",
  "example": "...",
  "example_translation": "...",
  "notes": "",
  "topic": "...",
  "sources": []
}

Rules:

- word and translation are required;
- use useful vocabulary appropriate for the requested level;
- avoid duplicate words;
- translation must be natural Russian;
- pronunciation should use IPA where possible;
- definition should be concise;
- example should sound natural;
- example_translation should accurately match the example;
- sources must be an array of URLs or source names;
- output valid JSON only.`,
};
