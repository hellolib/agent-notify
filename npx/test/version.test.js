const test = require('node:test');
const assert = require('node:assert/strict');
const { extractSemver, compareVersions } = require('../lib/version');

test('extracts plain semver', () => {
  assert.equal(extractSemver('v0.2.3\n'), '0.2.3');
});

test('extracts semver from prefixed output', () => {
  assert.equal(extractSemver('agent-notify version v1.4.0\n'), '1.4.0');
});

test('returns null when output has no semver', () => {
  assert.equal(extractSemver('development build\n'), null);
});

// 不捕获 prerelease 后缀会让已装的 0.15.0-beta.1 被读成 0.15.0,
// 与 package.json 的 0.15.0-beta.1 永远不等 —— 每次调用都重下一遍。
test('keeps the prerelease suffix', () => {
  assert.equal(extractSemver('agent-notify version v0.15.0-beta.1\n'), '0.15.0-beta.1');
  assert.equal(extractSemver('v1.0.0-alpha\n'), '1.0.0-alpha');
  assert.equal(extractSemver('v1.0.0-rc.2\n'), '1.0.0-rc.2');
  assert.equal(extractSemver('v1.0.0-x-y-z.1\n'), '1.0.0-x-y-z.1');
});

test('compares versions correctly', () => {
  assert.equal(compareVersions('0.2.3', '0.2.3'), 0);
  assert.equal(compareVersions('0.2.2', '0.2.3'), -1);
  assert.equal(compareVersions('0.3.0', '0.2.9'), 1);
});

// SemVer 2.0.0 §11 给出的官方优先级链，原样用作测试向量。
test('orders the SemVer 2.0.0 precedence chain from the spec', () => {
  const chain = [
    '1.0.0-alpha',
    '1.0.0-alpha.1',
    '1.0.0-alpha.beta',
    '1.0.0-beta',
    '1.0.0-beta.2',
    '1.0.0-beta.11',
    '1.0.0-rc.1',
    '1.0.0',
  ];

  for (let i = 0; i < chain.length; i += 1) {
    for (let j = 0; j < chain.length; j += 1) {
      const expected = i === j ? 0 : (i < j ? -1 : 1);
      assert.equal(
        compareVersions(chain[i], chain[j]),
        expected,
        `compareVersions(${chain[i]}, ${chain[j]}) should be ${expected}`,
      );
    }
  }
});

// 启动器的安装判定就是 cmp !== 0，所以这条性质是整个 gate 的地基。
test('returns 0 only for identical versions', () => {
  const versions = [
    '0.14.3', '0.15.0', '0.15.1', '1.0.0',
    '0.15.0-beta.1', '0.15.0-beta.2', '0.15.0-rc.1', '0.15.0-alpha',
  ];

  for (const left of versions) {
    for (const right of versions) {
      const result = compareVersions(left, right);
      assert.equal(
        result === 0,
        left === right,
        `compareVersions(${left}, ${right}) = ${result}`,
      );
    }
  }
});

// 旧实现对 '0.15.0-beta.1' 得到 [0, 15, NaN, 1]；NaN 的每次比较都是 false，
// 于是循环走完返回 0 —— 两个不同的版本被判为相等，自动更新静默失效。
test('never returns 0 for a prerelease versus its release', () => {
  assert.equal(compareVersions('0.15.0-beta.1', '0.15.0'), -1);
  assert.equal(compareVersions('0.15.0', '0.15.0-beta.1'), 1);
});

test('compares numeric prerelease identifiers numerically, not lexically', () => {
  // 字典序会判定 beta.11 < beta.2，规范要求按数值比较
  assert.equal(compareVersions('1.0.0-beta.11', '1.0.0-beta.2'), 1);
  assert.equal(compareVersions('1.0.0-alpha.9', '1.0.0-alpha.10'), -1);
});

test('ranks numeric identifiers below alphanumeric ones', () => {
  assert.equal(compareVersions('1.0.0-1', '1.0.0-alpha'), -1);
  assert.equal(compareVersions('1.0.0-alpha', '1.0.0-1'), 1);
});

test('ranks a longer identifier set higher when the prefix matches', () => {
  assert.equal(compareVersions('1.0.0-alpha', '1.0.0-alpha.1'), -1);
  assert.equal(compareVersions('1.0.0-alpha.1', '1.0.0-alpha'), 1);
});

test('is total: malformed input never yields NaN-driven false equality', () => {
  // 缺位补 0，保证结果始终落在 -1 / 0 / 1
  for (const [left, right] of [['1', '1.0.0'], ['1.2', '1.2.0'], ['', '0.0.0']]) {
    assert.equal(compareVersions(left, right), 0);
  }
  assert.equal(compareVersions('2', '1.9.9'), 1);
});
