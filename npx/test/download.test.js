const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { Readable } = require('node:stream');
const { downloadToFile } = require('../lib/download');

function tempFile(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-notify-download-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return path.join(dir, 'asset.tar.gz');
}

// fakeClient 记录每次 get 的参数，并回放一个可控的响应。
function fakeClient(captured, respond) {
  return {
    get(url, options, cb) {
      captured.push({ url, options });
      const request = {
        handlers: {},
        on(event, handler) {
          this.handlers[event] = handler;
          return this;
        },
        destroy() {},
      };
      process.nextTick(() => respond(cb, request));
      return request;
    },
  };
}

function okResponse(body = 'payload') {
  const res = Readable.from([body]);
  res.statusCode = 200;
  res.headers = {};
  return res;
}

function withEnv(vars, fn) {
  const keys = ['https_proxy', 'HTTPS_PROXY', 'http_proxy', 'HTTP_PROXY', 'no_proxy', 'NO_PROXY',
    'all_proxy', 'ALL_PROXY', 'npm_config_proxy', 'npm_config_https_proxy'];
  const saved = {};
  for (const key of keys) {
    saved[key] = process.env[key];
    delete process.env[key];
  }
  Object.assign(process.env, vars);
  return Promise.resolve()
    .then(fn)
    .finally(() => {
      for (const key of keys) {
        if (saved[key] === undefined) delete process.env[key];
        else process.env[key] = saved[key];
      }
    });
}

test('sends no agent when no proxy is configured', async (t) => {
  const dest = tempFile(t);
  const captured = [];
  const client = fakeClient(captured, (cb) => cb(okResponse()));

  await withEnv({}, () => downloadToFile('https://example.invalid/a.tar.gz', dest, client));

  assert.equal(captured.length, 1);
  assert.equal(captured[0].options.agent, undefined);
});

// 裸 https.get 从不读环境变量代理，企业代理后面的用户只会等到一个 300s 超时。
test('attaches a proxy agent when HTTPS_PROXY is set', async (t) => {
  const dest = tempFile(t);
  const captured = [];
  const client = fakeClient(captured, (cb) => cb(okResponse()));

  await withEnv({ HTTPS_PROXY: 'http://corp:8080' },
    () => downloadToFile('https://example.invalid/a.tar.gz', dest, client));

  assert.ok(captured[0].options.agent, 'proxy agent should be attached');
  assert.equal(captured[0].options.agent.proxy.hostname, 'corp');
  assert.equal(captured[0].options.agent.proxy.port, '8080');
});

test('attaches no agent when NO_PROXY excludes the host', async (t) => {
  const dest = tempFile(t);
  const captured = [];
  const client = fakeClient(captured, (cb) => cb(okResponse()));

  await withEnv({ HTTPS_PROXY: 'http://corp:8080', NO_PROXY: 'example.invalid' },
    () => downloadToFile('https://example.invalid/a.tar.gz', dest, client));

  assert.equal(captured[0].options.agent, undefined);
});

test('keeps 404 distinguishable so a missing checksum manifest can be tolerated', async (t) => {
  const dest = tempFile(t);
  const client = fakeClient([], (cb) => {
    const res = Readable.from([]);
    res.statusCode = 404;
    res.headers = {};
    cb(res);
  });

  await assert.rejects(
    withEnv({}, () => downloadToFile('https://example.invalid/a.tar.gz', dest, client)),
    (err) => {
      assert.equal(err.statusCode, 404);
      return true;
    },
  );
});

// 407 来自代理本身而不是 GitHub，报错必须把用户指向正确的地方。
test('explains a 407 as proxy authentication', async (t) => {
  const dest = tempFile(t);
  const client = fakeClient([], (cb) => {
    const res = Readable.from([]);
    res.statusCode = 407;
    res.headers = {};
    cb(res);
  });

  await assert.rejects(
    withEnv({ HTTPS_PROXY: 'http://corp:8080' },
      () => downloadToFile('https://example.invalid/a.tar.gz', dest, client)),
    /proxy requires authentication/,
  );
});

test('names the proxy in a connection failure', async (t) => {
  const dest = tempFile(t);
  const client = fakeClient([], (cb, request) => {
    request.handlers.error(new Error('ECONNREFUSED'));
  });

  await assert.rejects(
    withEnv({ HTTPS_PROXY: 'http://corp:8080' },
      () => downloadToFile('https://example.invalid/a.tar.gz', dest, client)),
    /via proxy http:\/\/corp:8080/,
  );
});

test('suggests configuring a proxy when a connection fails without one', async (t) => {
  const dest = tempFile(t);
  const client = fakeClient([], (cb, request) => {
    request.handlers.error(new Error('ETIMEDOUT'));
  });

  await assert.rejects(
    withEnv({}, () => downloadToFile('https://example.invalid/a.tar.gz', dest, client)),
    /set HTTPS_PROXY/,
  );
});

// 重定向可能跨主机，而 NO_PROXY 是按主机匹配的，所以每一跳都要重新解析。
test('re-resolves the proxy on each redirect hop', async (t) => {
  const dest = tempFile(t);
  const captured = [];
  const client = {
    get(url, options, cb) {
      captured.push({ url, agent: options.agent });
      const request = { on() { return this; }, destroy() {} };
      process.nextTick(() => {
        if (url.includes('start')) {
          const res = Readable.from([]);
          res.statusCode = 302;
          res.headers = { location: 'https://cdn.invalid/final.tar.gz' };
          cb(res);
        } else {
          cb(okResponse());
        }
      });
      return request;
    },
  };

  await withEnv({ HTTPS_PROXY: 'http://corp:8080', NO_PROXY: 'cdn.invalid' },
    () => downloadToFile('https://start.invalid/start.tar.gz', dest, client));

  assert.equal(captured.length, 2);
  assert.ok(captured[0].agent, 'first hop goes through the proxy');
  assert.equal(captured[1].agent, undefined, 'redirect target is excluded by NO_PROXY');
});
