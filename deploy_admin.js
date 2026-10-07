'use strict';
const { Client } = require('ssh2');
const fs = require('fs');
const path = require('path');
const os = require('os');

const HOST = '175.178.182.166'; // 旧机 43.136.54.189 已迁移废弃(2026-07-31)
const USER = 'root';
const SSH_KEY = path.join(os.homedir(), '.ssh', 'huawei_app_ed25519');
const DEPLOY = '/usr/jack/deploy/go_workspace';
const ADMIN_DIST = path.join(__dirname, 'admin', 'dist');
const REMOTE_ADMIN = `${DEPLOY}/admin-dist`;

const conn = new Client();
conn.on('ready', async () => {
  function exec(cmd) {
    return new Promise((res, rej) => {
      conn.exec(cmd, (err, stream) => {
        if (err) return rej(err);
        let o = '', e = '';
        stream.on('data', d => o += d);
        stream.stderr.on('data', d => e += d);
        stream.on('close', () => { const r = (o + e).trim(); if (r) console.log('   ', r.replace(/\n/g,'\n    ')); res(r); });
      });
    });
  }
  function getSftp() {
    return new Promise((res, rej) => conn.sftp((err, s) => err ? rej(err) : res(s)));
  }
  async function uploadDir(sftp, localDir, remoteDir) {
    await exec(`mkdir -p ${remoteDir}`);
    for (const entry of fs.readdirSync(localDir, { withFileTypes: true })) {
      const lp = path.join(localDir, entry.name);
      const rp = `${remoteDir}/${entry.name}`;
      if (entry.isDirectory()) {
        await uploadDir(sftp, lp, rp);
      } else {
        await new Promise((res, rej) => {
          const data = fs.readFileSync(lp);
          sftp.open(rp, 'w', (err, fd) => {
            if (err) return rej(err);
            sftp.write(fd, data, 0, data.length, 0, err2 => {
              if (err2) return rej(err2);
              sftp.close(fd, res);
            });
          });
        });
      }
    }
    console.log(`    uploaded ${remoteDir}`);
  }

  console.log('\n[1] 上传 admin 静态文件...');
  const sftp = await getSftp();
  await uploadDir(sftp, ADMIN_DIST, REMOTE_ADMIN);

  console.log('\n[2] 检查 nginx admin 路由...');
  // location 可能在 nginx.conf 或 conf.d/*.conf(如新机的 pet.conf),必须全量 grep,不能硬编码文件。
  const existFile = (await exec("grep -rl 'location /message-admin/' /etc/nginx/ 2>/dev/null | grep -v '\\.bak' | head -1")).trim();
  if (existFile) {
    console.log(`    /message-admin/ 已存在(${existFile})，跳过`);
  } else {
    const target = (await exec("grep -rl 'location /message/' /etc/nginx/ 2>/dev/null | grep -v '\\.bak' | head -1")).trim();
    const anchor = 'location /message/ {';
    const content = target ? await exec(`cat ${target}`) : '';
    const idx = content.indexOf(anchor);
    if (!target || idx === -1) {
      // 找不到可靠插入点时绝不盲插(location 写错层级会导致 nginx -t 失败),留给人工处理。
      console.log('    ❌ 未找到含 location /message/ 的 nginx 配置，请手动添加 admin 路由');
    } else {
      const adminLocation = `# driftbottle admin
    location /message-admin/ {
        alias ${REMOTE_ADMIN}/;
        index index.html;
        try_files $uri $uri/ /message-admin/index.html;
    }

    `;
      const newContent = content.slice(0, idx) + adminLocation + content.slice(idx);
      const buf = Buffer.from(newContent);
      await new Promise((res, rej) => {
        sftp.open(target, 'w', (err, fd) => {
          if (err) return rej(err);
          sftp.write(fd, buf, 0, buf.length, 0, err2 => {
            if (err2) return rej(err2);
            sftp.close(fd, res);
          });
        });
      });
      console.log(`    location /message-admin/ 已插入 ${target}`);
    }
  }

  const test = await exec('nginx -t 2>&1');
  if (test.includes('successful')) {
    await exec('systemctl reload nginx');
    console.log('    ✅ nginx 重载成功');
  } else {
    console.log('    ❌ nginx 配置有误:', test);
  }

  console.log('\n[3] 验证...');
  const code = await exec('curl -s -o /dev/null -w "%{http_code}" https://ambertu.com/message-admin/');
  console.log(`    GET /message-admin/ → HTTP ${code}`);

  sftp.end(); conn.end();
  console.log(`
${'='.repeat(50)}
  管理后台: https://ambertu.com/message-admin/
  账号密码见部署配置(默认账号首次登录后请及时修改)
${'='.repeat(50)}
`);
}).connect({ host: HOST, port: 22, username: USER, privateKey: fs.readFileSync(SSH_KEY), readyTimeout: 15000 });
