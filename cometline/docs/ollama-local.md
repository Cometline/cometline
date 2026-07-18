# Run Cometline locally with Ollama

Cometline can use a user-installed [Ollama](https://ollama.com) daemon for private chat/title models and memory embeddings. Cometline does **not** bundle or silently install Ollama.

## Quick start

1. Install Ollama from the [official download page](https://ollama.com/download) and launch it once.
2. In Cometline → Settings → Providers, open **Ollama Local** (or pick it in the setup wizard).
3. Click **Check again** until the daemon at `http://127.0.0.1:11434` is healthy.
4. Pull a recommended model:
   - **Private Memory** — `qwen3-embedding:0.6b` (~639 MB) for memory embeddings
   - **Local Companion** — Gemma MLX chat models for chat/titles on Apple Silicon
5. Enable the installed model, then assign roles under Settings → Model Roles / Memory.

The setup wizard recommends **Private Memory** first so you can use local embeddings without a cloud API key.

## Privacy by role

Each role can point at a different provider:

| Role | Typical local option | Notes |
|------|----------------------|--------|
| Chat / default model | Ollama Gemma companion | Chat and titles in v1; not advertised as agent-ready until validated |
| Title generation | Same local chat model or hosted | Local Ollama providers are labeled Local in Model Roles |
| Memory extraction | Usually hosted | Local extraction is not catalogued in v1 |
| Memory embeddings | Ollama `qwen3-embedding:0.6b` | Uses CometMind’s native Ollama embedder |

Cometline never silently falls back from a local role to a cloud provider. If Ollama is stopped or a model is missing, that role fails with a repairable error.

## Disk and RAM

- Embedding pull is relatively small (~0.6 GB).
- Gemma companion pulls are multi‑GB; confirm free disk before pulling.
- To free space, remove models in Ollama itself (Cometline does not delete shared model files other apps may use).

## Switching embedding models

Changing the embedding model starts a cancellable background re-embed when existing memories use a different vector space. Retrieval keeps the previous index until the new one is complete.

## Troubleshooting

| Symptom | What to try |
|---------|-------------|
| “Ollama is not installed or not running” | Install/launch Ollama, then Check again |
| Port conflict on 11434 | Stop the other process or reconfigure Ollama; Cometline’s built-in profile is loopback-only |
| No models listed | Pull a catalog card or advanced custom name |
| Chat works but memory does not | Enable an embedding model under Ollama and select it in Settings → Memory |
| Embedding migration required | Confirm the re-embed dialog; cancel keeps the previous index |
| Want a remote OpenAI-compatible server | Use **Advanced / Custom endpoint**, not Ollama Local |

## Advanced custom pull

Settings → Ollama Local → advanced custom pull accepts a validated `model:tag` and streams progress through Electron main. Arbitrary URLs and shell commands are rejected.
