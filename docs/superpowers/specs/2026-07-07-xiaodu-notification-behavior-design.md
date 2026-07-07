# Xiaodu Notification Behavior Design

Date: 2026-07-07

## Problem

Xiaodu is a transient audio channel. It speaks once and leaves no visible,
persistent notification surface. Existing notification channels such as system
notifications, Bark, Slack, and webhooks can be reviewed after delivery, but a
missed Xiaodu speech message is gone.

The current Xiaodu sender treats every event as a single one-shot speech
message. That is acceptable for completion notices, but weak for events that
need user action, especially permission confirmation and input waiting.

## Goals

- Make Xiaodu useful as an attention-capturing channel.
- Avoid turning completion notifications into noisy repeated speech.
- Keep persistent details in existing durable channels.
- Include the short project name in Xiaodu speech when workspace information is
  available.
- Avoid blocking Codex or other agent hook execution while reminders repeat.
- Keep configuration small and understandable.

## Non-Goals

- Do not make Xiaodu a durable notification store.
- Do not add full per-event scheduling policy in this iteration.
- Do not require Xiaodu device interaction or voice acknowledgement.
- Do not change behavior of non-Xiaodu senders.

## Recommended Behavior

Xiaodu should use channel-specific behavior:

- `permission_required`: speak immediately and repeat while pending.
- `input_required`: speak immediately and repeat while pending.
- `run_failed`: speak immediately and repeat a fixed number of times.
- `run_completed`: speak once by default.
- `session_start`: do not speak by default unless user explicitly subscribes and
  future config enables it.

The default keeps completion feedback audible because the user requested
`speak_completed` default `true`, while still treating action-needed events as
more urgent.

## Configuration

Add Xiaodu-specific fields to `XiaoduChannelConfig`:

- `speak_completed bool`, default `true`.
- `repeat_count int`, default `2`.
- `repeat_interval_seconds int`, default `25`.

Interpretation:

- `repeat_count` is the total number of speeches for repeatable events,
  including the immediate first speech.
- If `repeat_count <= 1`, no background repeat is scheduled.
- If `repeat_interval_seconds <= 0`, use the default interval.
- `run_completed` ignores repeat settings and only uses `speak_completed`.

## Reminder Cancellation

Hook processes must not sleep in the foreground. Repeating inside the original
hook process would block agent flow, especially permission approval.

Instead, Xiaodu reminders should be background-driven:

1. On `permission_required` or `input_required`, speak immediately.
2. Store a pending reminder keyed by agent, session id, event, and tool name
   when available.
3. Start a detached background reminder process for remaining repeats.
4. On later agent activity that proves progress, cancel matching pending
   reminders.

For Codex, `PostToolUse` is the best cancellation signal after permission is
approved and the tool runs. The Codex hook installer should add a lightweight
`PostToolUse` hook for `agent-notify` cancellation only. It should not send a
user notification.

On `Stop`, clear all pending reminders for the session.

If the user denies permission or never responds, no reliable denial callback is
available. The reminder stops after `repeat_count`.

## Speech Copy

Speech text should be short and action-oriented:

- `Codex in agent-notify needs permission confirmation. Please return to the terminal.`
- `Codex in agent-notify is waiting for your input. Please return to the terminal.`
- `Codex in agent-notify task failed and needs attention.`
- `Codex in agent-notify task completed.`

Chinese locale equivalents should be similarly short:

- `Codex 在 agent-notify 需要权限确认，请回到终端处理。`
- `Codex 在 agent-notify 正在等待你的输入，请回到终端处理。`
- `Codex 在 agent-notify 任务失败，需要处理。`
- `Codex 在 agent-notify 任务已完成。`

Project name should come from `notify.Message.Workspace` using the existing
short workspace rule. If workspace is empty, omit the project phrase. Do not
read full paths, long task titles, JSON payloads, or detailed summaries on
Xiaodu by default.

## Data Flow

Normal repeatable event:

1. Agent hook invokes `agent-notify handle-*-hook`.
2. Hook parser produces `notify.Message`.
3. Dispatcher sends to Xiaodu.
4. Xiaodu sender speaks immediately.
5. Reminder scheduler records pending reminder and starts background worker.
6. Worker checks pending state before each repeat.
7. Worker stops after cancellation or after configured repeat limit.

Cancellation:

1. Codex emits `PostToolUse` after approved tool execution.
2. `agent-notify` handles the event as cancellation-only.
3. Pending reminder matching session/tool is removed.
4. Background worker observes missing/cancelled state and exits.

## Error Handling

- If Xiaodu immediate speech fails, return sender error as today.
- If reminder state cannot be written, return error for action-needed events.
- If detached background process cannot start, log and return error.
- If cancellation fails, append log entry but do not block the agent.
- If a repeat speech fails, append log entry and stop that reminder.

## Testing

Add unit tests for:

- `run_completed` speaks once when `speak_completed=true`.
- `run_completed` is skipped when `speak_completed=false`.
- `permission_required` creates pending reminder when repeats are enabled.
- `PostToolUse` cancels pending Codex permission reminder.
- `Stop` clears pending reminders for the session.
- Background worker exits when reminder is cancelled.
- Speech copy remains short and excludes long title/body content.
- Speech copy includes short project name when workspace is available.

Add integration-style tests for Codex hook settings:

- `PermissionRequest` includes notification command.
- `Stop` includes notification command.
- `PostToolUse` includes cancellation command only.

## Rollout

Use backwards-compatible defaults:

- Existing configs without new fields get `speak_completed=true`.
- Existing configs without repeat fields use `repeat_count=2` and
  `repeat_interval_seconds=25`.
- Non-Xiaodu channels keep current behavior.

Users who want quiet Xiaodu completion notices can set
`speak_completed=false`.
