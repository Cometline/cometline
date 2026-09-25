# Output limit

This page uses plain English (about B2). Sentences are short. A new word is explained the first time it appears.

Read this after [05-cometmind-runtime.md](./05-cometmind-runtime.md).

## The problem

A model reply has a size limit. That limit is called the **output limit**. It counts the tokens in one step. A **token** is a small piece of text. About four characters is one token.

If the limit is too small, the model stops in the middle. This can cut a thought, a tool call, or the final answer. An old hidden default of 4096 tokens was too small for many models.

If the limit is too large, the app must reserve that space in the **context window**. The context window is the total space for the prompt plus the reply. A huge reserve leaves less room for the conversation.

## The rule we use

CometMind does not ask you to type a token number.

For each step it uses the **current model**, not the default model. You can change the model in the middle of a chat. The next step uses the new model.

The formula is in `cometmind/internal/agent/contextwindow.go`:

```text
step limit = min(this model's output limit, 32,000)
```

`min` means "the smaller number".

| Model output limit | Step limit |
| --- | --- |
| 8,000 | 8,000 |
| 32,000 | 32,000 |
| 128,000 | 32,000 |
| unknown | 32,000 |

32,000 is the same ceiling OpenCode uses (`OUTPUT_TOKEN_MAX`). It is large enough for a normal thought and one tool call. It does not reserve a 128,000-token reply inside the context window.

## What the settings file does not do

`cometmind.maxTokens` may still appear in `~/.cometmind/cometline-settings.json`. The agent does **not** use that number as the step limit. The Settings screen no longer shows a "Max output tokens" field.

Do not add a new slider for this. The limit follows the model on the turn.

## Thinking is not a second wallet

**Thinking** is the model's private reasoning. On the screen it appears as a Thinking block.

On most current models, thinking tokens and answer tokens share the same step limit. There is no separate thinking budget.

- Claude 4.5 and older can take `thinking.budget_tokens`. We do not send that field. Those models are rarely used now.
- Claude 4.6, Claude 4.7, and Claude 5 (including Opus 5 and Opus 5.5) reject `budget_tokens`. They use **effort** (`low`, `medium`, `high`, …). Effort changes how hard the model thinks. It is not a token count. The hard stop is still the step limit.
- OpenAI and xAI also have no separate thinking-token field. Reasoning uses the same 32,000 cap.

So a model can still spend the whole step on thinking. The step then ends with finish reason `max_tokens`. CometMind may ask the model to continue, up to 8 extra steps, and only while the turn is still under `MaxSteps` (default 100). A cut tool call can be retried up to 4 times. The extra steps are a safety net, not the normal path.

## Context reserve

Before a step, CometMind reserves space for the reply. The reserve is the larger of:

- the step limit, and
- 20,000 tokens

The rest of the context window is available for the prompt. If the prompt is too big, the runtime **compacts** older messages into a short summary. Compaction is not the same as memory compaction.

Code: `EffectiveMaxTokens` and `ComputeReserveAndAvailable` in `contextwindow.go`.

## What the chat UI does

If a continuation creates several reasoning pieces in a row, the UI joins them into one Thinking block. You should not see a stack of Thinking buttons for one thought.

Code: `coalesceReasoningEntries` in `cometline/src/lib/conversation/thinking-attribution.ts`.

## Exercise

1. Open `contextwindow.go` and find `OutputTokenMax`. What number is it?
2. A model lists an output limit of 8,192. What step limit does `EffectiveMaxTokens` return?
3. A model lists 128,000. What step limit does it return?
4. Why do we not send `budget_tokens` to `claude-opus-5`?
