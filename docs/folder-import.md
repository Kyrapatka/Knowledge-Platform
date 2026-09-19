# Folder import

Knowledge Platform accepts a single versioned JSON envelope for user-created folder imports. Imports use the normal folder, material, and interview-profile domain services; imported content is available in the library, editors, and training immediately.

## API

- `POST /api/v1/folders/import/validate` validates a multipart upload without writing to the database.
- `POST /api/v1/folders/import` validates and imports the same multipart upload atomically.
- Both endpoints expect one `.json` file in the `file` form field and require authentication.

A successful import returns `folder_id`, the final duplicate-safe `folder_name`, `template`, `items_created`, and `warnings`.

## Envelope and version

Every file uses this structure:

```json
{
  "format_version": 1,
  "folder": {
    "name": "Folder name",
    "description": "Optional description",
    "template": "interview_questions"
  },
  "items": []
}
```

Only `format_version: 1` is supported. An unknown version fails validation with no database writes. Folder names are required and limited to 200 characters; descriptions are limited to 2,000 characters.

## Interview questions

Use `folder.template: "interview_questions"`. Each item requires:

- `question`: non-empty string;
- `short_answer`: non-empty real answer (not `"."`).

Optional fields are `full_answer`, `source`, `topic`, `subtopic`, `difficulty`, `frequency`, `level`, `company`, `keywords`, and `concepts`. `difficulty` is an integer from 1–5; `frequency` is an integer from 1–10. `keywords` and `concepts` are arrays of unique non-empty strings.

During import, `full_answer` maps to the existing detailed `answer` value and `source` maps to `sources`. Difficulty and frequency populate the normal interview profile. Concepts populate the interview graph; a safe fallback concept makes the minimal two-field item trainable immediately. Imported questions are saved as ready interview profiles.

Complete example: [interview_questions.json](import/examples/interview_questions.json).

## English words

Use `folder.template: "english_words"`. Each item requires:

- `word`: non-empty string;
- `translation`: non-empty string.

Optional fields are `pronunciation`, `definition`, `example`, `example_translation`, `notes`, `topic`, and `sources`. Sources are an array of unique non-empty strings.

The importer adapts these names to the existing English material schema: `word → foreign`, `translation → native`, `pronunciation → transcription`, and `notes → note`. Sources are preserved as newline-separated source entries in the existing `sources` value.

Complete example: [english_words.json](import/examples/english_words.json).

## Validation and limits

- maximum file size: 10 MB;
- maximum items: 5,000;
- valid UTF-8 and exactly one JSON value are required;
- unknown templates, unknown fields, empty item arrays, invalid ranges, excessive field lengths, and duplicate questions or words are rejected;
- item errors use one-based item numbers and field-specific messages;
- any validation or persistence failure leaves both the folder and all materials uncommitted.

Folder names are scoped to the authenticated user. If an active folder already has the requested name, the importer chooses `Name (2)`, then `Name (3)`, and so on. Existing folders are never overwritten.

## AI generation

The Import Folder dialog contains separate, copyable AI prompts for Interview Questions and English Words. Each prompt embeds the version, exact envelope, item fields, required fields, and real numeric ranges above. The dialog also displays and downloads the same examples stored in this repository as `knowledge-platform-interview-example.json` and `knowledge-platform-english-example.json`.
