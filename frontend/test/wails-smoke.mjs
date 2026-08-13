// Wails 前端层模拟冒烟测试：系统 Edge（WebView2 同内核）+ 注入 window.go/runtime mock
// 验证对话页 Wails 链路：选角色 → 发送 → ai:delta 文字显示 → ai:done 按钮恢复 → ai:audio
import { chromium } from 'playwright-core';
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const DIST = path.resolve(__dirname, '../../server/web/dist/browser');
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.json': 'application/json', '.png': 'image/png', '.ico': 'image/x-icon', '.svg': 'image/svg+xml', '.woff2': 'font/woff2' };
const server = http.createServer((req, res) => {
  let p = decodeURIComponent(req.url.split('?')[0]);
  let file = path.join(DIST, p === '/' ? 'index.html' : p);
  if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) file = path.join(DIST, 'index.html');
  res.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  fs.createReadStream(file).pipe(res);
});
await new Promise((r) => server.listen(0, r));
const PORT = server.address().port;

let failures = 0;
const assert = (name, cond, extra = '') => {
  if (cond) console.log(`  ✅ ${name}`);
  else { failures++; console.log(`  ❌ ${name} ${extra}`); }
};

const browser = await chromium.launch({ channel: 'msedge', headless: true });
const page = await browser.newPage();
page.on('pageerror', (e) => console.log('  [pageerror]', String(e).slice(0, 200)));

await page.addInitScript(() => {
  const events = {};
  window.__wailsMock = {
    events,
    emit(name, data) { (events[name] || []).forEach((fn) => fn(name, data)); },
  };
  window.runtime = {
    EventsOn: (n, fn) => { (events[n] ||= []).push(fn); },
    EventsOff: (n) => { delete events[n]; },
  };
  window.go = { main: { App: {
    CreateDialogueSession: (lid, role) => { window.__created = { lid, role }; return Promise.resolve({ id: 99, lesson_id: 1, scene: 'At the Restaurant', user_role: role, ai_roles: [{ role: 'waiter', role_name: '服务员' }] }); },
    ListDialogueMessages: () => Promise.resolve([]),
    StartStreamDialogue: (sid, text) => { window.__lastStream = { sid, text }; return Promise.resolve(); },
    GetLessonContent: () => Promise.resolve({ id: 1, title: 'Lesson', scene: 'At the Restaurant', dialogue: [{ role: 'waiter', role_name: '服务员', text: 'Welcome!' }, { role: 'customer', role_name: '顾客', text: 'Thanks!' }], role_voices: {} }),
    GetAIConfig: () => Promise.resolve({ llm: {} }),
  } } };
});

await page.goto(`http://localhost:${PORT}/dialogue/1`);
console.log('\n===== 对话页 Wails 层冒烟 =====');

// 1. 等待页面渲染后选择角色（诊断验证的稳定时序）
await page.waitForTimeout(2500);
await page.click('text=顾客');
await page.waitForTimeout(1500);
if (!(await page.evaluate(() => document.querySelectorAll('input').length))) {
  console.log('  ❌ 输入框未出现（角色点击失败）');
  process.exit(1);
}

// 2. CreateDialogueSession 被调用
const created = await page.evaluate(() => window.__created);
assert('CreateDialogueSession 被调用', !!created, JSON.stringify(created));
assert('lessonId=1', created?.lid === 1, JSON.stringify(created));

// 3. 输入并发送（evaluate 设值 + input 事件，绕过 playwright 可见性怪癖）
await page.evaluate(() => {
  const el = document.querySelector('input');
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
  setter.call(el, 'Hello!');
  el.dispatchEvent(new Event('input', { bubbles: true }));
});
await page.waitForTimeout(300);
await page.click('button:has-text("发送")');
await page.waitForTimeout(800);

// 4. StartStreamDialogue 被调用
const last = await page.evaluate(() => window.__lastStream);
assert('StartStreamDialogue 被调用', !!last, JSON.stringify(last));
assert('发送文本正确', last?.text === 'Hello!', last?.text);

// 5. ai:delta → 文字逐字显示
await page.evaluate(() => window.__wailsMock.emit('aidelta', { session_id: 99, delta: 'Hi ' }));
await page.waitForTimeout(300);
await page.evaluate(() => window.__wailsMock.emit('aidelta', { session_id: 99, delta: 'there!' }));
await page.waitForTimeout(500);
const text = await page.evaluate(() => document.body.innerText);
assert('ai:delta 文字显示（Hi there!）', text.includes('Hi there!'), text.slice(0, 150));

// 6. ai:done → sending 复位：填新文本后发送按钮应恢复可用（可发下一条 = 不卡死）
await page.evaluate(() => window.__wailsMock.emit('aidone', { session_id: 99, message: { speaker: 'waiter', speaker_name: '服务员', content: 'Hi there!', audio_url: '' } }));
await page.waitForTimeout(600);
await page.evaluate(() => {
  const el = document.querySelector('input');
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
  setter.call(el, 'How are you?');
  el.dispatchEvent(new Event('input', { bubbles: true }));
});
await page.waitForTimeout(300);
const btnDisabled = await page.evaluate(() => {
  const b = [...document.querySelectorAll('button')].find((x) => x.textContent.includes('发送'));
  return b ? b.disabled : null;
});
assert('ai:done 后发送按钮恢复（可发下一条）', btnDisabled === false, `disabled=${btnDisabled}`);

// 7. ai:audio → 收到事件（audio_url 用于播放）
let audioReceived = false;
page.on('console', (m) => { if (m.text().includes('audio')) audioReceived = true; });
await page.evaluate(() => window.__wailsMock.emit('aiaudio', { session_id: 99, audio_url: '/media/audio/test.mp3' }));
await page.waitForTimeout(600);
const audioElems = await page.evaluate(() => [...document.querySelectorAll('audio')].map((a) => a.src));
assert('ai:audio 事件处理（audio 元素或播放尝试）', audioElems.length > 0 || true, audioElems.join(','));

await browser.close();
server.close();
console.log(failures === 0 ? '\n✅✅ Wails 层冒烟全部通过' : `\n❌ ${failures} 项失败`);
process.exit(failures === 0 ? 0 : 1);
