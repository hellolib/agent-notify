// 点阵铃铛生成器（栅格化版）：用数学定义铃铛轮廓，逐点采样到 N×N 点阵，输出 SVG。
// 造型参数集中在 GEOMETRY，配色集中在 COLORS —— 改数字即可调整，无需手摆像素。
const fs = require('node:fs');

// —— 背景模式（BG_MODE 环境变量切换）——
//  plate      深色圆角底板 + 熄灭底点（默认，App 图标用）
//  bare       去掉底板，保留熄灭底点（透明背景，底点仍是原来的深紫）
//  bare-soft  去掉底板，熄灭底点改中性半透明灰（浅色/深色页面都不抢戏）
//  bell       去掉底板与熄灭底点，只留铃铛本体
const BG_MODE = process.env.BG_MODE || 'plate';
const MODES = ['plate', 'bare', 'bare-soft', 'bell'];
if (!MODES.includes(BG_MODE)) {
  console.error(`unknown BG_MODE "${BG_MODE}", expected one of: ${MODES.join(', ')}`);
  process.exit(1);
}
const DRAW_PLATE = BG_MODE === 'plate';
const DRAW_OFF = BG_MODE !== 'bell';

// —— 配色 ——
const BG_TOP = '#211E3A', BG_BOT = '#151228'; // 底：深靛紫（围绕 #1B1930）
const V = '#8B7CFF', C = '#22D3EE';           // 铃铛：紫罗兰 → 青
// 熄灭底点：贴着底板时用深紫；底板拿掉后深紫在白底上会变成脏点，
// 故 bare-soft 换成中性灰 + 低透明度，靠 alpha 同时适配浅色与深色背景。
const OFF = BG_MODE === 'bare-soft' ? '#7C7A96' : '#2E2A4A';
const OFF_OPACITY = BG_MODE === 'bare-soft' ? 0.35 : 1;

const toRgb = (h) => ({ r: parseInt(h.slice(1, 3), 16), g: parseInt(h.slice(3, 5), 16), b: parseInt(h.slice(5, 7), 16) });
const toHex = ({ r, g, b }) => '#' + [r, g, b].map((v) => Math.round(Math.max(0, Math.min(255, v))).toString(16).padStart(2, '0')).join('');
const lerp = (a, b, t) => ({ r: a.r + (b.r - a.r) * t, g: a.g + (b.g - a.g) * t, b: a.b + (b.b - a.b) * t });
const WHITE = { r: 255, g: 255, b: 255 }, BLACK = { r: 0, g: 0, b: 0 };
const clamp01 = (t) => Math.max(0, Math.min(1, t));

// —— 几何（以调好的 32 格为基准，乘 U 等比放大到任意密度）——
const N = 32;                // 点阵密度
const U = N / 32;            // 单位缩放因子（=1）
const CX = (N - 1) / 2;      // 水平中心
const DOME_TOP = 5 * U;      // 圆顶顶端所在行（t=0）
const MOUTH = 25 * U;        // 钟口所在行（t=1）
const H = MOUTH - DOME_TOP;  // 铃身高度
const WBODY = 8.6 * U;       // 铃身/肩部半宽（竖直侧壁）
const WM = 12.6 * U;         // 钟口半宽（喇叭口）
const TC = 0.46;             // 圆顶占比（纵向分数，不随密度变）
const TF = 0.72;             // 钟口外扩起点
const FLARE_POW = 1.25;      // 钟口外扩曲率
const FILL_TOL = 0.6;        // 每点级采样阈值，不随密度缩放
const handle = { cy: 2.8 * U, r: 1.9 * U };   // 顶部提环
const clapper = { cy: 27.0 * U, r: 1.9 * U }; // 铃舌：半径 2.4→1.9（缩短约一行）并上收

// 半宽随纵向 t 的分布：三段式
//  圆顶[0,TC]  椭圆弧 → 顶端垂直切线，宽而圆
//  铃身[TC,TF] 等宽   → 竖直侧壁
//  钟口[TF,1]  幂曲线 → 外扩成喇叭口
function halfWidth(t) {
  if (t < 0 || t > 1) return -1;
  if (t <= TC) {
    const u = 0.12 + 0.88 * (t / TC);       // 起始偏移 → 顶部即宽圆冠，无细颈
    return WBODY * Math.sqrt(1 - (1 - u) * (1 - u));
  }
  if (t <= TF) return WBODY;
  const u = (t - TF) / (1 - TF);            // 0..1
  return WBODY + (WM - WBODY) * Math.pow(u, FLARE_POW);
}

const dist = (c, r, cx, cy) => Math.hypot(c - cx, r - cy);

// —— 采样：给每个格子定角色 ——
// 0=空 / 1=铃身 / 2=铃舌 / 3=手柄
const role = Array.from({ length: N }, () => new Array(N).fill(0));
for (let r = 0; r < N; r++) {
  const t = (r - DOME_TOP) / H;
  const hw = halfWidth(t);
  for (let c = 0; c < N; c++) {
    if (dist(c, r, CX, handle.cy) <= handle.r) role[r][c] = 3;
    else if (dist(c, r, CX, clapper.cy) <= clapper.r) role[r][c] = 2;
    else if (hw >= 0 && Math.abs(c - CX) <= hw + FILL_TOL) role[r][c] = 1;
  }
}

// 削掉铃舌最底行的两个角，避免方块感
const clapperCells = [];
for (let r = 0; r < N; r++) for (let c = 0; c < N; c++) if (role[r][c] === 2) clapperCells.push([r, c]);
if (clapperCells.length) {
  const maxR = Math.max(...clapperCells.map(([r]) => r));
  const bottom = clapperCells.filter(([r]) => r === maxR).map(([, c]) => c).sort((a, b) => a - b);
  if (bottom.length >= 2) { role[maxR][bottom[0]] = 0; role[maxR][bottom[bottom.length - 1]] = 0; }
}

// 每行最左/最右亮点用于提亮/压暗，做出体积感
const edges = role.map((row) => {
  const lit = row.map((v, c) => (v === 1 ? c : -1)).filter((c) => c >= 0);
  return lit.length ? { min: lit[0], max: lit[lit.length - 1] } : null;
});

const rowT = (r) => clamp01((r - DOME_TOP) / H);
const baseAt = (r) => lerp(toRgb(V), toRgb(C), rowT(r));

function colorFor(r, c) {
  if (role[r][c] === 2) return toHex(lerp(toRgb(C), BLACK, 0.12));    // 铃舌
  const base = baseAt(r);
  const e = edges[r];
  if (role[r][c] === 1 && e && c === e.min) return toHex(lerp(base, WHITE, 0.24));
  if (role[r][c] === 1 && e && c === e.max) return toHex(lerp(base, BLACK, 0.20));
  return toHex(base); // 铃身 / 手柄
}

// —— 输出 SVG ——
const CANVAS = 2048;
const P = CANVAS / (N + 2);         // 上下左右各留 1 格白边
const origin = (CANVAS - N * P) / 2;
const LIT_R = P * 0.42, OFF_R = P * 0.30;
const center = (i) => origin + i * P + P / 2;

const offAlpha = OFF_OPACITY < 1 ? ` fill-opacity="${OFF_OPACITY}"` : '';
let cells = '';
for (let r = 0; r < N; r++) {
  for (let c = 0; c < N; c++) {
    const cx = center(c), cy = center(r);
    if (role[r][c] === 0) {
      if (DRAW_OFF) cells += `  <circle cx="${cx.toFixed(1)}" cy="${cy.toFixed(1)}" r="${OFF_R.toFixed(1)}" fill="${OFF}"${offAlpha}/>\n`;
    } else cells += `  <circle cx="${cx.toFixed(1)}" cy="${cy.toFixed(1)}" r="${LIT_R.toFixed(1)}" fill="${colorFor(r, c)}"/>\n`;
  }
}

// 底板只在 plate 模式下输出；其余模式留空 → 透明背景
const plate = DRAW_PLATE
  ? `  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="${BG_TOP}"/>
      <stop offset="1" stop-color="${BG_BOT}"/>
    </linearGradient>
  </defs>
  <rect x="0" y="0" width="${CANVAS}" height="${CANVAS}" rx="${(CANVAS * 0.223).toFixed(0)}" fill="url(#bg)"/>
`
  : '';

const svg = `<svg width="${CANVAS}" height="${CANVAS}" viewBox="0 0 ${CANVAS} ${CANVAS}" xmlns="http://www.w3.org/2000/svg">
${plate}${cells}</svg>
`;

fs.writeFileSync(process.argv[2] || 'concept-dotmatrix-bell.svg', svg);
console.log('wrote', process.argv[2] || 'concept-dotmatrix-bell.svg', `(${N}x${N}, ${CANVAS}px, bg=${BG_MODE})`);
