// 从 newman JSON 报告抽取逐请求响应 → .responses.jsonl / .responses.md
const fs = require('fs');
const path = require('path');

const reportsDir = path.resolve(__dirname, '..', 'reports');
if (!fs.existsSync(reportsDir)) { console.log('no reports dir'); process.exit(0); }
const files = fs.readdirSync(reportsDir).filter(f => f.endsWith('.json') && !f.startsWith('_') && !f.endsWith('.responses.json'));

for (const f of files) {
  const collection = f.replace(/\.json$/, '');
  const raw = fs.readFileSync(path.join(reportsDir, f), 'utf8');
  let j;
  try { j = JSON.parse(raw); } catch (e) { console.error(`skip ${f}: ${e.message}`); continue; }
  const runStartedAt = (j.run && j.run.timings && j.run.timings.started)
    ? new Date(j.run.timings.started).toISOString() : null;

  const rows = [];
  for (const exec of (j.run && j.run.executions) || []) {
    const item = exec.item || {};
    const req = exec.request || {};
    const res = exec.response || {};
    const url = req.url
      ? '/' + ((req.url.path || []).join('/'))
        + (req.url.query && req.url.query.length
            ? '?' + req.url.query.map(q => `${q.key}=${q.value}`).join('&') : '')
      : '';
    let reqBody = '';
    if (req.body && req.body.mode === 'raw' && typeof req.body.raw === 'string') reqBody = req.body.raw;
    let resBody = '';
    try { if (res.stream && res.stream.data) resBody = Buffer.from(res.stream.data).toString('utf8'); } catch (_) {}
    const assertions = (exec.assertions || []).map(a => ({ name: a.assertion, passed: !a.error, message: a.error ? a.error.message : null }));
    rows.push({
      runStartedAt, collection, item: item.name || '', method: req.method || '', url,
      httpStatus: res.code || 0, responseTimeMs: res.responseTime || 0,
      requestBody: reqBody, responseBody: resBody, assertions,
      assertionsPassed: assertions.filter(a => a.passed).length,
      assertionsFailed: assertions.filter(a => !a.passed).length,
    });
  }

  fs.writeFileSync(path.join(reportsDir, `${collection}.responses.jsonl`), rows.map(r => JSON.stringify(r)).join('\n'), 'utf8');

  const md = [`# ${collection} · 响应明细 (${runStartedAt || '时间未知'})`, ''];
  for (const r of rows) {
    md.push(`## ${r.assertionsFailed === 0 ? '✅' : '❌'} ${r.item}`, '');
    md.push(`- ${r.method} ${r.url} → HTTP ${r.httpStatus} (${r.responseTimeMs}ms)`);
    md.push(`- 断言: ${r.assertionsPassed} passed / ${r.assertionsFailed} failed`);
    if (r.requestBody) { md.push('', '请求 body：', '```', r.requestBody.length > 500 ? r.requestBody.slice(0, 500) + ' ...(truncated)' : r.requestBody, '```'); }
    md.push('', '响应 body：', '```', r.responseBody.length > 800 ? r.responseBody.slice(0, 800) + ' ...(truncated)' : r.responseBody, '```');
    const fa = r.assertions.filter(a => !a.passed);
    if (fa.length) { md.push('', '失败断言：'); for (const a of fa) md.push(`- \`${a.name}\` — ${a.message}`); }
    md.push('');
  }
  fs.writeFileSync(path.join(reportsDir, `${collection}.responses.md`), md.join('\n'), 'utf8');
  console.log(`${collection}: ${rows.length} requests → responses.jsonl / responses.md`);
}
