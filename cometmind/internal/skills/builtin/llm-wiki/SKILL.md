---
name: llm-wiki
description: Build and maintain a persistent personal knowledge wiki at @runtime/wiki using the Karpathy LLM Wiki pattern (ingest, query, lint).
---

# LLM Wiki

Use this skill to build and maintain a **persistent, compounding knowledge wiki** at `@runtime/wiki/` (`~/.cometmind/wiki/` on disk). This follows the [Karpathy LLM Wiki](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) pattern: raw sources are immutable; compiled pages are LLM-maintained synthesis with cross-references.

## When to use

- Research accumulation over days or weeks
- Ingest URLs, articles, notes, or pasted content
- Query synthesized knowledge (not just chat history)
- Periodic wiki health checks (lint)
- `/llm-wiki`, `/llm-wiki ingest …`, `/llm-wiki lint`, `/llm-wiki research {topic}`

## Prerequisites

- `@runtime/wiki` mount is available (read/write, not age-purged)
- On first use, run **Scaffold** below before ingest/query/lint
- CometMind memory stores atomic facts; this wiki stores **synthesized knowledge graphs**

## Paths

Flat layout under `@runtime/wiki/` (no nested `wiki/wiki/`):

| Alias | Disk path | Role |
|-------|-----------|------|
| `@runtime/wiki/` | `~/.cometmind/wiki/` | Wiki root |
| `@runtime/wiki/index.md` | `…/index.md` | Default catalog — start query/ingest here |
| `@runtime/wiki/log.md` | `…/log.md` | Append-only activity timeline |
| `@runtime/wiki/raw/` | `…/raw/` | Immutable sources (never edit after capture) |
| `@runtime/wiki/entities/` | `…/entities/` | People, orgs, products |
| `@runtime/wiki/concepts/` | `…/concepts/` | Ideas and frameworks |
| `@runtime/wiki/syntheses/` | `…/syntheses/` | Cross-topic write-ups |
| `@runtime/wiki/WIKI.md` | `…/WIKI.md` | Schema and conventions |

Browse with `/llm-wiki query`, any text editor, or Finder at `~/.cometmind/wiki/`.

## Scaffold (run first)

If `@runtime/wiki/WIKI.md` is missing, create the full skeleton:

1. `write_file` directories via placeholder files or `write_file` to paths that create parents:
   - `@runtime/wiki/raw/assets/.gitkeep` (empty or minimal)
   - `@runtime/wiki/entities/.gitkeep`
   - `@runtime/wiki/concepts/.gitkeep`
   - `@runtime/wiki/syntheses/.gitkeep`
2. `write_file` `@runtime/wiki/index.md`:

```markdown
# Wiki Index

Catalog of all wiki pages. Updated on every ingest.

## Entities

## Concepts

## Syntheses

## Sources
```

3. `write_file` `@runtime/wiki/log.md`:

```markdown
# Wiki Log

Append-only timeline of ingests, queries, and lint passes.
```

4. `write_file` `@runtime/wiki/WIKI.md` with the schema template below (full content in **WIKI.md schema template** section).

5. Optionally offer a **weekly lint scheduled job**: if scheduler is enabled and no job named "LLM Wiki lint" exists, use `create_scheduled_job` with `cron_expr: "0 9 * * 0"` (Sundays 09:00) and `definition_of_done: "Run /llm-wiki lint on the global wiki at @runtime/wiki. Report auto-fixed items and needs-review items."`

## Ingest (capture → compile)

Two steps — never skip capture:

### Capture

Obtain content and save to `raw/` **without modifying it later**:

| Source | How |
|--------|-----|
| URL | `web_fetch` → `write_file` to `@runtime/wiki/raw/{YYYY-MM-DD}-{slug}.md` |
| Search | `web_search` → `web_fetch` each selected URL → save each to `raw/` |
| Pasted text | `write_file` to `raw/{YYYY-MM-DD}-{slug}.md` |
| Existing raw file | Skip capture; go to compile |

Raw file frontmatter (YAML):

```yaml
---
title: Article title
source_url: https://…
captured_at: 2026-04-02
---
```

### Compile

1. Read the raw file and relevant existing wiki pages (start from `@runtime/wiki/index.md`)
2. Discuss key takeaways with the user when appropriate
3. Write or update wiki pages under `entities/`, `concepts/`, `syntheses/`
4. Use `[[wikilink]]` cross-references between pages
5. Update `@runtime/wiki/index.md` (catalog with one-line summaries)
6. Append to `@runtime/wiki/log.md`: `## [YYYY-MM-DD] ingest | {title}`
7. **Prefer updating existing pages** over creating duplicates; one raw source may touch 10–15 wiki pages

**Do not edit files under `raw/` during compile.**

## Query

1. Read `@runtime/wiki/index.md` to locate relevant pages
2. `read_file` the most relevant pages under `entities/`, `concepts/`, or `syntheses/`
3. Synthesize an answer with citations to wiki paths
4. **File good answers back into the wiki** as new pages under `syntheses/` or update existing pages — explorations should compound like ingests

## Lint (scan → analyze → report → fix)

Run when the user asks `/llm-wiki lint` or after many ingests without a recent lint.

### Phase 1 — Scan (mechanical)

| Check | How |
|-------|-----|
| Orphan pages | `glob` `entities/**/*.md`, `concepts/**/*.md`, `syntheses/**/*.md` → `grep` each basename for `[[…]]` inbound links |
| Index drift | Compare `index.md` entries vs actual files |
| Broken links | `grep` `[[…]]` → verify targets exist |
| Stale pages | Compare page `updated:` frontmatter vs `log.md` |
| Uncompiled raw | `glob raw/*.md` vs ingest entries in `log.md` |

### Phase 2 — Analyze (semantic)

Read related pages and check for:

- **Contradictions** between pages on the same topic
- **Stale claims** superseded by newer raw sources
- **Duplicates** that should merge
- **Gaps** — concepts mentioned but lacking dedicated pages
- **Memory candidates** — stable atomic facts better suited to CometMind memory

### Phase 3 — Report + fix

Produce a lint report in chat and optionally `write_file` `@runtime/wiki/lint-report.md`.

**Auto-fix (no confirmation needed):**

- Fix broken wikilinks and missing cross-refs
- Sync `index.md`
- Merge obvious duplicates (high confidence only)
- Add `> **STALE**` or `> **CONTRADICTS** [[page]]` banners

**Ask user first:**

- Semantic contradictions (both views may be valid)
- Deleting pages
- Major synthesis rewrites

Append to `log.md`: `## [YYYY-MM-DD] lint | N fixed, M needs review`

After 5+ ingests without lint, remind the user to run `/llm-wiki lint`.

## Raw naming

`raw/{YYYY-MM-DD}-{slug}.md` where slug is lowercase hyphenated from title.

## Wiki page format

```yaml
---
title: Page title
tags: [concept, proactive-agents]
updated: 2026-04-02
sources:
  - raw/2026-04-02-example.md
---
```

Body uses `[[wikilink]]` for cross-references. Entity pages → `entities/`, concepts → `concepts/`, broad syntheses → `syntheses/`.

## Memory boundary

| Store | Content |
|-------|---------|
| LLM Wiki | Synthesized research, evolving theses, cross-linked knowledge |
| CometMind memory | Atomic user preferences and stable facts |

During lint, flag facts that should move to memory; do not duplicate across both stores.

## WIKI.md schema template

When scaffolding, write this to `@runtime/wiki/WIKI.md`:

```markdown
# LLM Wiki Schema

Persistent personal knowledge base at `@runtime/wiki/` (~/.cometmind/wiki/).

## Layout

- **index.md** — default catalog; start every query here
- **log.md** — append-only ingest/query/lint timeline
- **raw/** — immutable sources; never edit after capture
- **entities/**, **concepts/**, **syntheses/** — compiled markdown pages
- **WIKI.md** — this file (conventions)

## Operations

- **ingest** — capture → compile
- **query** — read `index.md` first; file good answers back
- **lint** — scan → analyze → report + conservative auto-fix

## Backup

Enable automatic backup in Settings → CometMind → Storage to zip `~/.cometmind/` to a local folder. Protects wiki, database, and settings.

## Lint auto-fix policy

Auto-fix: broken links, index sync, obvious duplicate merge, STALE/CONTRADICTS banners.
Ask first: deletions, semantic contradictions, major synthesis rewrites.
```

## Research workflow

`/llm-wiki research {topic}`:

1. Scaffold if needed
2. `web_search` for the topic
3. Select 3–5 high-quality sources
4. Ingest each (capture → compile)
5. Write or update a synthesis page under `syntheses/`
6. Summarize what was added and suggest `/llm-wiki lint` when appropriate
