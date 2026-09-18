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

  // ask 工具阻塞会话等待用户回答，等价于「等待输入」。OMP 自己的通知层正是用
  // tool_execution_start + toolName==="ask" 产生 question_asked 事件，
  // 因此把它归一化为 input_required（此前 README 声称没有此信号，不成立）。
  pi.on("tool_execution_start", async (event: any, ctx: any) => {
    if ((event?.toolName ?? "") !== "ask") return;
    const q = event?.args?.questions?.[0]?.question;
    emit("question_asked", {
      session_id: sessionId(ctx),
      cwd: ctx?.cwd ?? process.cwd(),
      tool_name: "ask",
      question: typeof q === "string" ? q : "",
    });
  });

  pi.on("session_stop", async (event: any, ctx: any) => {
    // 会话要续写（stop hook 触发）或被用户信号中断，都不算完成。
    if (event?.stop_hook_active || event?.signal?.aborted) return;
    // 转发最后一条助手消息的 stopReason / errorMessage，供 Go 侧区分
    // 「正常运行完成」与「运行失败」（stopReason === "error"）。
    const last = event?.last_assistant_message;
    emit("session_stop", {
      session_id: sessionId(ctx, event?.session_id),
      cwd: ctx?.cwd ?? process.cwd(),
      stop_reason: typeof last?.stopReason === "string" ? last.stopReason : "",
      error_message: typeof last?.errorMessage === "string" ? last.errorMessage : "",
    });
  });
}
