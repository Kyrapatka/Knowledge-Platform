import type { Range } from "../model/types.js";
export class JSONIssue extends Error {
  constructor(
    public code: string,
    public path: string,
    public range: Range,
    message: string,
  ) {
    super(message);
  }
}
// Token walk rejects duplicate object keys before JSON.parse can discard them.
export function strictJSON(source: string): {
  value: unknown;
  ranges: Map<string, Range>;
} {
  let p = 0;
  const ranges = new Map<string, Range>();
  const newlinePositions: number[] = [];
  for (let i = 0; i < source.length; i++)
    if (source[i] === "\n") newlinePositions.push(i);
  const range = (start: number, end = p): Range => {
    let lo = 0,
      hi = newlinePositions.length;
    while (lo < hi) {
      const mid = (lo + hi) >>> 1;
      if (newlinePositions[mid] < start) lo = mid + 1;
      else hi = mid;
    }
    return { start, end, line: lo + 1 };
  };
  const fail = (
    code: string,
    path: string,
    start: number,
    message: string,
  ): never => {
    throw new JSONIssue(
      code,
      path,
      range(start, Math.max(p, start + 1)),
      message,
    );
  };
  const ws = () => {
    while (/[ \r\n\t]/.test(source[p] ?? "x")) p++;
  };
  function string(path: string): string {
    const start = p++;
    while (p < source.length) {
      if (source[p] === "\\") {
        p += 2;
        continue;
      }
      if (source[p++] === '"') {
        try {
          return JSON.parse(source.slice(start, p)) as string;
        } catch {
          return fail("invalid_json", path, start, "Invalid JSON string");
        }
      }
    }
    return fail("invalid_json", path, start, "Unterminated JSON string");
  }
  function value(path: string, depth: number): unknown {
    ws();
    const start = p;
    if (depth > 32) fail("json_depth", path, start, "JSON nesting exceeds 32");
    let result: unknown;
    if (source[p] === "{") {
      p++;
      ws();
      const obj: Record<string, unknown> = Object.create(null),
        seen = new Set<string>();
      if (source[p] !== "}")
        while (true) {
          ws();
          const keyStart = p;
          if (source[p] !== '"')
            fail("invalid_json", path, p, "Expected object key");
          const key = string(path);
          const child = path ? `${path}.${key}` : key;
          if (seen.has(key))
            fail("duplicate_json_key", child, keyStart, "Duplicate JSON key");
          seen.add(key);
          ws();
          if (source[p++] !== ":")
            fail("invalid_json", child, p - 1, "Expected colon");
          obj[key] = value(child, depth + 1);
          ws();
          if (source[p] !== ",") break;
          p++;
        }
      if (source[p++] !== "}")
        fail("invalid_json", path, p - 1, "Expected object end");
      result = obj;
    } else if (source[p] === "[") {
      p++;
      ws();
      const arr: unknown[] = [];
      if (source[p] !== "]")
        while (true) {
          arr.push(value(`${path}[${arr.length}]`, depth + 1));
          ws();
          if (source[p] !== ",") break;
          p++;
        }
      if (source[p++] !== "]")
        fail("invalid_json", path, p - 1, "Expected array end");
      result = arr;
    } else if (source[p] === '"') result = string(path);
    else {
      const m = source
        .slice(p)
        .match(
          /^(?:true|false|null|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?)/,
        );
      if (!m) fail("invalid_json", path, p, "Invalid JSON value");
      p += m![0].length;
      result = JSON.parse(m![0]);
    }
    ranges.set(path, range(start));
    return result;
  }
  const parsed = value("", 0);
  ws();
  if (p !== source.length) fail("invalid_json", "", p, "Trailing JSON content");
  return { value: parsed, ranges };
}
