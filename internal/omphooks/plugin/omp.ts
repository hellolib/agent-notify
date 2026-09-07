// agent-notify OMP extension
// The binary path is baked in by agent-notify when this file is installed.
import { spawn } from "node:child_process";

const BINARY = __AGENT_NOTIFY_BINARY__;

function sessionId(ctx: any, fallback?: string): string {
  if (fallback) return String(fallback);
  try {
    return String(ctx?.sessionManager?.getSessionId?.() ?? "");
  } catch {
    return "";
  }
}

function emit(type: string, fields: Record<string, unknown> = {}) {
  const payload = JSON.stringify({ type, ...fields });
  try {
    const child = spawn(BINARY, ["handle-omp-hook"], {
      stdio: ["pipe", "ignore", "ignore"],
      detached: true,
    });
    child.stdin.write(payload);
    child.stdin.end();
    child.on("error", () => {});
    child.unref();
  } catch {
    // A notification extension must never break the OMP session.
  }
}

export default function agentNotify(pi: any) {
  pi.on("session_start", async (_event: any, ctx: any) => {
    emit("session_start", {
      session_id: sessionId(ctx),
      cwd: ctx?.cwd ?? process.cwd(),
    });
  });

  pi.on("tool_approval_requested", async (event: any, ctx: any) => {
    emit("tool_approval_requested", {
      session_id: sessionId(ctx, event?.sessionId),
      cwd: ctx?.cwd ?? process.cwd(),
      tool_name: event?.toolName ?? "",
      reason: event?.reason ?? "",
    });
  });

  pi.on("tool_execution_end", async (event: any, ctx: any) => {
    if (!event?.isError) return;
    emit("tool_execution_end", {
      session_id: sessionId(ctx),
      cwd: ctx?.cwd ?? process.cwd(),
      tool_name: event?.toolName ?? "",
      is_error: true,
    });
  });

  pi.on("session_stop", async (event: any, ctx: any) => {
    if (event?.stop_hook_active || event?.signal?.aborted) return;
    emit("session_stop", {
      session_id: sessionId(ctx, event?.session_id),
      cwd: ctx?.cwd ?? process.cwd(),
      stop_hook_active: Boolean(event?.stop_hook_active),
      signal_aborted: Boolean(event?.signal?.aborted),
    });
  });
}
