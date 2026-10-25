# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: quick-import-ux.spec.ts >> redesign offers working steps, filters, undo and a pinned action
- Location: web\tests\quick-import-ux.spec.ts:4:1

# Error details

```
Error: expect(locator).toBeVisible() failed

Locator: getByRole('button', { name: 'Undo', exact: true })
Expected: visible
Timeout: 10000ms
Error: element(s) not found

Call log:
  - Expect "toBeVisible" getByRole('button', { name: 'Undo', exact: true }) with timeout 10000ms
  - waiting for getByRole('button', { name: 'Undo', exact: true })

```

```yaml
- complementary:
  - link "Knowledge home":
    - /url: /
    - text: knowledge.
  - text: WORKSPACE
  - navigation "Main navigation":
    - link "My library":
      - /url: /
      - img
      - text: My library 1
    - link "Training":
      - /url: /training
      - img
      - text: Training
    - link "Statistics":
      - /url: /statistics
      - img
      - text: Statistics
    - link "Mock interview":
      - /url: /interview
      - img
      - text: Mock interview
  - text: ✧
  - paragraph:
    - text: A little practice.
    - strong: A lasting difference.
  - text: IM
  - strong: Import test
  - text: Personal workspace
  - button "Sign out":
    - img
- banner:
  - text: Workspace
  - img
  - text: My library Sun, Oct 25
  - button "Open profile": I
- main:
  - link "← Back to library":
    - /url: /
  - img
  - text: Languages
  - heading "Import destination" [level=1]
  - paragraph: Make this knowledge your own.
  - button "Customize"
  - button "Edit folder":
    - img
    - text: Edit folder
  - button "Delete folder":
    - img
    - text: Delete folder
  - button "Settings":
    - img
    - text: Settings
  - button "Train folder":
    - img
    - text: Train folder
  - img
  - text: 1 materials
  - img
  - text: 0 ready to review
  - img
  - text: 0 in progress
  - heading "Materials" [level=2]
  - text: "1"
  - button "Import"
  - button "Add material":
    - img
    - text: Add material
  - img
  - textbox "Search materials":
    - /placeholder: Search questions and answers…
  - img
  - combobox "Filter by topic":
    - option "All topics" [selected]
  - img
  - combobox "Sort materials":
    - option "Newest first" [selected]
    - option "Question"
    - option "Topic"
    - option "Next review"
    - option "Stage"
  - button "Select all 1 shown"
  - text: MATERIAL / QUESTION TOPIC NEXT REVIEW STAGE
  - checkbox "Select existing"
  - button "existing medium difficulty No topic Not started —":
    - text: existing medium difficulty No topic Not started —
    - img
  - text: 1–1 of 1 materials
  - button "Previous" [disabled]
  - button "Next" [disabled]
  - dialog "Import materials":
    - heading "Import materials" [level=2]
    - paragraph: Import destination
    - button "Close dialog":
      - img
    - navigation "Import steps": 1 Input 2 Review 3 Result
    - status: Local draft saved · 7-day recovery
    - text: 2 cards 0 examples 1 errors 0 warnings 0 batch duplicates Unchecked existing duplicates
    - group:
      - button "Edit source"
      - button "Revalidate"
      - text: Show
      - combobox "Review filter":
        - option "All" [selected]
        - option "Issues"
        - option "Ready"
      - complementary:
        - list:
          - listitem:
            - button "subtle needs_review":
              - strong: subtle
              - text: needs_review
          - listitem:
            - button "book ready":
              - strong: book
              - text: ready
        - button "Previous cards" [disabled]
        - text: 1/1
        - button "Next cards" [disabled]
      - region "Card editor":
        - heading "subtle" [level=3]
        - button "Go to source"
        - button "Exclude card"
        - text: Foreign *
        - textbox "foreign": subtle
        - button "Source line 1"
        - text: Native *
        - textbox "native": тонкий
        - button "Source line 1"
        - text: Definition
        - textbox "definition"
        - text: Example Translation
        - textbox "example_translation"
        - text: Note
        - textbox "note"
        - text: Sources
        - textbox "sources"
        - text: Topic
        - textbox "Metadata topic"
        - text: Difficulty
        - combobox "Card difficulty":
          - option "Easy"
          - option "Medium" [selected]
          - option "Hard"
        - region "Examples":
          - heading "Examples · 0" [level=4]
          - button "Add example"
        - text: Duplicate decision
        - combobox "Duplicate decision":
          - option "Skip exact duplicate (default)" [selected]
          - option "Stop if duplicate exists"
          - option "Create even if duplicate exists"
        - text: Target card for merge or example reassignment
        - combobox "Target card":
          - option "Choose card" [selected]
          - option "book"
        - button "Merge into target" [disabled]
        - button "Split card"
        - paragraph: Split adds a blank card beside this one. Edit both cards to choose which text belongs to each. The source and automatic interpretation stay available.
        - article:
          - strong: unsupported_fragment
          - paragraph: Неподдерживаемый фрагмент после перевода
          - text: сатл Line 1 · Unresolved
          - button "Go to source"
          - button "Exclude fragment"
          - text: Assign to card
          - combobox "Assign i-zqyvmy-0 to card":
            - option "subtle" [selected]
            - option "book"
          - text: Field
          - combobox "Field for i-zqyvmy-0":
            - option "Choose field"
            - option "Foreign"
            - option "Native"
            - option "Definition"
            - option "Example" [selected]
            - option "Example Translation"
            - option "Note"
            - option "Sources"
          - button "Assign fragment"
        - group: Automatic interpretation
      - group: Edit history · 0
      - button "Import all" [disabled]
      - button "Import valid only"
- contentinfo: Make it a little more familiar. Your own pace. Your own progress.
```

# Test source

```ts
  1  | import { test, expect } from "@playwright/test";
  2  | import { fixture } from "./quick-import.spec";
  3  | 
  4  | test("redesign offers working steps, filters, undo and a pinned action", async ({ page }) => {
  5  |   await fixture(page);
  6  |   await page.getByLabel("Import source").fill("subtle - тонкий - сатл\nbook - книга");
  7  |   await page.getByRole("button", { name: "Review import", exact: true }).click();
> 8  |   await expect(page.getByRole("button", { name: "Undo", exact: true })).toBeVisible();
     |                                                                         ^ Error: expect(locator).toBeVisible() failed
  9  |   await expect(page.getByRole("button", { name: "Issues 1", exact: true })).toBeVisible();
  10 |   await expect(page.getByRole("button", { name: "1 Input", exact: true })).toBeEnabled();
  11 |   await expect(page.getByRole("button", { name: "Revalidate", exact: true })).toHaveCount(0);
  12 |   await expect(page.getByText("Advanced", { exact: true })).toBeVisible();
  13 |   await expect(page.getByLabel("Difficulty", { exact: true })).not.toBeVisible();
  14 |   await expect(page.getByTestId("import-footer")).toBeVisible();
  15 | });
  16 | 
```