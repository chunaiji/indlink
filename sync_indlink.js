#!/usr/bin/env node
'use strict';
// 把本仓库当前分支的代码快照推送到镜像仓库 indlink(只推不拉,不在上面改)。
//
//   node sync_indlink.js              # 同步当前 HEAD
//   node sync_indlink.js main         # 同步指定分支/commit
//   node sync_indlink.js --dry-run    # 只打印差异,不推送
//
// 做法:用临时索引把 HEAD 的树剔掉 EXCLUDE 里的路径,再 commit-tree 挂到
// indlink/main 上追加一个提交。indlink 的历史与本仓库无关,每次同步 = 一个快照提交,
// 永远 fast-forward,不改写历史、不 force push。
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const REMOTE = 'indlink';
const REMOTE_URL = 'git@github.com:chunaiji/indlink.git';
const REMOTE_BRANCH = 'main';
// 不进镜像仓库的路径:国内版 App 单独维护不外发;漂流瓶/ 是素材草稿目录
const EXCLUDE = ['app/bottles_zh', '漂流瓶'];

function git(args, opts = {}) {
  const out = execFileSync('git', args, {
    cwd: __dirname,
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
    ...opts,
  });
  return out === null ? '' : out.trim(); // stdio:'inherit' 时 stdout 为 null
}

function main() {
  const argv = process.argv.slice(2);
  const dryRun = argv.includes('--dry-run');
  const source = argv.find((a) => !a.startsWith('-')) || 'HEAD';

  // 1. 确保 remote 存在且指向正确地址
  let remotes = '';
  try { remotes = git(['remote']); } catch (e) {
    console.error('[ERROR] 不是 git 仓库?', e.message); process.exit(1);
  }
  if (!remotes.split(/\r?\n/).includes(REMOTE)) {
    console.log(`[INFO] 添加 remote ${REMOTE} -> ${REMOTE_URL}`);
    git(['remote', 'add', REMOTE, REMOTE_URL]);
  } else {
    const url = git(['remote', 'get-url', REMOTE]);
    if (url !== REMOTE_URL) {
      console.log(`[INFO] 更新 remote ${REMOTE}: ${url} -> ${REMOTE_URL}`);
      git(['remote', 'set-url', REMOTE, REMOTE_URL]);
    }
  }

  // 2. 解析本地源提交
  const srcSha = git(['rev-parse', '--verify', `${source}^{commit}`]);
  const srcShort = srcSha.slice(0, 8);
  const srcSubject = git(['log', '-1', '--format=%s', srcSha]);
  const srcBranch = (() => {
    try { return git(['rev-parse', '--abbrev-ref', source === 'HEAD' ? 'HEAD' : source]); }
    catch (e) { return source; }
  })();
  const dirty = git(['status', '--porcelain', '--untracked-files=no']);
  if (dirty) {
    console.log('[WARN] 工作区有未提交改动,本次同步只包含已提交内容:');
    console.log(dirty.split(/\r?\n/).slice(0, 10).map((l) => '       ' + l).join('\n'));
  }

  // 3. 拉取镜像仓库当前状态
  console.log(`[INFO] fetch ${REMOTE}/${REMOTE_BRANCH} ...`);
  git(['fetch', '--quiet', REMOTE, REMOTE_BRANCH], { stdio: ['ignore', 'inherit', 'inherit'] });
  const parent = git(['rev-parse', 'FETCH_HEAD']);
  const parentTree = git(['rev-parse', `${parent}^{tree}`]);

  // 4. 用临时索引构造「HEAD 的树减去 EXCLUDE」
  const idxFile = path.join(os.tmpdir(), `indlink-index-${process.pid}`);
  const env = { ...process.env, GIT_INDEX_FILE: idxFile };
  let tree;
  try {
    git(['read-tree', srcSha], { env });
    for (const p of EXCLUDE) {
      git(['rm', '--cached', '-r', '-f', '-q', '--ignore-unmatch', '--', p], { env });
    }
    tree = git(['write-tree'], { env });
  } finally {
    if (fs.existsSync(idxFile)) fs.unlinkSync(idxFile);
  }

  // 5. 没变化就不推
  if (tree === parentTree) {
    console.log(`[OK] ${REMOTE}/${REMOTE_BRANCH} 已是最新(与 ${srcShort} 内容一致),无需推送。`);
    return;
  }

  const stat = git(['diff', '--stat', parentTree, tree]);
  console.log(`[INFO] 相对 ${REMOTE}/${REMOTE_BRANCH} 的差异:`);
  console.log(stat ? stat.split(/\r?\n/).slice(-12).map((l) => '       ' + l).join('\n') : '       (无)');

  if (dryRun) {
    console.log('[DRY-RUN] 到此为止,未创建提交、未推送。');
    return;
  }

  // 6. 追加一个快照提交并推送
  const message =
    `sync: ${srcBranch} @ ${srcShort} — ${srcSubject}\n\n` +
    `Snapshot of ai-message ${srcBranch} (${srcSha}).\n` +
    `Excluded: ${EXCLUDE.join(', ')}\n`;
  const commit = git(['commit-tree', tree, '-p', parent, '-m', message]);
  console.log(`[INFO] push ${commit.slice(0, 8)} -> ${REMOTE}/${REMOTE_BRANCH}`);
  git(['push', REMOTE, `${commit}:refs/heads/${REMOTE_BRANCH}`], {
    stdio: ['ignore', 'inherit', 'inherit'],
  });
  console.log(`[OK] 已同步 ${srcShort} 到 ${REMOTE_URL} (${REMOTE_BRANCH})`);
}

main();
