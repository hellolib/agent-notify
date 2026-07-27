// SEMVER_RE 同时捕获 release 三元组与可选的 prerelease 后缀。
// 旧版只捕获 \d+\.\d+\.\d+,会把 v0.15.0-beta.1 削成 0.15.0——
// 「已装的」与「要装的」于是永远对不上,精确 pin 会陷入无限重装。
// 构建元数据(+build)不必处理:Makefile 的 VERSION 校验正则不接受 '+'。
const SEMVER_RE = /v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)/;

const NUMERIC_IDENTIFIER = /^\d+$/;

function extractSemver(output) {
  const match = String(output).match(SEMVER_RE);
  return match ? match[1] : null;
}

// splitVersion 把版本拆成 release 三元组与 prerelease 标识符列表。
// 数字位一律经 parseInt 兜底为 0,保证 compareVersions 永不产出 NaN——
// 旧实现正是因为 '0-beta' 经 Number() 变成 NaN,而 NaN 的每次比较都是
// false,才让 compareVersions 对两个不同的版本返回 0,自动更新随之静默失效。
function splitVersion(version) {
  const [core, ...rest] = String(version).split('-');
  const prerelease = rest.join('-');
  const parts = core.split('.').map((part) => Number.parseInt(part, 10));
  return {
    core: [parts[0] || 0, parts[1] || 0, parts[2] || 0],
    prerelease: prerelease ? prerelease.split('.') : [],
  };
}

// comparePrereleaseIdentifiers 实现 SemVer 2.0.0 §11 对单个标识符的规则:
// 纯数字按数值比较,其余按 ASCII 字典序,且数字标识符的优先级低于非数字。
function comparePrereleaseIdentifiers(left, right) {
  const leftNumeric = NUMERIC_IDENTIFIER.test(left);
  const rightNumeric = NUMERIC_IDENTIFIER.test(right);

  if (leftNumeric && rightNumeric) {
    const l = Number(left);
    const r = Number(right);
    if (l === r) return 0;
    return l < r ? -1 : 1;
  }
  if (leftNumeric) return -1;
  if (rightNumeric) return 1;
  if (left === right) return 0;
  return left < right ? -1 : 1;
}

// compareVersions 按 SemVer 2.0.0 §11 比较优先级,返回 -1 / 0 / 1。
//
// 关键性质:结果为 0 当且仅当两者是同一个版本。启动器的安装判定
// (cmp !== 0 就重装)完全建立在这条性质上。
function compareVersions(left, right) {
  const l = splitVersion(left);
  const r = splitVersion(right);

  for (let i = 0; i < 3; i += 1) {
    if (l.core[i] !== r.core[i]) {
      return l.core[i] < r.core[i] ? -1 : 1;
    }
  }

  // 带 prerelease 的版本低于 release 三元组相同的正式版本
  if (l.prerelease.length === 0 && r.prerelease.length === 0) return 0;
  if (l.prerelease.length === 0) return 1;
  if (r.prerelease.length === 0) return -1;

  const shared = Math.min(l.prerelease.length, r.prerelease.length);
  for (let i = 0; i < shared; i += 1) {
    const result = comparePrereleaseIdentifiers(l.prerelease[i], r.prerelease[i]);
    if (result !== 0) return result;
  }

  // 前缀完全相同时,标识符更多的一方优先级更高
  if (l.prerelease.length === r.prerelease.length) return 0;
  return l.prerelease.length < r.prerelease.length ? -1 : 1;
}

module.exports = {
  extractSemver,
  compareVersions,
};
