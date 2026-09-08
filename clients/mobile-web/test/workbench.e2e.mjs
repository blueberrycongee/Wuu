// Real browser -> encrypted relay -> Go Agent -> host filesystem. Only the
// model is deterministic; no renderer, bridge, tool or session API is mocked.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import fs from 'node:fs/promises';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const clientDir = fileURLToPath(new URL('..', import.meta.url));
const repoDir = path.resolve(clientDir, '../..');
const temp = await fs.mkdtemp(path.join(os.tmpdir(), 'wuu-web-e2e-'));
const workspace = path.join(temp, 'workspace'), state = path.join(temp, 'state');
const processes = [];
let browser, provider;
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
async function until(check, label, timeout = 30000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) { if (await check()) return; await pause(50); }
  throw new Error(`Timed out: ${label}`);
}
function start(command, args, options = {}) {
  const child = spawn(command, args, { cwd: repoDir, ...options, stdio: ['ignore', 'pipe', 'pipe'] });
  const run = { child, output: '', exited: false };
  child.stdout.on('data', data => { run.output += data; });
  child.stderr.on('data', data => { run.output += data; });
  child.on('exit', () => { run.exited = true; });
  child.on('error', error => { run.output += error.message; run.exited = true; });
  processes.push(run); return run;
}
async function listen(server) {
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  return server.address().port;
}
async function freePort() {
  const server = http.createServer(); const port = await listen(server);
  await new Promise(resolve => server.close(resolve)); return port;
}
let releaseCompletion, completionRequested = false;
const completionGate = new Promise(resolve => { releaseCompletion = resolve; });
const finalText = 'Completed the browser task. Created src/browser-task.ts on the paired computer.';
const fileText = 'export const remoteResult = "written on the paired computer";\n';
try {
  await fs.mkdir(workspace); await fs.mkdir(state);
  await fs.writeFile(path.join(workspace, '.gitignore'), '.wuu-state/\n');
  const git = (...args) => execFileSync('git', ['-C', workspace, ...args], { stdio: 'pipe' });
  git('init', '-b', 'main'); git('add', '.gitignore');
  git('-c', 'user.name=Web Test', '-c', 'user.email=web@example.invalid', '-c', 'commit.gpgSign=false', 'commit', '-m', 'Initial');
  const binary = process.env.WUU_E2E_BINARY || path.join(temp, 'wuu');
  if (!process.env.WUU_E2E_BINARY) execFileSync('go', ['build', '-o', binary, './cmd/wuu'], { cwd: repoDir, stdio: 'inherit' });
  provider = http.createServer(async (req, res) => {
    try {
      let raw = ''; for await (const data of req) raw += data;
      if (req.method === 'GET') { res.end(JSON.stringify({ object: 'list', data: [{ id: 'web-fixture', object: 'model' }] })); return; }
      const body = JSON.parse(raw);
      const writeTool = body.tools?.some(tool => tool.function?.name === 'write_file');
      const hasToolReply = body.messages?.some(message => message.role === 'tool');
      const message = writeTool && !hasToolReply
        ? { role: 'assistant', tool_calls: [{ id: 'write-web-fixture', type: 'function', function: { name: 'write_file', arguments: JSON.stringify({ path: 'src/browser-task.ts', content: fileText }) } }] }
        : { role: 'assistant', content: writeTool ? finalText : 'Browser coding task' };
      if (writeTool && hasToolReply) { completionRequested = true; await completionGate; }
      const finish_reason = message.tool_calls ? 'tool_calls' : 'stop';
      if (body.stream) {
        res.writeHead(200, { 'Content-Type': 'text/event-stream' });
        for (const choice of [{ index: 0, delta: message, finish_reason: null }, { index: 0, delta: {}, finish_reason }]) {
          res.write(`data: ${JSON.stringify({ id: 'fixture', object: 'chat.completion.chunk', choices: [choice] })}\n\n`);
        }
        res.end('data: [DONE]\n\n');
      } else {
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify({ id: 'fixture', choices: [{ index: 0, message, finish_reason }] }));
      }
    } catch (error) { res.writeHead(500); res.end(String(error)); }
  });
  const webHost = process.env.WUU_E2E_WEB_HOST || "127.0.0.1";
  const modelPort = await listen(provider), relayPort = await freePort(), webPort = relayPort;
  await fs.writeFile(path.join(state, 'config.json'), JSON.stringify({ default_provider: 'web-test', providers: { 'web-test': { type: 'openai-compatible', base_url: `http://127.0.0.1:${modelPort}/v1`, api_key: 'local-test', model: 'web-fixture' } }, agent: { permission_mode: 'standard' } }));
  const env = { ...process.env, WUU_HOME: state };
  execFileSync(process.execPath, [path.join(repoDir, 'desktop/scripts/build-web.cjs')], { cwd: repoDir, stdio: 'pipe' });
  const relay = start(binary, ['relay', '--addr', `${webHost}:${relayPort}`, '--state', path.join(temp, 'relay.json'), '--web-root', path.join(repoDir, 'desktop/build/mobile-web')], { env });
  await until(() => relay.output.includes('listening'), 'relay startup');
  const host = start(binary, ['remote', 'host', '--workdir', workspace, '--relay', `ws://${webHost}:${relayPort}/v1/connect`, '--pair'], { env });
  await until(() => host.output.includes('wuu://pair?'), 'host pairing');
  const uri = host.output.match(/wuu:\/\/pair\?[^\s]+/)[0];
  browser = await chromium.launch({ channel: process.env.WUU_BROWSER_CHANNEL || undefined });
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, locale: 'zh-CN' });
  const page = await context.newPage(); const pageErrors = [];
  page.on('pageerror', error => pageErrors.push(error.message));
  await page.addInitScript(() => {
    const Native = window.WebSocket;
    window.WebSocket = class extends Native { constructor(...args) { super(...args); window.testSocket = this; } };
    window.testDocumentID = Math.random();
  });
  await page.goto(`http://${webHost}:${webPort}/#${new URLSearchParams({ pair: uri })}`);
  await page.locator('.app-shell').waitFor();
  assert.equal(new URL(page.url()).hash, '');
  const documentID = await page.evaluate(() => window.testDocumentID);
  const input = page.locator('textarea:visible').first();
  await input.fill('Create src/browser-task.ts with the requested test export.');
  await page.getByRole('button', { name: '发送', exact: true }).filter({ visible: true }).click();
  await until(() => completionRequested, 'Agent tool execution');
  assert.equal(await fs.readFile(path.join(workspace, 'src/browser-task.ts'), 'utf8'), fileText);
  await input.fill('Draft that must survive restoration');
  await context.setOffline(true); await page.evaluate(() => window.testSocket.close());
  await page.getByRole('status').filter({ hasText: '连接已断开' }).waitFor();
  assert.equal(await page.locator('.web-workbench').getAttribute('inert'), '');
  releaseCompletion();
  // The host emits this after completing the turn while no browser is attached.
  await until(() => host.output.includes('push agent_done'), 'host completion while detached');
  await context.setOffline(false);
  await page.locator('.web-workbench:not([inert])').waitFor({ timeout: 45000 });
  await page.getByText(finalText, { exact: true }).filter({ visible: true }).waitFor();
  assert.equal(await page.evaluate(() => window.testDocumentID), documentID);
  assert.equal(await input.inputValue(), 'Draft that must survive restoration');
  host.child.kill('SIGTERM');
  await until(() => host.exited, 'host shutdown');
  await page.getByRole('status').filter({ hasText: '连接已断开' }).waitFor();
  const restarted = start(binary, ['remote', 'host', '--workdir', workspace, '--relay', `ws://${webHost}:${relayPort}/v1/connect`], { env });
  await until(() => restarted.output.includes('connected to relay'), 'host restart');
  await page.locator('.web-workbench:not([inert])').waitFor({ timeout: 45000 });
  await page.getByText(finalText, { exact: true }).filter({ visible: true }).waitFor();
  assert.equal(await page.getByText(finalText, { exact: true }).filter({ visible: true }).count(), 1);
  assert.equal(await input.inputValue(), 'Draft that must survive restoration');
  assert.equal(await page.evaluate(() => window.testDocumentID), documentID);
  const changes = await page.evaluate(() => window.wuu.listGitChanges());
  assert(changes.files.some(file => file.path === 'src/browser-task.ts' && file.additions === 1));
  const diff = await page.evaluate(() => window.wuu.readGitFileDiff('src/browser-task.ts'));
  assert.equal(diff.modified_text, fileText);
  await page.getByRole('button', { name: '打开右侧栏', exact: true }).click();
  await page.getByRole('button', { name: '文件', exact: true }).click();
  await page.getByRole('treeitem', { name: 'src', exact: true }).filter({ visible: true }).click();
  await page.getByRole('treeitem', { name: 'browser-task.ts', exact: true }).filter({ visible: true }).click();
  await page.locator('.monaco-editor').filter({ visible: true }).waitFor();
  await page.getByRole('button', { name: '返回', exact: true }).click();
  assert.equal(await page.locator('.workspace-right-panel').getAttribute('data-wuu-view'), 'files');
  await page.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('button', { name: '返回', exact: true }).click();
  for (const [width, height] of [[320, 740], [390, 844], [430, 932], [390, 360]]) {
    await page.setViewportSize({ width, height });
    await until(async () => {
      const rect = await page.getByRole('button', { name: '发送', exact: true }).filter({ visible: true }).boundingBox();
      return rect && rect.x >= 0 && rect.y >= 0 && rect.x + rect.width <= width && rect.y + rect.height <= height;
    }, `composer reachable at ${width}x${height}`);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), width);
  }
  assert.deepEqual(pageErrors, []);
  console.log('PASS: pairing, host tool execution, offline completion, snapshot and draft restoration, host restart, Git review, file navigation and phone viewports');
} catch (error) {
  for (const run of processes) console.error(run.output.slice(-3000).replace(/wuu:\/\/pair\?[^\s]+/g, '[pairing URI]'));
  throw error;
} finally {
  releaseCompletion();
  await browser?.close();
  for (const run of processes) if (!run.exited) run.child.kill('SIGTERM');
  for (const run of processes) {
    try { await until(() => run.exited, 'process shutdown', 5000); } catch { run.child.kill('SIGKILL'); }
  }
  provider?.closeAllConnections(); provider?.close();
  await fs.rm(temp, { recursive: true, force: true });
}
