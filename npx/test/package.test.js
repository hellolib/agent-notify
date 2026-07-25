const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const pkg = JSON.parse(
  fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'),
);

test('package.json exposes GitHub metadata for npm page', () => {
  assert.equal(pkg.license, 'MIT');
  assert.equal(pkg.author, 'hellolib');
  assert.equal(pkg.homepage, 'https://github.com/hellolib/agent-notify#readme');
  assert.equal(pkg.repository.type, 'git');
  assert.match(pkg.repository.url, /github\.com\/hellolib\/agent-notify/);
  assert.equal(pkg.bugs.url, 'https://github.com/hellolib/agent-notify/issues');
});

test('package.json has discoverable keywords', () => {
  assert.ok(Array.isArray(pkg.keywords));
  for (const kw of ['claude-code', 'codex', 'notifications', 'cli']) {
    assert.ok(pkg.keywords.includes(kw), `missing keyword: ${kw}`);
  }
});

test('README is published in the npm tarball', () => {
  assert.ok(pkg.files.includes('README.md'));
});
