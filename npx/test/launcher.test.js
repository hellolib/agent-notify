const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { main } = require('../bin/agent-notify');

test('downloads when installed binary is missing', async (t) => {
  const calls = [];
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-launcher-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const binaryPath = path.join(root, 'agent-notify');

  const exitCode = await main(['doctor'], {
    getDesiredVersion: () => '0.2.3',
    getPlatformTarget: () => ({ goos: 'linux', goarch: 'amd64', ext: '' }),
    getInstalledBinaryPath: () => binaryPath,
    getInstalledVersion: () => null,
    downloadAndInstall: async () => {
      calls.push('download');
      fs.writeFileSync(binaryPath, 'binary');
      return binaryPath;
    },
    runBinary: async (targetPath, args) => {
      calls.push(['run', targetPath, args]);
      return 0;
    },
    pathExists: (value) => fs.existsSync(value),
    warn: () => {},
  });

  assert.equal(exitCode, 0);
  assert.deepEqual(calls, ['download', ['run', binaryPath, ['doctor']]]);
});

test('reuses installed binary when version is current', async () => {
  const calls = [];

  const exitCode = await main(['doctor'], {
    getDesiredVersion: () => '0.2.3',
    getPlatformTarget: () => ({ goos: 'linux', goarch: 'amd64', ext: '' }),
    getInstalledBinaryPath: () => '/tmp/agent-notify',
    getInstalledVersion: () => '0.2.3',
    downloadAndInstall: async () => {
      throw new Error('should not download');
    },
    runBinary: async (targetPath, args) => {
      calls.push(['run', targetPath, args]);
      return 0;
    },
    pathExists: () => true,
    compareVersions: (left, right) => (left === right ? 0 : -1),
    warn: () => {},
  });

  assert.equal(exitCode, 0);
  assert.deepEqual(calls, [['run', '/tmp/agent-notify', ['doctor']]]);
});

// 精确 pin:npx agent-notify@0.2.3 装的就该是 0.2.3。旧的「只在 installed <
// desired 时安装」让显式降级静默失效,用户拿到的仍是那个更新的二进制。
test('reinstalls when the installed binary is newer than the requested version', async () => {
  const calls = [];

  const exitCode = await main(['doctor'], {
    getDesiredVersion: () => '0.2.3',
    getPlatformTarget: () => ({ goos: 'linux', goarch: 'amd64', ext: '' }),
    getInstalledBinaryPath: () => '/tmp/agent-notify',
    getInstalledVersion: () => '0.2.4',
    downloadAndInstall: async (version) => {
      calls.push(['download', version]);
      return '/tmp/agent-notify';
    },
    runBinary: async (targetPath, args) => {
      calls.push(['run', targetPath, args]);
      return 0;
    },
    pathExists: () => true,
    warn: () => {},
  });

  assert.equal(exitCode, 0);
  assert.deepEqual(calls, [['download', '0.2.3'], ['run', '/tmp/agent-notify', ['doctor']]]);
});

// extractSemver 若不捕获 prerelease 后缀,已装的 0.15.0-beta.1 会被读成
// 0.15.0,与 desired 永远不等 —— 每次调用都重新下载一遍。
test('does not reinstall on every call when the pinned version is a prerelease', async () => {
  const calls = [];

  const exitCode = await main(['doctor'], {
    getDesiredVersion: () => '0.15.0-beta.1',
    getPlatformTarget: () => ({ goos: 'linux', goarch: 'amd64', ext: '' }),
    getInstalledBinaryPath: () => '/tmp/agent-notify',
    getInstalledVersion: () => '0.15.0-beta.1',
    downloadAndInstall: async () => {
      throw new Error('should not download');
    },
    runBinary: async (targetPath, args) => {
      calls.push(['run', targetPath, args]);
      return 0;
    },
    pathExists: () => true,
    warn: () => {},
  });

  assert.equal(exitCode, 0);
  assert.deepEqual(calls, [['run', '/tmp/agent-notify', ['doctor']]]);
});

// 旧的 compareVersions 对 '0.15.0-beta.1' 得到 NaN,比较恒返回 0(相等),
// 于是从 beta 升到正式版这一步整个静默跳过。
test('installs when moving from a prerelease to the matching release', async () => {
  const calls = [];

  const exitCode = await main(['doctor'], {
    getDesiredVersion: () => '0.15.0',
    getPlatformTarget: () => ({ goos: 'linux', goarch: 'amd64', ext: '' }),
    getInstalledBinaryPath: () => '/tmp/agent-notify',
    getInstalledVersion: () => '0.15.0-beta.1',
    downloadAndInstall: async (version) => {
      calls.push(['download', version]);
      return '/tmp/agent-notify';
    },
    runBinary: async (targetPath, args) => {
      calls.push(['run', targetPath, args]);
      return 0;
    },
    pathExists: () => true,
    warn: () => {},
  });

  assert.equal(exitCode, 0);
  assert.deepEqual(calls, [['download', '0.15.0'], ['run', '/tmp/agent-notify', ['doctor']]]);
});

test('runs the installed binary with a warning when package.json version is unusable', async () => {
  const warnings = [];
  const calls = [];

  const exitCode = await main(['doctor'], {
    getDesiredVersion: () => 'latest',
    getPlatformTarget: () => ({ goos: 'linux', goarch: 'amd64', ext: '' }),
    getInstalledBinaryPath: () => '/tmp/agent-notify',
    getInstalledVersion: () => '0.2.3',
    downloadAndInstall: async () => {
      throw new Error('should not attempt a doomed download');
    },
    runBinary: async (targetPath, args) => {
      calls.push(['run', targetPath, args]);
      return 0;
    },
    pathExists: () => true,
    warn: (message) => warnings.push(message),
  });

  assert.equal(exitCode, 0);
  assert.deepEqual(calls, [['run', '/tmp/agent-notify', ['doctor']]]);
  assert.equal(warnings.length, 1);
  assert.match(warnings[0], /unusable package version "latest"/);
});

test('throws when package.json version is unusable and no binary is installed', async () => {
  await assert.rejects(
    main(['doctor'], {
      getDesiredVersion: () => 'latest',
      getPlatformTarget: () => ({ goos: 'linux', goarch: 'amd64', ext: '' }),
      getInstalledBinaryPath: () => '/tmp/agent-notify',
      getInstalledVersion: () => null,
      downloadAndInstall: async () => {
        throw new Error('should not attempt a doomed download');
      },
      runBinary: async () => 0,
      pathExists: () => false,
      warn: () => {},
    }),
    /unusable version "latest"/,
  );
});

test('falls back to old binary when download fails but installed binary exists', async () => {
  const warnings = [];
  const calls = [];

  const exitCode = await main(['doctor'], {
    getDesiredVersion: () => '0.2.3',
    getPlatformTarget: () => ({ goos: 'linux', goarch: 'amd64', ext: '' }),
    getInstalledBinaryPath: () => '/tmp/agent-notify',
    getInstalledVersion: () => '0.2.2',
    compareVersions: () => -1,
    downloadAndInstall: async () => {
      throw new Error('network down');
    },
    runBinary: async (targetPath, args) => {
      calls.push(['run', targetPath, args]);
      return 0;
    },
    pathExists: () => true,
    warn: (message) => warnings.push(message),
  });

  assert.equal(exitCode, 0);
  assert.equal(warnings.length, 1);
  assert.match(warnings[0], /network down/);
  assert.deepEqual(calls, [['run', '/tmp/agent-notify', ['doctor']]]);
});

test('fails when download fails and no installed binary exists', async () => {
  await assert.rejects(
    main(['doctor'], {
      getDesiredVersion: () => '0.2.3',
      getPlatformTarget: () => ({ goos: 'linux', goarch: 'amd64', ext: '' }),
      getInstalledBinaryPath: () => '/tmp/agent-notify',
      getInstalledVersion: () => null,
      compareVersions: () => -1,
      downloadAndInstall: async () => {
        throw new Error('network down');
      },
      runBinary: async () => 0,
      pathExists: () => false,
      warn: () => {},
    }),
    /network down/,
  );
});

test('fails when installed binary is unreadable and update also fails', async () => {
  const warnings = [];

  await assert.rejects(
    main(['doctor'], {
      getDesiredVersion: () => '0.2.3',
      getPlatformTarget: () => ({ goos: 'linux', goarch: 'amd64', ext: '' }),
      getInstalledBinaryPath: () => '/tmp/agent-notify',
      getInstalledVersion: () => null,
      compareVersions: () => -1,
      downloadAndInstall: async () => {
        throw new Error('network down');
      },
      runBinary: async () => 0,
      pathExists: () => true,
      warn: (message) => warnings.push(message),
    }),
    /network down/,
  );

  assert.deepEqual(warnings, []);
});
