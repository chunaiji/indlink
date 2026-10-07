'use strict';
// 批量生成 200 个机器人账号（带头像 + 已认证）
// 运行: node seed_robots.js

const { Client } = require('ssh2');

const HOST   = '43.136.54.189';
const USER   = 'root';
const PASS   = 'qwezdc!@#%^&123!@#$%QWE';
const TENANT = 100;
const COUNT  = 200;
const BASE_URL = 'https://ambertu.com/message';
const AVATAR_DIR = '/usr/jack/deploy/go_workspace/uploads/robot';
const MYSQL = `mysql -u root -pcnj008 ai_message`;

// 同 Go 端数据
const nickPrefix = ['夏天的','海边的','深夜','云朵','柠檬','晚风','听海','旧时光','星河','温柔','麦田','拾光','南巷','北屿','暖阳'];
const nickSuffix = ['风','猫','鲸','信','光','屿','river','森林','汽水','candy','鹿','晚安','贝壳','潮汐'];
const cities = ['北京','上海','广州','深圳','杭州','成都','武汉','南京','西安','重庆','苏州','长沙','厦门','青岛'];

const rand = (arr) => arr[Math.floor(Math.random() * arr.length)];
const randInt = (min, max) => min + Math.floor(Math.random() * (max - min + 1));

// 简易 Snowflake：(ms - epoch) << 12 | seq
const EPOCH = 1704067200000n; // 2024-01-01
function snowflake(seq) {
  const ms = BigInt(Date.now()) - EPOCH;
  return (ms << 12n) | BigInt(seq % 4096);
}

function exec(conn, cmd) {
  return new Promise((resolve, reject) => {
    conn.exec(cmd, (err, stream) => {
      if (err) return reject(err);
      let out = '', errout = '';
      stream.on('data', d => out += d);
      stream.stderr.on('data', d => errout += d);
      stream.on('close', () => resolve({ out: out.trim(), err: errout.trim() }));
    });
  });
}

async function main() {
  const conn = new Client();
  await new Promise((res, rej) =>
    conn.on('ready', res).on('error', rej)
      .connect({ host: HOST, port: 22, username: USER, password: PASS, readyTimeout: 20000 })
  );
  console.log('[OK] SSH 连接');

  // 1. 获取头像文件列表
  const { out: lsOut } = await exec(conn, `ls ${AVATAR_DIR} 2>/dev/null`);
  const avatarFiles = lsOut.split('\n').filter(f => /\.(jpg|jpeg|png)$/i.test(f));
  if (!avatarFiles.length) {
    console.error('[ERROR] 未找到头像文件，目录:', AVATAR_DIR);
    conn.end(); process.exit(1);
  }
  console.log(`[OK] 头像池 ${avatarFiles.length} 张`);

  // 2. 检查当前机器人数量
  const { out: cntOut } = await exec(conn, `${MYSQL} -se "SELECT COUNT(*) FROM users WHERE tenant_id=${TENANT} AND is_robot=1;"`);
  const existing = parseInt(cntOut) || 0;
  console.log(`[INFO] 当前机器人 ${existing} 个`);

  const toCreate = COUNT - existing;
  if (toCreate <= 0) {
    console.log(`[OK] 已有 ${existing} 个机器人，无需新增`);
    conn.end(); return;
  }
  console.log(`[INFO] 将新增 ${toCreate} 个机器人`);

  // 3. 生成 SQL
  const now = new Date().toISOString().slice(0, 19).replace('T', ' ');
  const userRows = [];
  const walletRows = [];
  const usedNicks = new Set();

  for (let i = 0; i < toCreate; i++) {
    const uid = snowflake(i + Date.now() % 1000).toString();
    const gender = randInt(1, 2);
    const age = randInt(18, 35);
    const city = rand(cities);

    let nick;
    do {
      nick = rand(nickPrefix) + rand(nickSuffix);
    } while (usedNicks.has(nick));
    usedNicks.add(nick);

    const avatarFile = rand(avatarFiles);
    const avatar = `${BASE_URL}/static/robot/${avatarFile}`;
    // 随机注册时间（过去 0-720 小时内）
    const createdOffset = randInt(0, 720);
    const created = new Date(Date.now() - createdOffset * 3600000).toISOString().slice(0, 19).replace('T', ' ');

    const esc = (s) => s.replace(/'/g, "\\'");
    userRows.push(
      `(${uid},${TENANT},'${esc(nick)}','${esc(avatar)}',${gender},${age},'${esc(city)}',1,1,'active','${created}','${now}')`
    );
    walletRows.push(`(${uid},${TENANT},'${now}')`);
  }

  // 4. 执行插入
  const userSQL = `INSERT IGNORE INTO users (user_id,tenant_id,nickname,avatar,gender,age,city,is_verified,is_robot,status,created_at,last_active_at) VALUES ${userRows.join(',')};`;
  const walletSQL = `INSERT IGNORE INTO wallets (user_id,tenant_id,updated_at) VALUES ${walletRows.join(',')};`;

  console.log('\n[1] 插入 users...');
  const { out: u1, err: e1 } = await exec(conn, `${MYSQL} -e "${userSQL.replace(/"/g, '\\"')}"`);
  if (e1) console.log('   ', e1);

  console.log('[2] 插入 wallets...');
  const { out: u2, err: e2 } = await exec(conn, `${MYSQL} -e "${walletSQL.replace(/"/g, '\\"')}"`);
  if (e2) console.log('   ', e2);

  // 5. 验证
  const { out: finalCnt } = await exec(conn, `${MYSQL} -se "SELECT COUNT(*) FROM users WHERE tenant_id=${TENANT} AND is_robot=1;"`);
  console.log(`\n[OK] 机器人总数: ${finalCnt}`);

  conn.end();
  console.log('\n=== 完成 ===');
}

main().catch(e => { console.error('[FATAL]', e.message); process.exit(1); });
