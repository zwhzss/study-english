import { chromium } from 'playwright-core';
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const DIST = path.resolve(__dirname, '../../server/web/dist/browser');
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.json': 'application/json' };
const server = http.createServer((req, res) => {
  let p = decodeURIComponent(req.url.split('?')[0]);
  let file = path.join(DIST, p === '/' ? 'index.html' : p);
  if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) file = path.join(DIST, 'index.html');
  res.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  fs.createReadStream(file).pipe(res);
});
await new Promise((r) => server.listen(0, r));
const browser = await chromium.launch({ channel: 'msedge', headless: true });
const page = await browser.newPage();
page.on('pageerror', (e) => console.log('[pageerror]', String(e).slice(0, 200)));

// mock：1 本教材 + 1 个 has_content=true 的课时（有 AI 内容）
await page.addInitScript(() => {
  window.runtime = { EventsOn: () => {}, EventsOff: () => {} };
  window.go = { main: { App: {
    ListTextbooks: () => Promise.resolve([{ id: 1, name: '测试教材', publisher: '社', stage: '小学', grade: '一年级', catalog_source: 'builtin' }]),
    GetCatalog: (id) => Promise.resolve([{ level: 'Grade1', unit: 'Unit 1', title: 'Unit 1', lessons: [{ id: 101, lesson: 'Lesson 1', title: 'Hello', scene: '', status: 'published', has_content: true }] }]),
    GetLessonContent: (id, kind) => Promise.resolve({ id: 101, catalog_item_id: 101, unit: 'Unit 1', title: 'Hello', scene: '问候', dialogue: [{ role: 'teacher', role_name: '老师', text: 'Good morning!', audio_url: '' }], key_vocab: [], grammar_points: [], practice: [], images: [], svg_images: [], role_voices: {} }),
    GetImportedContent: (id) => Promise.reject(new Error('无导入内容')),
    GetTextbookMeta: () => Promise.resolve({ textbooks: [], stages: [], grades: {} }),
    GetAIConfig: () => Promise.resolve({ llm: {} }),
  } } };
});

await page.goto(`http://localhost:${server.address().port}/textbooks`);
await page.waitForTimeout(2500);
// 点教材（加载目录）
await page.click('text=测试教材').catch(() => {});
await page.waitForTimeout(1500);
// 点课时
await page.click('text=Hello').catch(() => {});
await page.waitForTimeout(1500);

const body = await page.evaluate(() => document.body.innerText);
const hasGen = body.includes('AI 生成中') || body.includes('生成中');
const hasContent = body.includes('Good morning!');
console.log('=== 课时点击结果 ===');
console.log('显示"生成中":', hasGen);
console.log('显示课文内容(Good morning!):', hasContent);
console.log('页面片段:', body.slice(0, 300).replace(/\n/g, ' | '));
await browser.close(); server.close();
process.exit(hasGen && !hasContent ? 1 : 0);
