const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
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

async function installFromArchive({ archivePath, installDir, binaryNameInArchive, finalBinaryName }) {
  fs.mkdirSync(installDir, { recursive: true });

  const extractDir = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-extract-'));
  const finalPath = path.join(installDir, finalBinaryName);
  const tempFinalPath = `${finalPath}.tmp`;

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
};
