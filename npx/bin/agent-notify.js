#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { getPlatformTarget } = require('../lib/platform');
const { extractSemver, compareVersions } = require('../lib/version');
const { getInstalledBinaryPath, TMP_DIR } = require('../lib/constants');
const { buildAssetName, buildDownloadUrl } = require('../lib/release');
const { downloadToFile } = require('../lib/download');
const { installFromArchive } = require('../lib/install');
const { CHECKSUMS_ASSET, parseChecksums, verifyChecksum } = require('../lib/checksum');
const { runBinary } = require('../lib/run');
const { clearQuarantine, adHocSign, NOTIFIER_BUNDLE } = require('../lib/notifier');

function getDesiredVersion() {
  return require('../package.json').version;
}

function getInstalledVersion(binaryPath) {
  const result = spawnSync(binaryPath, ['--version'], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  if (result.error || result.status !== 0) {
    return null;
  }

  return extractSemver(result.stdout);
}

// fetchExpectedChecksum 取回该 release 的 SHA256SUMS 并查出 assetName 的期望值。
// 返回 null 表示「该 release 没有发布校验和」——v0.14.x 及更早的 release 都没有,
// 此时降级为不校验并提醒用户,而不是让老版本彻底装不上。
async function fetchExpectedChecksum(version, assetName, downloadDir, warn) {
  const checksumsPath = path.join(downloadDir, CHECKSUMS_ASSET);
  try {
    await downloadToFile(buildDownloadUrl(version, CHECKSUMS_ASSET), checksumsPath);
  } catch (error) {
    if (error.statusCode === 404) {
      warn(`warning: release v${version} publishes no ${CHECKSUMS_ASSET}; skipping integrity check`);
      return null;
    }
    throw error;
  }
  const expected = parseChecksums(fs.readFileSync(checksumsPath, 'utf8'))[assetName];
  if (!expected) {
    warn(`warning: ${CHECKSUMS_ASSET} has no entry for ${assetName}; skipping integrity check`);
    return null;
  }
  return expected;
}

async function downloadAndInstall(version, target, binaryPath, warn = (m) => console.warn(m)) {
  fs.mkdirSync(TMP_DIR, { recursive: true });

  const assetName = buildAssetName(version, target);
  const binaryNameInArchive = `agent-notify-v${version}-${target.goos}-${target.goarch}${target.ext}`;

  // 下载到本次调用独占的临时目录:共享的确定性路径会让两个并发 npx
  // (两个 agent 同时首启)互相截断下载,产生损坏的 gzip(issue #38)。
  const downloadDir = fs.mkdtempSync(path.join(TMP_DIR, 'download-'));
  const archivePath = path.join(downloadDir, assetName);

  let installed;
  try {
    await downloadToFile(buildDownloadUrl(version, assetName), archivePath);

    // 校验完整性后再安装:下载到的必须是我们发布的那个文件(issue #37)
    const expected = await fetchExpectedChecksum(version, assetName, downloadDir, warn);
    if (expected) {
      verifyChecksum(archivePath, expected, assetName);
    }

    installed = await installFromArchive({
      archivePath,
      installDir: path.dirname(binaryPath),
      binaryNameInArchive,
      finalBinaryName: path.basename(binaryPath),
    });
  } finally {
    fs.rmSync(downloadDir, { recursive: true, force: true });
  }

  // macOS：terminal-notifier.app 已随 tar.gz 解压到 installDir（见 install.js），
  // 此处只做 quarantine 清除与 ad-hoc 签名（点击跳转依赖）。失败仅警告，不阻断主流程。
  if (process.platform === 'darwin') {
    const bundlePath = path.join(path.dirname(binaryPath), NOTIFIER_BUNDLE);
    if (fs.existsSync(bundlePath)) {
      try { clearQuarantine(bundlePath); } catch {}
      try { adHocSign(bundlePath); } catch {}
    }
  }

  return installed;
}

async function main(argv, deps = {}) {
  const rawDesiredVersion = (deps.getDesiredVersion || getDesiredVersion)();
  const target = (deps.getPlatformTarget || getPlatformTarget)();
  const binaryPath = (deps.getInstalledBinaryPath || getInstalledBinaryPath)(target);
  const pathExists = deps.pathExists || fs.existsSync;
  const getVersion = deps.getInstalledVersion || getInstalledVersion;
  const compare = deps.compareVersions || compareVersions;
  const warn = deps.warn || ((message) => console.warn(message));
  const install = deps.downloadAndInstall || ((version, releaseTarget) => downloadAndInstall(version, releaseTarget, binaryPath, warn));
  const run = deps.runBinary || runBinary;

  let installedVersion = null;
  const hasInstalledBinary = pathExists(binaryPath);
  if (hasInstalledBinary) {
    installedVersion = getVersion(binaryPath);
  }
  const canFallbackToInstalledBinary = Boolean(installedVersion);

  // package.json 的 version 由 CI 从 tag 写入,正常情况下必然合法。手工改坏了
  // 的话,与其带着垃圾去拼一个必然 404 的下载地址(还要先耗掉一次网络往返),
  // 不如就地说清楚。已有可用二进制时降级运行它,不让一个打包错误砸掉用户的手。
  const desiredVersion = extractSemver(rawDesiredVersion);
  if (!desiredVersion) {
    if (!canFallbackToInstalledBinary) {
      throw new Error(`package.json has an unusable version "${rawDesiredVersion}"; reinstall agent-notify`);
    }
    warn(`unusable package version "${rawDesiredVersion}"; running installed v${installedVersion}`);
    return run(binaryPath, argv);
  }

  // 精确 pin:npx agent-notify@X 就该跑 X。旧的「只在 installed < desired 时装」
  // 让显式降级静默失效——用户拿到的仍是那个更新的二进制,且没有任何提示。
  // hook 命令写的是二进制绝对路径、不经 npx,所以来回切版本的开销只落在
  // 手动交替执行 npx 的人身上。
  const needsInstall = !hasInstalledBinary
    || !installedVersion
    || compare(installedVersion, desiredVersion) !== 0;

  if (needsInstall) {
    try {
      await install(desiredVersion, target, binaryPath);
    } catch (error) {
      if (!canFallbackToInstalledBinary) {
        throw error;
      }
      warn(`failed to update agent-notify: ${error.message}`);
    }
  }

  return run(binaryPath, argv);
}

if (require.main === module) {
  main(process.argv.slice(2))
    .then((code) => {
      process.exitCode = code;
    })
    .catch((error) => {
      console.error(error.message);
      process.exitCode = 1;
    });
}

module.exports = {
  main,
  downloadAndInstall,
  getDesiredVersion,
  getInstalledVersion,
};
