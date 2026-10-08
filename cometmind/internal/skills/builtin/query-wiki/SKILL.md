---
name: query-wiki
description: Look up the user's existing LLM wiki before answering a topic they may already have learned. Read @runtime/wiki/index.md first, then open only the relevant pages. Use when a question might already be answered in the wiki. Do not use for ingest, lint, or saving new research.
---

# Query wiki

Read the user's wiki. Do not write to it.

1. `read_file` `@runtime/wiki/index.md`.
2. If it is missing, tell the user the wiki has not been created. Stop. Do not create directories or call `llm-wiki`.
3. Choose the relevant pages from the catalog. Read at most 3 page files with `read_file`.
4. Answer from those pages and cite their `@runtime/wiki/...` paths.
5. Do not write or update wiki files. If the answer should be saved, tell the user to use `llm-wiki`.
6. Do not read `raw/` unless a compiled page cites a source the user explicitly asks to see.
