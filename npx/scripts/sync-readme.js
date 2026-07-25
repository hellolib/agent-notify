const fs = require('node:fs');
const path = require('node:path');

const RAW_BASE = 'https://raw.githubusercontent.com/hellolib/agent-notify/main/';
const BLOB_BASE = 'https://github.com/hellolib/agent-notify/blob/main/';

// rewriteAssetPaths converts repo-relative asset references into absolute URLs
// so images and doc links resolve on the npm package page.
function rewriteAssetPaths(md) {
  return md
    // markdown ![alt](assist/...) and [text](assist/...)
    .replace(/(\]\()(assist\/)/g, `$1${RAW_BASE}$2`)
    // HTML src="assist/..." / href="assist/..."
    .replace(/((?:src|href)=")(assist\/)/g, `$1${RAW_BASE}$2`)
    // the sibling Chinese README link (markdown + HTML)
    .replace(/(\]\()(README\.zh-CN\.md)/g, `$1${BLOB_BASE}$2`)
    .replace(/((?:src|href)=")(README\.zh-CN\.md)/g, `$1${BLOB_BASE}$2`);
}

function syncReadme(srcPath, destPath) {
  const md = fs.readFileSync(srcPath, 'utf8');
  fs.writeFileSync(destPath, rewriteAssetPaths(md));
}

module.exports = { rewriteAssetPaths, syncReadme };

if (require.main === module) {
  const src = path.join(__dirname, '..', '..', 'README.md');
  const dest = path.join(__dirname, '..', 'README.md');
  syncReadme(src, dest);
  console.log(`Synced README: ${src} -> ${dest}`);
}
