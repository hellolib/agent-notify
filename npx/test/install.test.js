const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { PassThrough } = require('node:stream');
const tar = require('tar');
const { installFromArchive, WINDOWS_FOCUS_HELPER, MAC_FOCUS_HELPER, AGENT_LOGO_DIR, FALLBACK_ICON } = require('../lib/install');
const { downloadToFile } = require('../lib/download');

test('replaces installed binary with extracted binary', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-install-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const installDir = path.join(root, '.agent-notify');
  const archivePath = path.join(root, 'agent-notify-v0.2.3-linux-amd64.tar.gz');
  const extractedBinaryName = 'agent-notify-v0.2.3-linux-amd64';

  fs.mkdirSync(path.join(root, 'src'));
  fs.writeFileSync(path.join(root, 'src', extractedBinaryName), 'new-binary');

  await tar.c(
    {
      gzip: true,
      file: archivePath,
      cwd: path.join(root, 'src'),
    },
    [extractedBinaryName],
  );

  fs.mkdirSync(installDir, { recursive: true });
  fs.writeFileSync(path.join(installDir, 'agent-notify'), 'old-binary');

  const installedPath = await installFromArchive({
    archivePath,
    installDir,
    binaryNameInArchive: extractedBinaryName,
    finalBinaryName: 'agent-notify',
  });

  assert.equal(installedPath, path.join(installDir, 'agent-notify'));
  assert.equal(fs.readFileSync(installedPath, 'utf8'), 'new-binary');
});

test('installs windows focus helper when archive contains it', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-install-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const installDir = path.join(root, '.agent-notify');
  const archivePath = path.join(root, 'agent-notify-v0.2.3-windows-amd64.tar.gz');
  const extractedBinaryName = 'agent-notify-v0.2.3-windows-amd64.exe';

  fs.mkdirSync(path.join(root, 'src'));
  fs.writeFileSync(path.join(root, 'src', extractedBinaryName), 'new-binary');
  fs.writeFileSync(path.join(root, 'src', WINDOWS_FOCUS_HELPER), 'helper-binary');

  await tar.c(
    {
      gzip: true,
      file: archivePath,
      cwd: path.join(root, 'src'),
    },
    [extractedBinaryName, WINDOWS_FOCUS_HELPER],
  );

  await installFromArchive({
    archivePath,
    installDir,
    binaryNameInArchive: extractedBinaryName,
    finalBinaryName: 'agent-notify.exe',
  });

  const helperPath = path.join(installDir, WINDOWS_FOCUS_HELPER);
  assert.equal(fs.readFileSync(helperPath, 'utf8'), 'helper-binary');
});

test('installs mac focus helper when archive contains it', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-install-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const installDir = path.join(root, '.agent-notify');
  const archivePath = path.join(root, 'agent-notify-v0.2.3-darwin-arm64.tar.gz');
  const extractedBinaryName = 'agent-notify-v0.2.3-darwin-arm64';

  fs.mkdirSync(path.join(root, 'src'));
  fs.writeFileSync(path.join(root, 'src', extractedBinaryName), 'new-binary');
  fs.writeFileSync(path.join(root, 'src', MAC_FOCUS_HELPER), 'helper-binary');

  await tar.c(
    {
      gzip: true,
      file: archivePath,
      cwd: path.join(root, 'src'),
    },
    [extractedBinaryName, MAC_FOCUS_HELPER],
  );

  await installFromArchive({
    archivePath,
    installDir,
    binaryNameInArchive: extractedBinaryName,
    finalBinaryName: 'agent-notify',
  });

  const helperPath = path.join(installDir, MAC_FOCUS_HELPER);
  assert.equal(fs.readFileSync(helperPath, 'utf8'), 'helper-binary');
  // helper must be executable (0o755) on macOS
  const mode = fs.statSync(helperPath).mode;
  assert.equal(mode & 0o111, 0o111, 'mac-focus-helper should be executable');
});

test('installs per-agent logos and fallback icon when archive contains agentlogo/', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-install-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const installDir = path.join(root, '.agent-notify');
  const archivePath = path.join(root, 'agent-notify-v0.2.3-linux-amd64.tar.gz');
  const extractedBinaryName = 'agent-notify-v0.2.3-linux-amd64';

  fs.mkdirSync(path.join(root, 'src'));
  fs.writeFileSync(path.join(root, 'src', extractedBinaryName), 'new-binary');
  // release archive 在根目录打包 agentlogo/ 目录与 agent-notify.png
  fs.mkdirSync(path.join(root, 'src', AGENT_LOGO_DIR));
  fs.writeFileSync(path.join(root, 'src', AGENT_LOGO_DIR, 'claude.png'), 'claude-logo');
  fs.writeFileSync(path.join(root, 'src', AGENT_LOGO_DIR, 'openai.png'), 'openai-logo');
  fs.writeFileSync(path.join(root, 'src', FALLBACK_ICON), 'fallback-logo');

  await tar.c(
    {
      gzip: true,
      file: archivePath,
      cwd: path.join(root, 'src'),
    },
    [extractedBinaryName, AGENT_LOGO_DIR, FALLBACK_ICON],
  );

  await installFromArchive({
    archivePath,
    installDir,
    binaryNameInArchive: extractedBinaryName,
    finalBinaryName: 'agent-notify',
  });

  // agentlogo/<agent>.png -> ~/.agent-notify/agentlogo/<agent>.png
  assert.equal(
    fs.readFileSync(path.join(installDir, AGENT_LOGO_DIR, 'claude.png'), 'utf8'),
    'claude-logo',
  );
  assert.equal(
    fs.readFileSync(path.join(installDir, AGENT_LOGO_DIR, 'openai.png'), 'utf8'),
    'openai-logo',
  );
  // agent-notify.png -> ~/.agent-notify/agent-notify.png
  assert.equal(fs.readFileSync(path.join(installDir, FALLBACK_ICON), 'utf8'), 'fallback-logo');
});

test('preserves user-added custom logos when updating agentlogo/', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-install-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const installDir = path.join(root, '.agent-notify');
  const archivePath = path.join(root, 'agent-notify-v0.2.3-linux-amd64.tar.gz');
  const extractedBinaryName = 'agent-notify-v0.2.3-linux-amd64';

  // 用户此前自放了自定义 logo，升级前已存在
  fs.mkdirSync(path.join(installDir, AGENT_LOGO_DIR), { recursive: true });
  fs.writeFileSync(path.join(installDir, AGENT_LOGO_DIR, 'custom.png'), 'user-custom');

  fs.mkdirSync(path.join(root, 'src'));
  fs.writeFileSync(path.join(root, 'src', extractedBinaryName), 'new-binary');
  fs.mkdirSync(path.join(root, 'src', AGENT_LOGO_DIR));
  fs.writeFileSync(path.join(root, 'src', AGENT_LOGO_DIR, 'claude.png'), 'claude-logo');

  await tar.c(
    {
      gzip: true,
      file: archivePath,
      cwd: path.join(root, 'src'),
    },
    [extractedBinaryName, AGENT_LOGO_DIR],
  );

  await installFromArchive({
    archivePath,
    installDir,
    binaryNameInArchive: extractedBinaryName,
    finalBinaryName: 'agent-notify',
  });

  // release 自带 logo 被覆盖更新
  assert.equal(
    fs.readFileSync(path.join(installDir, AGENT_LOGO_DIR, 'claude.png'), 'utf8'),
    'claude-logo',
  );
  // 用户自放 logo 保留 —— agentlogo/ 是图标查找链第一优先级，不应被升级清空
  assert.equal(
    fs.readFileSync(path.join(installDir, AGENT_LOGO_DIR, 'custom.png'), 'utf8'),
    'user-custom',
  );
});

test('throws when archive does not contain expected binary', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-install-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const installDir = path.join(root, '.agent-notify');
  const archivePath = path.join(root, 'empty.tar.gz');

  fs.mkdirSync(path.join(root, 'src'));
  fs.writeFileSync(path.join(root, 'src', 'other-file'), 'nope');

  await tar.c(
    {
      gzip: true,
      file: archivePath,
      cwd: path.join(root, 'src'),
    },
    ['other-file'],
  );

  await assert.rejects(
    installFromArchive({
      archivePath,
      installDir,
      binaryNameInArchive: 'agent-notify-v0.2.3-linux-amd64',
      finalBinaryName: 'agent-notify',
    }),
    /binary not found in archive/,
  );
});

test('follows relative redirects when downloading', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-download-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const destinationPath = path.join(root, 'archive.tar.gz');

  const client = {
    get(url, options, handler) {
      const request = new PassThrough();
      process.nextTick(() => {
        if (url === 'https://example.com/releases/archive.tar.gz') {
          const response = new PassThrough();
          response.statusCode = 302;
          response.headers = { location: '/download/archive.tar.gz' };
          handler(response);
          response.end();
          return;
        }

        const response = new PassThrough();
        response.statusCode = 200;
        response.headers = {};
        handler(response);
        response.end('payload');
      });
      return request;
    },
  };

  const filePath = await downloadToFile('https://example.com/releases/archive.tar.gz', destinationPath, client);
  assert.equal(filePath, destinationPath);
  assert.equal(fs.readFileSync(destinationPath, 'utf8'), 'payload');
});

test('removes destination file when download fails with non-200 response', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-download-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const destinationPath = path.join(root, 'archive.tar.gz');

  const client = {
    get(_url, _options, handler) {
      const request = new PassThrough();
      process.nextTick(() => {
        const response = new PassThrough();
        response.statusCode = 404;
        response.headers = {};
        handler(response);
        response.end();
      });
      return request;
    },
  };

  await assert.rejects(
    downloadToFile('https://example.com/releases/archive.tar.gz', destinationPath, client),
    /download failed: 404/,
  );
  assert.equal(fs.existsSync(destinationPath), false);
});

test('rejects with timeout error when download takes too long', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-download-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const destinationPath = path.join(root, 'archive.tar.gz');

  const client = {
    get(_url, _options, handler) {
      const request = new PassThrough();
      process.nextTick(() => {
        request.emit('timeout');
      });
      return request;
    },
  };

  await assert.rejects(
    downloadToFile('https://example.com/releases/archive.tar.gz', destinationPath, client),
    /download timeout after 300s/,
  );
  assert.equal(fs.existsSync(destinationPath), false);
});

// issue #38:两个 npx 并发首装时,临时文件名必须互不冲突,
// 否则一方的 finally 清理会删掉另一方进行中的临时拷贝。
test('concurrent installs from the same archive both succeed', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-concurrent-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const installDir = path.join(root, '.agent-notify');
  const archivePath = path.join(root, 'agent-notify-v0.2.3-linux-amd64.tar.gz');
  const extractedBinaryName = 'agent-notify-v0.2.3-linux-amd64';

  fs.mkdirSync(path.join(root, 'src'));
  fs.writeFileSync(path.join(root, 'src', extractedBinaryName), 'new-binary');
  await tar.c({ gzip: true, file: archivePath, cwd: path.join(root, 'src') }, [extractedBinaryName]);
  fs.mkdirSync(installDir, { recursive: true });

  const opts = {
    archivePath,
    installDir,
    binaryNameInArchive: extractedBinaryName,
    finalBinaryName: 'agent-notify',
  };
  const results = await Promise.all([
    installFromArchive({ ...opts }),
    installFromArchive({ ...opts }),
    installFromArchive({ ...opts }),
  ]);

  for (const installedPath of results) {
    assert.equal(fs.readFileSync(installedPath, 'utf8'), 'new-binary');
  }
  // 不留下任何 .tmp- 残骸
  const leftovers = fs.readdirSync(installDir).filter((name) => name.includes('.tmp-'));
  assert.deepEqual(leftovers, []);
});
