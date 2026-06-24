# CometMind Helm Chart

Deploy a headless CometMind instance with HTTP API (`serve`) and optional Discord gateway in a single StatefulSet pod.

## Prerequisites

- Kubernetes 1.24+
- Helm 3
- A container image (build locally with `docker build -f cometmind/Dockerfile .` or use GHCR after release)

## Quick start

```bash
helm install cometmind ./deploy/helm/cometmind \
  --set secrets.ANTHROPIC_API_KEY=sk-ant-... \
  --set secrets.DISCORD_BOT_TOKEN=... \
  --set settings.json.cometmind.gateway.discord.allowedUsers[0]=YOUR_DISCORD_USER_ID
```

Check health:

```bash
kubectl port-forward svc/cometmind 7700:7700
curl http://127.0.0.1:7700/api/v1/health
```

## Architecture

```text
Pod (StatefulSet, replicas: 1)
├── init:     cometmind init -w /workspace
├── serve:    cometmind serve --port 7700 --bind 0.0.0.0
└── gateway:  cometmind gateway run --platform discord   (when discord.enabled=true)

Volumes
├── /data      PVC — cometmind.db, skills/, mcp-oauth/, settings (if not subPath mounted)
├── /workspace emptyDir or PVC — registered workspace root
└── settings   ConfigMap subPath — /data/cometline-settings.json
```

SQLite is single-writer: **keep `replicaCount: 1`**.

## Required values

| Value | Purpose |
|---|---|
| `secrets.ANTHROPIC_API_KEY` (or provider key) | LLM provider authentication |
| `secrets.DISCORD_BOT_TOKEN` | Discord bot token (when gateway enabled) |
| `settings.json.cometmind.gateway.discord.allowedUsers` | Discord user IDs allowed to talk to the bot |

## ConfigMap settings shape

The chart renders `settings.json` into a ConfigMap. Minimum Discord-focused example:

```json
{
  "providers": [
    {
      "id": "anthropic",
      "name": "Anthropic",
      "method": "anthropic",
      "enabled": true,
      "baseURL": "https://api.anthropic.com",
      "apiKey": "",
      "selectedModel": "claude-sonnet-4-5",
      "models": ["claude-sonnet-4-5"],
      "enabledModels": ["claude-sonnet-4-5"]
    }
  ],
  "activeProviderId": "anthropic",
  "defaultProviderId": "anthropic",
  "defaultModelId": "claude-sonnet-4-5",
  "cometmind": {
    "gateway": {
      "discord": {
        "enabled": true,
        "botTokenEnv": "DISCORD_BOT_TOKEN",
        "workspacePath": "/workspace",
        "allowedUsers": ["1234567890"],
        "requireMention": true,
        "providerId": "anthropic",
        "modelId": "claude-sonnet-4-5"
      }
    }
  }
}
```

Provider API keys should come from **Secrets** (env vars), not inline in ConfigMap JSON.

## Environment variables

| Variable | Default in chart | Purpose |
|---|---|---|
| `COMETMIND_DATA_DIR` | `/data` | SQLite, settings, skills root |
| `COMETMIND_BIND_ADDR` | `0.0.0.0` | HTTP bind address for probes |
| `COMETMIND_LOG_LEVEL` | `info` | Log verbosity |
| `DISCORD_BOT_TOKEN` | from Secret | Discord bot auth |

## Admin runbook

### Change settings via ConfigMap (GitOps)

1. Edit `values.yaml` or your overlay (`settings.json`, secrets).
2. `helm upgrade cometmind ./deploy/helm/cometmind -f my-values.yaml`
3. Restart pods so processes reload settings: `kubectl rollout restart statefulset/cometmind`

Settings are read at process start; there is no hot reload.

### Change settings via CLI inside the pod

```bash
kubectl exec -it statefulset/cometmind -c serve -- sh
export COMETMIND_DATA_DIR=/data

cometmind config validate
cometmind gateway show
cometmind model set anthropic claude-sonnet-4-5
cometmind gateway set --workspace /workspace --allowed-user NEW_USER_ID
cometmind settings import /path/to/settings.json
```

Restart after changes that affect running processes:

```bash
kubectl rollout restart statefulset/cometmind
```

### Debug sessions

```bash
kubectl exec -it statefulset/cometmind -c serve -- \
  cometmind session list --all --json
```

## Validation

```bash
helm lint deploy/helm/cometmind
helm template cometmind deploy/helm/cometmind --set secrets.ANTHROPIC_API_KEY=test
```

## Docker (without Helm)

```bash
docker build -f cometmind/Dockerfile -t cometmind:local .
docker run --rm -p 7700:7700 \
  -e COMETMIND_DATA_DIR=/data \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  -v cometmind-data:/data \
  cometmind:local
```

See also [docs/deploy/headless-runbook.md](../../docs/deploy/headless-runbook.md).
