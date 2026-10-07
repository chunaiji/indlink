#!/usr/bin/env node
'use strict';
const { Client } = require('ssh2');
const fs = require('fs');
const path = require('path');
const os = require('os');

const HOST = '175.178.182.166'; // 旧机 43.136.54.189 已迁移废弃(2026-07-31)
const USER = 'root';
const SSH_KEY = path.join(os.homedir(), '.ssh', 'huawei_app_ed25519');
const DEPLOY = '/usr/jack/deploy/go_workspace';
const PORT_APP = 8980;
const BASE_URL = 'https://ambertu.com/message';

// 密钥从 deploy.local.json(已 gitignore)或同名环境变量读取,不写入仓库。
const SECRETS = loadSecrets();

function loadSecrets() {
  const need = ['MYSQL_DSN', 'REDIS_URL', 'JWT_SECRET', 'CRED_MASTER_KEY'];
  let fromFile = {};
  const p = path.join(__dirname, 'deploy.local.json');
  if (fs.existsSync(p)) {
    try { fromFile = JSON.parse(fs.readFileSync(p, 'utf8')); } catch (e) {
      console.error('[ERROR] deploy.local.json 解析失败:', e.message); process.exit(1);
    }
  }
  const s = {};
  const missing = [];
  for (const k of need) {
    s[k] = process.env[k] || fromFile[k];
    if (!s[k]) missing.push(k);
  }
  if (missing.length) {
    console.error(`[ERROR] 缺少部署密钥: ${missing.join(', ')}`);
    console.error('  请复制 deploy.local.json.example 为 deploy.local.json 并填写,或设置同名环境变量。');
    process.exit(1);
  }
  return s;
}

const ROOT = path.join(__dirname);
const BINARY = path.join(ROOT, 'server', 'driftbottle-linux');
const CERT_DIR = path.join(ROOT, 'server', 'cert');
const AVATAR_DIR = path.join(ROOT, 'server', 'header_image');
const AVATAR_DIR_MAN = path.join(ROOT, 'server', 'header_image_man');

const ENV = `APP_ENV=production
APP_PORT=${PORT_APP}
JWT_SECRET=${SECRETS.JWT_SECRET}
JWT_EXPIRE_HOURS=168

MYSQL_DSN=${SECRETS.MYSQL_DSN}

REDIS_URL=${SECRETS.REDIS_URL}

UPLOAD_DIR=./uploads
PUBLIC_BASE_URL=${BASE_URL}

MULTI_TENANT_ENABLED=1
DEFAULT_TENANT_ID=100
CRED_MASTER_KEY=${SECRETS.CRED_MASTER_KEY}

# Google/Apple 验签拉 JWKS 公钥的出海代理(国内服务器直连 googleapis 超时)。
# 走首尔中转,仅影响 JWKS 拉取,不碰微信/腾讯等国内接口(见 appauth.go jwksHTTPClient)。
JWKS_PROXY=http://119.28.239.151:3128

# App 端(Flutter)租户。App 只有一个包,租户是部署期常量,
# 设了它就不必往 app_credentials 加 platform=app 行。
# ⚠️ 这一段必须写在这里:本脚本每次部署都会整份覆盖远端 .env,
# 手动在服务器上加的变量会被冲掉。
APP_DEFAULT_TENANT_ID=358804313465688064
`;

const SYSTEMD = `[Unit]
Description=Driftbottle API Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${DEPLOY}
ExecStart=${DEPLOY}/driftbottle
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`;

function exec(conn, cmd) {
  return new Promise((resolve, reject) => {
    conn.exec(cmd, (err, stream) => {
      if (err) return reject(err);
      let out = '', errout = '';
      stream.on('data', d => out += d);
      stream.stderr.on('data', d => errout += d);
      stream.on('close', code => {
        const result = (out + errout).trim();
        if (result) console.log(`    ${result.replace(/\n/g, '\n    ')}`);
        resolve({ code, out, errout });
      });
    });
  });
}

function writeRemote(sftp, remotePath, content) {
  return new Promise((resolve, reject) => {
    const buf = Buffer.isBuffer(content) ? content : Buffer.from(content);
    sftp.open(remotePath, 'w', (err, fd) => {
      if (err) return reject(err);
      sftp.write(fd, buf, 0, buf.length, 0, err2 => {
        if (err2) return reject(err2);
        sftp.close(fd, resolve);
      });
    });
  });
}

function uploadFile(sftp, local, remote) {
  return new Promise((resolve, reject) => {
    const data = fs.readFileSync(local);
    const size = (data.length / 1024 / 1024).toFixed(1);
    process.stdout.write(`    upload ${path.basename(local)} (${size}MB)...`);
    sftp.open(remote, 'w', (err, fd) => {
      if (err) return reject(err);
      sftp.write(fd, data, 0, data.length, 0, err2 => {
        if (err2) return reject(err2);
        sftp.close(fd, () => { console.log(' done'); resolve(); });
      });
    });
  });
}

function getSftp(conn) {
  return new Promise((resolve, reject) => conn.sftp((err, sftp) => err ? reject(err) : resolve(sftp)));
}

function uploadFileSilent(sftp, local, remote) {
  return new Promise((resolve, reject) => {
    const data = fs.readFileSync(local);
    sftp.open(remote, 'w', (err, fd) => {
      if (err) return reject(err);
      sftp.write(fd, data, 0, data.length, 0, err2 => {
        if (err2) return reject(err2);
        sftp.close(fd, resolve);
      });
    });
  });
}

async function uploadDirIfNeeded(conn, sftp, localDir, remoteDir) {
  if (!fs.existsSync(localDir)) return;
  const files = fs.readdirSync(localDir).filter(f => /\.(jpg|jpeg|png)$/i.test(f));
  if (!files.length) return;

  // 服务器上已有同等数量则跳过
  const { out } = await exec(conn, `ls "${remoteDir}" 2>/dev/null | wc -l`);
  if (parseInt(out.trim()) >= files.length) {
    console.log(`    已存在 ${files.length} 张，跳过`);
    return;
  }

  await exec(conn, `mkdir -p "${remoteDir}"`);
  const CONCURRENCY = 20;
  let done = 0;
  for (let i = 0; i < files.length; i += CONCURRENCY) {
    const batch = files.slice(i, i + CONCURRENCY);
    await Promise.all(batch.map(f => uploadFileSilent(sftp, path.join(localDir, f), `${remoteDir}/${f}`)));
    done += batch.length;
    process.stdout.write(`\r    上传进度 ${done}/${files.length}`);
  }
  console.log(`\n    ✅ ${done} 张头像上传完成`);
}

async function main() {
  console.log(`\n${'='.repeat(60)}`);
  console.log(`  部署 driftbottle → ${HOST}:${DEPLOY}`);
  console.log(`${'='.repeat(60)}\n`);

  if (!fs.existsSync(BINARY)) {
    console.error(`[ERROR] 找不到 ${BINARY}`);
    process.exit(1);
  }

  const conn = new Client();
  await new Promise((resolve, reject) => {
    conn.on('ready', resolve).on('error', reject)
      .connect({ host: HOST, port: 22, username: USER, privateKey: fs.readFileSync(SSH_KEY), readyTimeout: Number(process.env.DEPLOY_SSH_TIMEOUT) || 60000 });
  });
  console.log('[OK] SSH 连接成功');

  const sftp = await getSftp(conn);

  // 1. 创建目录
  console.log('\n[1] 创建目录...');
  await exec(conn, `mkdir -p ${DEPLOY}/uploads ${DEPLOY}/cert`);

  // 2. 停止旧服务
  console.log('\n[2] 停止旧服务...');
  await exec(conn, 'systemctl stop driftbottle 2>/dev/null; true');

  // 3. 上传二进制
  console.log('\n[3] 上传二进制...');
  await uploadFile(sftp, BINARY, `${DEPLOY}/driftbottle`);
  await exec(conn, `chmod +x ${DEPLOY}/driftbottle`);

  // 4. 上传 cert
  console.log('\n[4] 上传证书文件...');
  if (fs.existsSync(CERT_DIR)) {
    for (const f of fs.readdirSync(CERT_DIR)) {
      if (f.endsWith('.pem')) {
        await uploadFile(sftp, path.join(CERT_DIR, f), `${DEPLOY}/cert/${f}`);
      }
    }
  }

  // 5. 上传机器人头像(女:robot / 男:robot_man)
  console.log('\n[5] 上传机器人头像...');
  await uploadDirIfNeeded(conn, sftp, AVATAR_DIR, `${DEPLOY}/uploads/robot`);
  console.log('    男性头像 robot_man...');
  await uploadDirIfNeeded(conn, sftp, AVATAR_DIR_MAN, `${DEPLOY}/uploads/robot_man`);

  // 6. 写 .env
  console.log('\n[6] 写入 .env...');
  await writeRemote(sftp, `${DEPLOY}/.env`, ENV);
  await exec(conn, `chmod 600 ${DEPLOY}/.env`);
  console.log('    .env 写入完成');

  // 7. systemd
  console.log('\n[7] 配置 systemd 服务...');
  await writeRemote(sftp, '/etc/systemd/system/driftbottle.service', SYSTEMD);
  await exec(conn, 'systemctl daemon-reload');
  await exec(conn, 'systemctl enable driftbottle');
  await exec(conn, 'systemctl start driftbottle');
  await new Promise(r => setTimeout(r, 2000));
  const { out: status } = await exec(conn, 'systemctl is-active driftbottle');
  if (status.trim() !== 'active') {
    console.log('\n[ERROR] 服务启动失败，日志:');
    await exec(conn, 'journalctl -u driftbottle -n 40 --no-pager');
    sftp.end(); conn.end(); process.exit(1);
  }
  console.log('    ✅ 服务已启动');

  // 8. nginx
  console.log('\n[8] 配置 nginx...');
  // 幂等检查:在所有 nginx 配置里查 location /message/(排除 .bak 备份),避免重复插入。
  // 注意:实际 location 可能在 nginx.conf 而非 sites-enabled,必须全量 grep,否则每次部署都会重复追加。
  const { out: nginxConf } = await exec(conn,
    "grep -rl 'location /message/' /etc/nginx/ 2>/dev/null | grep -v '\\.bak' | head -1");

  const locationBlock = `
    # driftbottle
    location /message/ {
        proxy_pass         http://127.0.0.1:${PORT_APP}/;
        proxy_http_version 1.1;
        proxy_set_header   Upgrade $http_upgrade;
        proxy_set_header   Connection "upgrade";
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;
        proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400;
    }`;

  if (nginxConf.trim()) {
    console.log(`    location /message/ 已存在(${nginxConf.trim()}),跳过`);
  } else {
    // 找含 listen 443 的配置文件(排除 .bak 备份)
    const { out: httpsFile } = await exec(conn,
      "grep -rl 'listen 443' /etc/nginx/ 2>/dev/null | grep -v '\\.bak' | head -1");
    const target = httpsFile.trim() || '/etc/nginx/sites-enabled/default';
    console.log(`    修改 ${target}`);

    const { out: origContent } = await exec(conn, `cat ${target}`);
    // 在最后一个 } 前插入 location 块
    const lastBrace = origContent.lastIndexOf('}');
    if (lastBrace === -1) {
      console.log('    [WARN] 未找到 } 插入点，手动添加片段');
      await writeRemote(sftp, '/etc/nginx/conf.d/driftbottle.conf',
        `# 手动添加到 server{} 内\n${locationBlock}\n`);
    } else {
      const newContent = origContent.slice(0, lastBrace) + locationBlock + '\n' + origContent.slice(lastBrace);
      await writeRemote(sftp, target, newContent);
      console.log('    location /message/ 已插入');
    }
  }

  const { out: testResult, errout: testErr } = await exec(conn, 'nginx -t 2>&1');
  const testOutput = (testResult + testErr).toLowerCase();
  if (testOutput.includes('successful')) {
    await exec(conn, 'systemctl reload nginx');
    console.log('    ✅ nginx 重载成功');
  } else {
    console.log(`    ❌ nginx 测试失败: ${testResult}${testErr}`);
  }

  sftp.end();
  conn.end();

  console.log(`
${'='.repeat(60)}
  部署完成！

  服务 Base URL : ${BASE_URL}
  支付回调     : ${BASE_URL}/api/pay/callback/wx
  验证         : curl ${BASE_URL}/api/pay/packages
${'='.repeat(60)}
`);
}

main().catch(e => { console.error('[FATAL]', e.message); process.exit(1); });
