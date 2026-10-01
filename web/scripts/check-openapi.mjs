import SwaggerParser from "@apidevtools/swagger-parser";
import { fileURLToPath } from "node:url";
const spec = await SwaggerParser.validate(
  fileURLToPath(new URL("../../docs/openapi.yaml", import.meta.url)),
  { resolve: { external: false } },
);
const ids = new Set();
for (const [path, item] of Object.entries(spec.paths)) {
  for (const [method, operation] of Object.entries(item)) {
    if (
      !["get", "post", "put", "patch", "delete", "head", "options"].includes(
        method,
      )
    )
      continue;
    if (!operation.operationId || ids.has(operation.operationId))
      throw new Error(`Missing or duplicate operationId: ${method} ${path}`);
    ids.add(operation.operationId);
    const expected = [...path.matchAll(/\{([^}]+)\}/g)].map((m) => m[1]).sort();
    const actual = [...(item.parameters || []), ...(operation.parameters || [])]
      .filter((p) => p.in === "path")
      .map((p) => {
        if (!p.required) throw new Error(`Optional path parameter: ${path}`);
        return p.name;
      })
      .sort();
    if (JSON.stringify(expected) !== JSON.stringify(actual))
      throw new Error(`Path parameter mismatch: ${path}`);
  }
}
console.log(
  `OpenAPI valid: ${Object.keys(spec.paths).length} paths, ${ids.size} unique operations; references resolved.`,
);
