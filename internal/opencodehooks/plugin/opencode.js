// agent-notify opencode plugin
// 分发时由 agent-notify 二进制写出，binaryPath 烘焙为绝对路径。
import { spawn } from "node:child_process";

const BINARY = "__AGENT_NOTIFY_BINARY__";

export const server = ({ directory }) => ({
  event({ event }) {
    const type = event?.type;
    if (!type) return;

    const subscribed = new Set([
      "session.created",
      "permission.asked",
      "session.status",
      "session.idle",
      "session.error",
    ]);
    if (!subscribed.has(type)) return;

    // session.status 的 status.type 为 busy 表示开始处理，不是通知时机，
    // 仅 idle 时才转发（映射为 input_required）。
    if (type === "session.status" && event?.properties?.status?.type !== "idle") return;

    // 从 session.created 的 info.directory 补充工作区，
    // 其他事件可能不带 directory，用加载时的 directory 兜底。
    const dir = event?.properties?.info?.directory || event?.properties?.directory || directory;

    const payload = {
      type: type,
      properties: {
        ...event?.properties,
        directory: dir,
      },
    };

    try {
      const child = spawn(BINARY, ["handle-opencode-hook"], {
        stdio: ["pipe", "ignore", "ignore"],
        detached: true,
      });
      child.stdin.write(JSON.stringify(payload));
      child.stdin.end();
      child.on("error", () => {});
      child.unref();
    } catch (e) {
      // 插件绝不能让 opencode 崩溃
    }
  },
});
