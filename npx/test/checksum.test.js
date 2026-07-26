const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const crypto = require('node:crypto');
const { parseChecksums, sha256OfFile, verifyChecksum } = require('../lib/checksum');

test('parses sha256sum-style manifests', () => {
  const text = [
    'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  agent-notify-v1.0.0-linux-amd64.tar.gz',
    'A94A8FE5CCB19BA61C4C0873D391E987982FBBD3E3B0C44298FC1C149AFBF4C8  agent-notify-v1.0.0-darwin-arm64.tar.gz',
    '',
    'not a checksum line',
  ].join('\n');

  const map = parseChecksums(text);
  assert.equal(
    map['agent-notify-v1.0.0-linux-amd64.tar.gz'],
    'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
  );
  // hex 统一转小写,便于直接比较
  assert.equal(
    map['agent-notify-v1.0.0-darwin-arm64.tar.gz'],
    'a94a8fe5ccb19ba61c4c0873d391e987982fbbd3e3b0c44298fc1c149afbf4c8',
  );
  assert.equal(Object.keys(map).length, 2);
});

test('parses binary-mode (asterisk) entries', () => {
  const map = parseChecksums(
    'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 *asset.tar.gz',
  );
  assert.ok(map['asset.tar.gz']);
});

test('verifyChecksum accepts a matching file and rejects a tampered one', (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-checksum-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));

  const filePath = path.join(dir, 'asset.tar.gz');
  fs.writeFileSync(filePath, 'release-payload');
  const good = crypto.createHash('sha256').update('release-payload').digest('hex');

  assert.equal(sha256OfFile(filePath), good);
  assert.doesNotThrow(() => verifyChecksum(filePath, good, 'asset.tar.gz'));

  // 内容被替换后必须拒绝安装
  fs.writeFileSync(filePath, 'tampered-payload');
  assert.throws(
    () => verifyChecksum(filePath, good, 'asset.tar.gz'),
    /checksum mismatch for asset\.tar\.gz/,
  );
});

test('verifyChecksum is case-insensitive on the expected hex', (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-checksum-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));

  const filePath = path.join(dir, 'asset.tar.gz');
  fs.writeFileSync(filePath, 'payload');
  const upper = crypto.createHash('sha256').update('payload').digest('hex').toUpperCase();

  assert.doesNotThrow(() => verifyChecksum(filePath, upper, 'asset.tar.gz'));
});
