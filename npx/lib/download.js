const fs = require('node:fs');
const https = require('node:https');
const { HttpsProxyAgent } = require('https-proxy-agent');
const { resolveProxy } = require('./proxy');

const TIMEOUT_MS = 300000;

function removeFileIfExists(filePath, callback) {
  fs.rm(filePath, { force: true }, () => callback());
}

// describeProxy 在报错信息里点出当前走的是哪个代理,或提示可以配一个。
// 企业代理后面的用户此前只会看到一个 300 秒超时,既不知道原因也没有出路。
function describeProxy(proxy) {
  if (proxy) {
    return `\n  via proxy ${proxy} — check the proxy is reachable, or set NO_PROXY to bypass it`;
  }
  return '\n  no proxy configured — behind a corporate proxy, set HTTPS_PROXY or run `npm config set https-proxy <url>`';
}

function downloadToFile(url, destinationPath, client = https) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(destinationPath);

    // 每次调用重新解析:重定向可能跨主机,而 NO_PROXY 是按主机匹配的
    const proxy = resolveProxy(url);
    const options = { timeout: TIMEOUT_MS };
    if (proxy) {
      options.agent = new HttpsProxyAgent(proxy);
    }

    const request = client.get(url, options, (response) => {
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        const redirectUrl = new URL(response.headers.location, url).toString();
        file.close(() => {
          removeFileIfExists(destinationPath, () => {
            downloadToFile(redirectUrl, destinationPath, client).then(resolve, reject);
          });
        });
        return;
      }

      if (response.statusCode !== 200) {
        const statusCode = response.statusCode;
        file.close(() => {
          removeFileIfExists(destinationPath, () => {
            // 407 来自代理本身而不是 GitHub:分开说,否则用户会去查错地方
            const detail = statusCode === 407
              ? '\n  the proxy requires authentication — use HTTPS_PROXY=http://user:pass@host:port'
              : '';
            const err = new Error(`download failed: ${statusCode} ${url}${detail}`);
            // 调用方据此区分「该 release 没有这个文件」(404) 与网络/服务端故障
            err.statusCode = statusCode;
            reject(err);
          });
        });
        return;
      }

      response.pipe(file);
      file.on('finish', () => file.close(() => resolve(destinationPath)));
    });

    request.on('error', (err) => {
      file.close(() => {
        removeFileIfExists(destinationPath, () => {
          err.message = `download failed: ${err.message} ${url}${describeProxy(proxy)}`;
          reject(err);
        });
      });
    });

    request.on('timeout', () => {
      request.destroy();
      file.close(() => {
        removeFileIfExists(destinationPath, () => {
          reject(new Error(
            `download timeout after ${TIMEOUT_MS / 1000}s: ${url}${describeProxy(proxy)}`,
          ));
        });
      });
    });
  });
}

module.exports = {
  downloadToFile,
};
