import { parseDraft } from "../model/draft";
import type { ImportFolder } from "../model/types";
self.onmessage = (
  event: MessageEvent<{
    source: string;
    kind: "text" | "json";
    folder: ImportFolder;
  }>,
) => {
  try {
    self.postMessage({
      draft: parseDraft(event.data.source, event.data.kind, event.data.folder),
    });
  } catch (error) {
    self.postMessage({
      error: error instanceof Error ? error.message : "Unable to parse source",
    });
  }
};
