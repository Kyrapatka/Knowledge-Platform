# OpenAPI and Swagger

[openapi.yaml](openapi.yaml) is the documentation source of truth for authentication,
browser auth, folders/import/workshop, materials, training/session/undo/recovery,
Mock/Deep Interview, concepts/profiles, library, statistics and operational routes.
It is maintained separately from handlers, without Swagger annotations.

Run `npm --prefix web ci` and `npm --prefix web run build`, or `make deploy-local`.
Open [Swagger UI](http://localhost:8080/swagger/) or [the contract](http://localhost:8080/openapi.yaml).
JavaScript, CSS and license are copied from pinned `swagger-ui-dist` into the
frontend build. There is no CDN, inline script, external validator or persisted
authorization; existing CSP remains in force. Settings follow the
[Swagger configuration documentation](https://swagger.io/docs/open-source-tools/swagger-ui/usage/configuration/).

Use token `POST /api/v1/auth/register` or `/login`, then select **Authorize** with
the returned access_token, without a Bearer prefix. IDs and expected versions
must come from actual owned resources. Reuse command IDs only for retries of the
same uncertain command. Browser auth instead requires matching Origin and uses
the HttpOnly kp_refresh cookie, managed by the browser and Secure on HTTPS.

The spec documents request/response schemas, common errors, limits, filters and
optimistic concurrency. Dynamic material fields depend on folder configuration.
The import envelope and examples are described in [folder-import.md](folder-import.md).

`make openapi-check` validates structure, references, unique operation IDs and path
parameters. It is included in `frontend-check` and CI. Strict integration compares
every registered Gin route/method to the spec in both directions. Playwright opens
Swagger under CSP and executes `/live`. Documentation validation does not replace
runtime validation: update schemas and examples whenever DTOs change.
