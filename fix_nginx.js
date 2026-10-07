'use strict';
const { Client } = require('ssh2');

const conn = new Client();
conn.on('ready', async () => {
  function exec(cmd) {
    return new Promise((resolve, reject) => {
      conn.exec(cmd, (err, stream) => {
        if (err) return reject(err);
        let out = '', e = '';
        stream.on('data', d => out += d);
        stream.stderr.on('data', d => e += d);
        stream.on('close', () => resolve((out + e).trim()));
      });
    });
  }
  function getSftp() {
    return new Promise((resolve, reject) => conn.sftp((err, s) => err ? reject(err) : resolve(s)));
  }

  const NGINX_CONF = '/etc/nginx/nginx.conf';
  const PORT = 8980;

  const content = await exec(`cat ${NGINX_CONF}`);

  if (content.includes('/message/')) {
    console.log('location /message/ 已存在于 nginx.conf');
  } else {
    console.log('插入 location /message/ 到 ambertu.com server block...');

    const location = `
        # driftbottle message
        location /message/ {
            proxy_pass         http://127.0.0.1:${PORT}/;
            proxy_http_version 1.1;
            proxy_set_header   Upgrade $http_upgrade;
            proxy_set_header   Connection "upgrade";
            proxy_set_header   Host $host;
            proxy_set_header   X-Real-IP $remote_addr;
            proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header   X-Forwarded-Proto $scheme;
            proxy_read_timeout 86400;
        }`;

    // 在 ambertu.com https server block 的 location /gateway/ 后面插入
    const insertAfter = 'location /gateway/ {';
    const idx = content.indexOf(insertAfter);
    if (idx === -1) {
      console.log('[WARN] 找不到 /gateway/ 块，插入到 https server block 末尾 }');
      // 找第一个 https server block 的结束 }
      const httpsIdx = content.indexOf("listen 443 ssl");
      const blockEnd = content.indexOf('\n    }', httpsIdx + 100);
      const newContent = content.slice(0, blockEnd) + '\n' + location + content.slice(blockEnd);
      const sftp = await getSftp();
      await new Promise((res, rej) => {
        const buf = Buffer.from(newContent);
        sftp.open(NGINX_CONF, 'w', (err, fd) => {
          if (err) return rej(err);
          sftp.write(fd, buf, 0, buf.length, 0, err2 => {
            if (err2) return rej(err2);
            sftp.close(fd, res);
          });
        });
      });
    } else {
      // 找 gateway block 结束位置
      let depth = 0, i = idx;
      while (i < content.length) {
        if (content[i] === '{') depth++;
        if (content[i] === '}') { depth--; if (depth === 0) break; }
        i++;
      }
      const insertPos = i + 1;
      const newContent = content.slice(0, insertPos) + '\n' + location + content.slice(insertPos);
      const sftp = await getSftp();
      await new Promise((res, rej) => {
        const buf = Buffer.from(newContent);
        sftp.open(NGINX_CONF, 'w', (err, fd) => {
          if (err) return rej(err);
          sftp.write(fd, buf, 0, buf.length, 0, err2 => {
            if (err2) return rej(err2);
            sftp.close(fd, res);
          });
        });
      });
      console.log('已插入');
    }
  }

  const test = await exec('nginx -t 2>&1');
  console.log('nginx -t:', test);
  if (test.includes('successful')) {
    await exec('systemctl reload nginx');
    console.log('✅ nginx 重载成功');
  } else {
    console.log('❌ nginx 配置有误');
  }

  // 验证服务
  console.log('\n验证服务...');
  const curl = await exec('curl -s -o /dev/null -w "%{http_code}" https://ambertu.com/message/api/pay/packages');
  console.log('GET /message/api/pay/packages → HTTP', curl);

  conn.end();
}).connect({ host: '43.136.54.189', port: 22, username: 'root', password: 'qwezdc!@#%^&123', readyTimeout: 15000 });
