import { copyFile, mkdir, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
const require = createRequire(import.meta.url);
const assets = dirname(require.resolve("swagger-ui-dist/package.json"));
const dist = fileURLToPath(new URL("../dist/", import.meta.url));
await mkdir(join(dist, "swagger"), { recursive: true });
for (const name of ["swagger-ui.css", "swagger-ui-bundle.js", "LICENSE"]) {
  await copyFile(join(assets, name), join(dist, "swagger", name));
}
await copyFile(
  new URL("../../docs/openapi.yaml", import.meta.url),
  join(dist, "openapi.yaml"),
);
await writeFile(
  join(dist, "swagger", "index.html"),
  `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Knowledge Platform API</title><link rel="stylesheet" href="/swagger/swagger-ui.css"></head>
<body><div id="swagger-ui"></div><script src="/swagger/swagger-ui-bundle.js"></script><script src="/swagger/initializer.js"></script></body></html>
`,
);
await writeFile(
  join(dist, "swagger", "initializer.js"),
  `window.ui = SwaggerUIBundle({
  url: "/openapi.yaml", dom_id: "#swagger-ui", deepLinking: true,
  validatorUrl: null, persistAuthorization: false, withCredentials: true,
  presets: [SwaggerUIBundle.presets.apis], layout: "BaseLayout"
});\n`,
);
console.log("Bundled local Swagger UI and OpenAPI contract.");
