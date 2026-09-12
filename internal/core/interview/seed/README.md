# Dynamic Interview Graph seed

`bank.json` contains the complete 368-question bank from the supplied September 12, 2026 specification. `Raw()` exposes a copy of the embedded JSON; imports should use `(user_id, seed_key)` to remain idempotent and preserve later user edits.

| Domain | IDs | Questions |
| --- | --- | ---: |
| Go runtime and standard library | GO001–GO130 | 130 |
| SQL and PostgreSQL | DB001–DB040 | 40 |
| HTTP and networking | NET001–NET035 | 35 |
| Git | GIT001–GIT018 | 18 |
| Docker, Linux, Kubernetes, CI/CD and IaC | INF001–INF050 | 50 |
| Kafka and RabbitMQ | MQ001–MQ025 | 25 |
| gRPC and Redis | SRV001–SRV020 | 20 |
| Architecture, observability and testing | ARC001–ARC035 | 35 |
| Algorithms and security | ALG001–ALG015 | 15 |

The PDF supplies questions and F/D/S/R metadata, not model answers. Every profile is therefore a **draft**. Importing this corpus must never fabricate an answer or make a draft eligible for interview selection. An author must add an answer and explicitly mark a profile ready.

The table's frequency numbers are editorial ordinal estimates, not measured market percentages. Per-question confidence is absent from the source table; `frequency_confidence: 0.5` is an uncalibrated default. Question strings, metadata and original primary/hooks were extracted from table cell coordinates, including text beyond the PDF's visible page boundary. All 368 question strings were cross-checked against independent plain-text extraction, ignoring line breaks. `source_page`, `source_primary` and `source_hooks` preserve provenance.

Question links keep `primary`, `tested` and `hook` distinct. Each primary is also tested. Additional tested links capture what compound questions explicitly ask about: for example, GO002 tests `goroutine` and `os_thread`, while `go_scheduler` remains a fallback hook. Hooks are not fixed child-question pointers.

Concept records contain `slug`, `display_name`, `domain`, `topic` and structured `aliases`. Each alias has `alias`, `language`, `weight`, `whole_word` and `constraints`. Russian technical transliterations complement English aliases. Bare ambiguous aliases such as `commit`, `index`, `context` and `stream` require domain or surrounding-text evidence. Canonicalization keeps Go/Kubernetes/OS schedulers, Git/database indexes, Git/database/Kafka commits and Kafka/network partitions distinct. All source concept-edge relation names are preserved in `edges` with `from_slug`, `to_slug`, `relation` and `weight`.

`seed_test.go` validates corpus completeness, ID ranges, metadata bounds, concept references, ambiguity constraints and the absence of ready or answered seed questions. It uses no database.
