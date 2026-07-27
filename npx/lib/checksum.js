const fs = require('node:fs');
const crypto = require('node:crypto');

// CHECKSUMS_ASSET 是 release 里校验和清单的文件名(release.yml 生成)。
const CHECKSUMS_ASSET = 'SHA256SUMS';

// parseChecksums 解析 `sha256sum` 风格的清单:每行 "<hex>  <filename>"。
// 返回 { filename: hexLowercase } 映射;无法识别的行忽略。
function parseChecksums(text) {
  const map = {};
  for (const line of text.split('\n')) {
    const match = line.trim().match(/^([0-9a-fA-F]{64})\s+\*?(.+)$/);
    if (match) {
      map[match[2].trim()] = match[1].toLowerCase();
    }
  }
  return map;
}

function sha256OfFile(filePath) {
  const hash = crypto.createHash('sha256');
  hash.update(fs.readFileSync(filePath));
  return hash.digest('hex');
}

// verifyChecksum 校验 archivePath 的 sha256 是否等于 expectedHex。
// 不匹配时抛错,调用方应当中止安装——下载到的不是我们发布的那个文件。
function verifyChecksum(archivePath, expectedHex, assetName) {
  const actual = sha256OfFile(archivePath);
  if (actual !== expectedHex.toLowerCase()) {
    throw new Error(
      `checksum mismatch for ${assetName}: expected ${expectedHex}, got ${actual}. ` +
      'The download may be corrupted or tampered with; installation aborted.',
    );
  }
}

module.exports = {
  CHECKSUMS_ASSET,
  parseChecksums,
  sha256OfFile,
  verifyChecksum,
};
