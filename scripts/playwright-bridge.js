#!/usr/bin/env node
// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Playwright bridge for HelixQA autonomous QA.
// Accepts a JSON command on stdin, executes it via Playwright
// CDP connection to a headless Chromium, writes result to stdout.
//
// Browser lifecycle:
//   "launch" — starts headless chromium, saves state to file
//   "close"  — kills the browser process
//   All other actions connect to the running browser via CDP.

const { chromium } = require('playwright');
const { execFileSync, spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const http = require('http');

const STATE_FILE = path.join(
  process.env.HELIX_OUTPUT_DIR || '/tmp',
  '.helix-browser-state.json'
);

const CDP_PORT = 9223;

async function main() {
  const input = fs.readFileSync(0, 'utf-8').trim();
  if (!input) {
    process.stderr.write('error: empty input\n');
    process.exit(1);
  }

  let cmd;
  try {
    cmd = JSON.parse(input);
  } catch (e) {
    process.stderr.write('error: invalid JSON: ' + e.message + '\n');
    process.exit(1);
  }

  switch (cmd.action) {
    case 'launch':
      await handleLaunch(cmd);
      break;
    case 'close':
      await handleClose();
      break;
    default:
      await handleAction(cmd);
      break;
  }
}

// findChromium locates the chromium binary using execFileSync
// to avoid shell injection.
function findChromium() {
  const candidates = [
    'chromium', 'chromium-browser',
    'google-chrome', 'google-chrome-stable',
  ];
  for (const name of candidates) {
    try {
      execFileSync('which', [name], { stdio: 'pipe' });
      return name;
    } catch (_) {}
  }
  return 'chromium';
}

// killExisting silently kills any previously launched browser
// without writing to stdout. Kills by port to handle stale
// processes not tracked by the state file.
function killExisting() {
  // Kill by state file PID.
  if (fs.existsSync(STATE_FILE)) {
    try {
      const state = JSON.parse(fs.readFileSync(STATE_FILE, 'utf-8'));
      process.kill(state.pid, 'SIGKILL');
    } catch (_) {}
    try { fs.unlinkSync(STATE_FILE); } catch (_) {}
  }
  // Kill ALL chromium instances on our CDP port.
  try {
    execFileSync('pkill', ['-9', '-f',
      'remote-debugging-port=' + CDP_PORT], { stdio: 'ignore' });
  } catch (_) {}
}

// waitForCDP polls the CDP endpoint until it responds.
function waitForCDP(url, retries) {
  return new Promise((resolve, reject) => {
    let attempt = 0;
    function tryConnect() {
      http.get(url + '/json/version', (res) => {
        let data = '';
        res.on('data', chunk => { data += chunk; });
        res.on('end', () => resolve(true));
      }).on('error', () => {
        attempt++;
        if (attempt >= retries) {
          reject(new Error('CDP not ready after ' + retries + ' attempts'));
        } else {
          setTimeout(tryConnect, 500);
        }
      });
    }
    tryConnect();
  });
}

async function handleLaunch(cmd) {
  // Kill any existing browser silently.
  killExisting();

  const chromiumPath = findChromium();
  const cdpURL = 'http://127.0.0.1:' + CDP_PORT;

  // Launch headless chromium with CDP.
  const proc = spawn(chromiumPath, [
    '--headless=new',
    '--no-sandbox',
    '--disable-gpu',
    '--disable-dev-shm-usage',
    '--remote-debugging-port=' + CDP_PORT,
    '--window-size=1920,1080',
    '--hide-scrollbars',
    cmd.url || 'about:blank',
  ], {
    stdio: 'ignore',
    detached: true,
  });
  proc.unref();

  // Wait for CDP to be ready.
  try {
    await waitForCDP(cdpURL, 40);
  } catch (e) {
    process.stderr.write('error: ' + e.message + '\n');
    process.exit(1);
  }

  // Save state — viewport setup happens on first action.
  fs.writeFileSync(STATE_FILE, JSON.stringify({
    pid: proc.pid,
    cdpURL,
    url: cmd.url,
  }));

  process.stdout.write(JSON.stringify({
    status: 'launched',
    pid: proc.pid,
  }));
}

async function handleClose() {
  if (!fs.existsSync(STATE_FILE)) {
    process.stdout.write(JSON.stringify({ status: 'no_browser' }));
    return;
  }
  const state = JSON.parse(fs.readFileSync(STATE_FILE, 'utf-8'));
  try {
    process.kill(state.pid, 'SIGTERM');
  } catch (_) {}
  try {
    fs.unlinkSync(STATE_FILE);
  } catch (_) {}
  process.stdout.write(JSON.stringify({ status: 'closed' }));
}

async function handleAction(cmd) {
  if (!fs.existsSync(STATE_FILE)) {
    process.stderr.write('error: no browser — call launch first\n');
    process.exit(1);
  }

  const state = JSON.parse(fs.readFileSync(STATE_FILE, 'utf-8'));
  let browser;
  try {
    browser = await chromium.connectOverCDP(state.cdpURL);
  } catch (e) {
    process.stderr.write('error: connect: ' + e.message + '\n');
    process.exit(1);
  }

  const contexts = browser.contexts();
  if (contexts.length === 0) {
    if (typeof browser.disconnect === 'function') {
      browser.disconnect();
    }
    process.stderr.write('error: no browser context\n');
    process.exit(1);
  }
  const pages = contexts[0].pages();
  if (pages.length === 0) {
    if (typeof browser.disconnect === 'function') {
      browser.disconnect();
    }
    process.stderr.write('error: no page\n');
    process.exit(1);
  }
  const page = pages[0];

  try {
    switch (cmd.action) {
      case 'screenshot': {
        const buf = await page.screenshot({
          type: 'png',
          fullPage: false,
        });
        process.stdout.write(buf);
        break;
      }
      case 'click': {
        await page.mouse.click(cmd.x, cmd.y);
        ok();
        break;
      }
      case 'type': {
        await page.keyboard.type(cmd.text, { delay: 30 });
        ok();
        break;
      }
      case 'scroll': {
        let dx = 0, dy = 0;
        const amt = cmd.amount || 300;
        if (cmd.direction === 'up')    dy = -amt;
        if (cmd.direction === 'down')  dy = amt;
        if (cmd.direction === 'left')  dx = -amt;
        if (cmd.direction === 'right') dx = amt;
        await page.mouse.wheel(dx, dy);
        ok();
        break;
      }
      case 'key': {
        await page.keyboard.press(cmd.key);
        ok();
        break;
      }
      case 'back': {
        await page.goBack({ timeout: 10000 }).catch(() => {});
        ok();
        break;
      }
      case 'navigate': {
        await page.goto(cmd.url, {
          waitUntil: 'load',
          timeout: 30000,
        });
        ok();
        break;
      }
      case 'longpress': {
        await page.mouse.move(cmd.x, cmd.y);
        await page.mouse.down();
        await page.waitForTimeout(1000);
        await page.mouse.up();
        ok();
        break;
      }
      case 'swipe': {
        await page.mouse.move(cmd.fromX, cmd.fromY);
        await page.mouse.down();
        await page.mouse.move(cmd.toX, cmd.toY, { steps: 10 });
        await page.mouse.up();
        ok();
        break;
      }
      default:
        process.stderr.write('error: unknown action: ' + cmd.action + '\n');
        process.exit(1);
    }
  } finally {
    // Force exit — disconnect() hangs on some CDP
    // connections. The browser process stays alive
    // independently via the detached spawn.
    setTimeout(() => process.exit(0), 500);
  }
}

function ok() {
  process.stdout.write(JSON.stringify({ status: 'ok' }));
}

main().catch(e => {
  process.stderr.write('error: ' + e.message + '\n');
  process.exit(1);
});
