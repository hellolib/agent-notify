const test = require('node:test');
const assert = require('node:assert/strict');
const { resolveProxy } = require('../lib/proxy');

// proxy-from-env 直接读 process.env，测试只能就地替换后还原。
function withEnv(vars, fn) {
  const saved = {};
  const keys = [
    'https_proxy', 'HTTPS_PROXY', 'http_proxy', 'HTTP_PROXY',
    'all_proxy', 'ALL_PROXY', 'no_proxy', 'NO_PROXY',
    'npm_config_proxy', 'npm_config_https_proxy',
  ];
  for (const key of keys) {
    saved[key] = process.env[key];
    delete process.env[key];
  }
  Object.assign(process.env, vars);
  try {
    return fn();
  } finally {
    for (const key of keys) {
      if (saved[key] === undefined) delete process.env[key];
      else process.env[key] = saved[key];
    }
  }
}

const TARGET = 'https://github.com/hellolib/agent-notify/releases/download/v1.0.0/x.tar.gz';

test('returns empty when nothing is configured', () => {
  withEnv({}, () => {
    assert.equal(resolveProxy(TARGET), '');
  });
});

test('uses HTTPS_PROXY', () => {
  withEnv({ HTTPS_PROXY: 'http://corp:8080' }, () => {
    assert.equal(resolveProxy(TARGET), 'http://corp:8080');
  });
});

test('uses the lowercase form too', () => {
  withEnv({ https_proxy: 'http://corp:8080' }, () => {
    assert.equal(resolveProxy(TARGET), 'http://corp:8080');
  });
});

// NO_PROXY 排除 github 的用户此前是直连的，能装上。强制走代理会把一个
// 本来可用的场景改坏，所以这个排除必须被尊重。
test('honours NO_PROXY', () => {
  withEnv({ HTTPS_PROXY: 'http://corp:8080', NO_PROXY: 'github.com' }, () => {
    assert.equal(resolveProxy(TARGET), '');
  });
});

test('honours a wildcard NO_PROXY', () => {
  withEnv({ HTTPS_PROXY: 'http://corp:8080', NO_PROXY: '*' }, () => {
    assert.equal(resolveProxy(TARGET), '');
  });
});

// npx 用户最常见的配代理方式是 `npm config set https-proxy`，写进 .npmrc，
// npm 把它作为 npm_config_* 注入子进程。proxy-from-env 不读这批变量。
test('falls back to npm config when no standard env var is set', () => {
  withEnv({ npm_config_https_proxy: 'http://npmrc:3128' }, () => {
    assert.equal(resolveProxy(TARGET), 'http://npmrc:3128');
  });
});

test('accepts the generic npm_config_proxy', () => {
  withEnv({ npm_config_proxy: 'http://npmrc:3128' }, () => {
    assert.equal(resolveProxy(TARGET), 'http://npmrc:3128');
  });
});

test('prefers npm_config_https_proxy over npm_config_proxy', () => {
  withEnv({ npm_config_proxy: 'http://a:1', npm_config_https_proxy: 'http://b:2' }, () => {
    assert.equal(resolveProxy(TARGET), 'http://b:2');
  });
});

// 关键歧义：getProxyForUrl 对「没配」和「被 NO_PROXY 排除」都返回空串。
// 若无条件回落到 npm 配置，NO_PROXY 就会被悄悄绕过。
test('does not let the npm fallback bypass NO_PROXY', () => {
  withEnv({
    HTTPS_PROXY: 'http://corp:8080',
    NO_PROXY: 'github.com',
    npm_config_https_proxy: 'http://npmrc:3128',
  }, () => {
    assert.equal(resolveProxy(TARGET), '', 'NO_PROXY 命中时不应回落到 npm 配置');
  });
});

test('standard env wins over npm config', () => {
  withEnv({ HTTPS_PROXY: 'http://corp:8080', npm_config_https_proxy: 'http://npmrc:3128' }, () => {
    assert.equal(resolveProxy(TARGET), 'http://corp:8080');
  });
});

// 真实环境里大小写两种写法常常同时存在（shell profile 设一个、桌面环境设另一个）。
// proxy-from-env 的 getEnv 是 lowercase || uppercase，小写优先——不知道这点
// 会以为设了 NO_PROXY 却不生效。固化下来免得日后重新踩。
test('lowercase env vars take precedence over uppercase', () => {
  withEnv({ https_proxy: 'http://lower:1', HTTPS_PROXY: 'http://upper:2' }, () => {
    assert.equal(resolveProxy(TARGET), 'http://lower:1');
  });

  withEnv({ HTTPS_PROXY: 'http://corp:8080', no_proxy: 'github.com', NO_PROXY: 'nothing.invalid' }, () => {
    assert.equal(resolveProxy(TARGET), '', '小写 no_proxy 应当胜出并排除 github.com');
  });
});

test('an empty proxy value is treated as unset', () => {
  withEnv({ HTTPS_PROXY: '', npm_config_https_proxy: 'http://npmrc:3128' }, () => {
    assert.equal(resolveProxy(TARGET), 'http://npmrc:3128');
  });
});
