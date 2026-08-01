const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const crypto = require('node:crypto');
const tar = require('tar');

function isUnsafeArchivePath(entryPath) {
  return path.isAbsolute(entryPath) || entryPath.split('/').includes('..');
}

// NOTIFIER_BUNDLE 是 macOS 打包进 tar.gz 的 terminal-notifier app bundle 目录名。
const NOTIFIER_BUNDLE = 'terminal-notifier.app';
// WINDOWS_FOCUS_HELPER 是 Windows release 包内的点击聚焦 helper 文件名。
const WINDOWS_FOCUS_HELPER = 'toast-focus-helper.exe';
// MAC_FOCUS_HELPER 是 macOS release 包内的窗口级聚焦 helper 文件名。
const MAC_FOCUS_HELPER = 'mac-focus-helper';
// AGENT_LOGO_DIR 是 release archive 内 per-agent logo 目录名，运行时解压到 ~/.agent-notify/agentlogo/。
const AGENT_LOGO_DIR = 'agentlogo';
// FALLBACK_ICON 是 release archive 根目录的 fallback 图标，运行时解压到 ~/.agent-notify/agent-notify.png。
const FALLBACK_ICON = 'agent-notify.png';

async function installFromArchive({ archivePath, installDir, binaryNameInArchive, finalBinaryName }) {
  fs.mkdirSync(installDir, { recursive: true });

  const extractDir = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-extract-'));
  const finalPath = path.join(installDir, finalBinaryName);
  // 临时文件名带 pid + 随机后缀:两个 npx 并发首装(两个 agent 同时启动)时
  // 各写各的,且 finally 里的清理只会删掉自己的那份(issue #38)。
  const tempFinalPath = `${finalPath}.tmp-${process.pid}-${crypto.randomBytes(4).toString('hex')}`;

  try {
    const entries = [];
    await tar.t({
      file: archivePath,
      gzip: true,
      onReadEntry: (entry) => entries.push({ path: entry.path, type: entry.type }),
    });

    const binaryEntry = entries.find((entry) => entry.path === binaryNameInArchive);
    if (!binaryEntry) {
      throw new Error(`binary not found in archive: ${binaryNameInArchive}`);
    }

    if (binaryEntry.type !== 'File' || entries.some((entry) => isUnsafeArchivePath(entry.path))) {
      throw new Error('unsafe archive contents');
    }

    // 解压全部内容（含可能的 terminal-notifier.app 目录树）
    await tar.x({
      file: archivePath,
      cwd: extractDir,
      gzip: true,
    });

    const extractedPath = path.join(extractDir, binaryNameInArchive);
    if (!fs.existsSync(extractedPath)) {
      throw new Error(`binary not found in archive: ${binaryNameInArchive}`);
    }

    fs.copyFileSync(extractedPath, tempFinalPath);
    if (process.platform !== 'win32') {
      fs.chmodSync(tempFinalPath, 0o755);
    }
    fs.renameSync(tempFinalPath, finalPath);

    // macOS: 若 tar.gz 内含 terminal-notifier.app，提取到 installDir
    const hasNotifier = entries.some((e) => e.path === NOTIFIER_BUNDLE || e.path.startsWith(NOTIFIER_BUNDLE + '/'));
    if (hasNotifier) {
      const srcBundle = path.join(extractDir, NOTIFIER_BUNDLE);
      const dstBundle = path.join(installDir, NOTIFIER_BUNDLE);
      if (fs.existsSync(srcBundle)) {
        fs.rmSync(dstBundle, { recursive: true, force: true });
        fs.cpSync(srcBundle, dstBundle, { recursive: true });
        const exe = path.join(dstBundle, 'Contents', 'MacOS', 'terminal-notifier');
        if (fs.existsSync(exe)) {
          fs.chmodSync(exe, 0o755);
        }
      }
    }

    // Windows: 若 tar.gz 内含点击聚焦 helper，提取到 installDir，与 agent-notify.exe 同目录。
    const hasWindowsFocusHelper = entries.some((e) => e.path === WINDOWS_FOCUS_HELPER && e.type === 'File');
    if (hasWindowsFocusHelper) {
      const srcHelper = path.join(extractDir, WINDOWS_FOCUS_HELPER);
      const dstHelper = path.join(installDir, WINDOWS_FOCUS_HELPER);
      if (fs.existsSync(srcHelper)) {
        fs.copyFileSync(srcHelper, dstHelper);
      }
    }

    // macOS: 若 tar.gz 内含窗口级聚焦 helper，提取到 installDir，与 agent-notify 同目录。
    const hasMacFocusHelper = entries.some((e) => e.path === MAC_FOCUS_HELPER && e.type === 'File');
    if (hasMacFocusHelper) {
      const srcHelper = path.join(extractDir, MAC_FOCUS_HELPER);
      const dstHelper = path.join(installDir, MAC_FOCUS_HELPER);
      if (fs.existsSync(srcHelper)) {
        fs.copyFileSync(srcHelper, dstHelper);
        fs.chmodSync(dstHelper, 0o755);
      }
    }

    // per-agent logo 资源（docs/agent-logo-plan.md）：
    //   agentlogo/<agent>.png -> installDir/agentlogo/  覆盖更新 release 自带 logo，
    //     但保留用户自放的其它 logo —— agentlogo/ 是图标查找链的第一优先级，用户可覆盖。
    //   agent-notify.png      -> installDir/  fallback 图标。
    // 逐文件拷贝（仅顶层文件），避免整目录 rmSync 清掉用户自定义 logo。
    for (const entry of entries) {
      if (entry.type !== 'File') continue;
      if (entry.path.startsWith(AGENT_LOGO_DIR + '/')) {
        const rel = entry.path.slice(AGENT_LOGO_DIR.length + 1);
        // 全局 isUnsafeArchivePath 已防 .. 与绝对路径，这里再挡一层子目录，保持 agentlogo/ 扁平。
        if (!rel || rel.includes('/')) continue;
        const srcFile = path.join(extractDir, AGENT_LOGO_DIR, rel);
        const dstDir = path.join(installDir, AGENT_LOGO_DIR);
        fs.mkdirSync(dstDir, { recursive: true });
        if (fs.existsSync(srcFile)) {
          fs.copyFileSync(srcFile, path.join(dstDir, rel));
        }
      } else if (entry.path === FALLBACK_ICON) {
        const srcFile = path.join(extractDir, FALLBACK_ICON);
        if (fs.existsSync(srcFile)) {
          fs.copyFileSync(srcFile, path.join(installDir, FALLBACK_ICON));
        }
      }
    }

    return finalPath;
  } finally {
    fs.rmSync(tempFinalPath, { force: true });
    fs.rmSync(extractDir, { recursive: true, force: true });
  }
}

module.exports = {
  installFromArchive,
  NOTIFIER_BUNDLE,
  WINDOWS_FOCUS_HELPER,
  MAC_FOCUS_HELPER,
  AGENT_LOGO_DIR,
  FALLBACK_ICON,
};
