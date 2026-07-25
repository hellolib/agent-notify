const test = require('node:test');
const assert = require('node:assert/strict');
const { rewriteAssetPaths } = require('../scripts/sync-readme');

const RAW = 'https://raw.githubusercontent.com/hellolib/agent-notify/main/';

test('rewrites markdown image paths to absolute raw URLs', () => {
  const out = rewriteAssetPaths('![demo](assist/demo.gif)');
  assert.ok(out.includes(`${RAW}assist/demo.gif`));
  assert.ok(!out.includes('](assist/'));
});

test('rewrites HTML src/href assist paths', () => {
  const out = rewriteAssetPaths('<img src="assist/logo/feishu.png"> <a href="assist/x.png">');
  assert.ok(out.includes(`src="${RAW}assist/logo/feishu.png"`));
  assert.ok(out.includes(`href="${RAW}assist/x.png"`));
});

test('rewrites the zh-CN readme link to absolute blob URL', () => {
  const out = rewriteAssetPaths('<a href="README.zh-CN.md">简体中文</a>');
  assert.ok(out.includes('https://github.com/hellolib/agent-notify/blob/main/README.zh-CN.md'));
});

test('leaves absolute URLs untouched', () => {
  const url = 'https://example.com/assist/demo.gif';
  assert.equal(rewriteAssetPaths(`![x](${url})`), `![x](${url})`);
});
