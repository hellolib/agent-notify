const { getProxyForUrl } = require('proxy-from-env');

// proxy-from-env 读取的标准变量。它对「压根没配代理」和「配了但被 NO_PROXY
// 排除」都返回空串,靠这份清单把两者区分开。
const STANDARD_PROXY_ENV = [
  'https_proxy', 'HTTPS_PROXY',
  'http_proxy', 'HTTP_PROXY',
  'all_proxy', 'ALL_PROXY',
];

function hasStandardProxyEnv(env) {
  return STANDARD_PROXY_ENV.some((key) => Boolean(env[key]));
}

// resolveProxy 给出下载 url 时应当使用的代理地址,没有则返回 ''。
//
// 两级查找:
//  1. 标准环境变量,交给 proxy-from-env 处理,它同时实现 NO_PROXY 的后缀 /
//     通配 / 端口匹配语义——那套规则自己写极易出错。
//  2. npm 自身的配置。`.npmrc` 里的 proxy= / https-proxy= 由 npm 以
//     npm_config_* 注入子进程,而这正是企业用户最常配代理的地方;
//     proxy-from-env 不读这批变量。
//
// 第 2 级只在标准变量一个都没设时才生效:设了却被判空,唯一的解释是
// NO_PROXY 命中,那就该尊重这个排除,而不是绕过它去用 npm 的配置。
function resolveProxy(url, env = process.env) {
  const fromEnv = getProxyForUrl(url);
  if (fromEnv) return fromEnv;
  if (hasStandardProxyEnv(env)) return '';
  return env.npm_config_https_proxy || env.npm_config_proxy || '';
}

module.exports = {
  resolveProxy,
};
