// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0
//
// HelixQA Challenges Dashboard — client-side logic.
// Reads pipeline-report.json files, aggregates stats, and renders charts.
// All DOM construction uses createElement/textContent — no innerHTML.

'use strict';

// ── State ────────────────────────────────────────────────────────────────────

/** @type {Array<{session: string, report: Object}>} */
let sessions = [];

let trendChart = null;
let bankChart  = null;

// ── DOM helpers ───────────────────────────────────────────────────────────────

const $ = (id) => document.getElementById(id);

function setText(id, val) {
  const el = $(id);
  if (el) el.textContent = val;
}

function setStatus(msg) {
  const el = $('status');
  if (el) el.textContent = msg;
}

/**
 * Create an element, set optional text content and class names.
 * @param {string} tag
 * @param {Object} [opts]
 * @param {string} [opts.text]
 * @param {string} [opts.cls]
 * @param {Object} [opts.style]  key→value pairs applied to el.style
 * @returns {HTMLElement}
 */
function el(tag, opts = {}) {
  const node = document.createElement(tag);
  if (opts.text  !== undefined) node.textContent = opts.text;
  if (opts.cls)                  node.className   = opts.cls;
  if (opts.style) {
    for (const [k, v] of Object.entries(opts.style)) {
      node.style[k] = v;
    }
  }
  return node;
}

// ── File loading ──────────────────────────────────────────────────────────────

/**
 * Read a File as JSON, resolving with parsed object.
 * @param {File} file
 * @returns {Promise<Object>}
 */
function readJSON(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = (e) => {
      try {
        resolve(JSON.parse(e.target.result));
      } catch (err) {
        reject(new Error(`${file.name}: invalid JSON — ${err.message}`));
      }
    };
    reader.onerror = () => reject(new Error(`${file.name}: read error`));
    reader.readAsText(file);
  });
}

/**
 * Derive a short session label from a File.
 * Extracts a timestamp or index from the filename / relative path.
 * @param {File} file
 * @param {number} idx
 * @returns {string}
 */
function sessionLabel(file, idx) {
  const src = file.webkitRelativePath || file.name || '';
  const m   = src.match(/session[-_](\d+)/);
  if (m) {
    const ts = parseInt(m[1], 10);
    if (ts > 1_000_000_000) {
      return new Date(ts * 1000).toLocaleDateString();
    }
    return `S${m[1]}`;
  }
  return `Session ${idx + 1}`;
}

// ── Data extraction helpers ───────────────────────────────────────────────────

/**
 * Extract pass/fail/skip counts from a pipeline-report.json object.
 * Supports multiple common shapes produced by the HelixQA reporter.
 * @param {Object} report
 * @returns {{pass: number, fail: number, skip: number, total: number}}
 */
function extractCounts(report) {
  const s     = report.summary ?? report.results ?? report;
  const pass  = num(s.passed  ?? s.pass  ?? s.success ?? 0);
  const fail  = num(s.failed  ?? s.fail  ?? s.failure ?? 0);
  const skip  = num(s.skipped ?? s.skip  ?? 0);
  const total = num(s.total   ?? (pass + fail + skip));
  return { pass, fail, skip, total: total || (pass + fail + skip) };
}

/**
 * Extract per-bank (category) pass rates from a report.
 * Returns a map of bankName → {pass, total}.
 * @param {Object} report
 * @returns {Map<string, {pass: number, total: number}>}
 */
function extractBanks(report) {
  const map   = new Map();
  const banks = report.banks ?? report.categories ?? report.by_bank ?? [];

  if (Array.isArray(banks)) {
    for (const b of banks) {
      const name  = String(b.name ?? b.bank ?? b.category ?? 'unknown');
      const pass  = num(b.passed ?? b.pass ?? 0);
      const total = num(b.total  ?? 0);
      if (total > 0) {
        const prev = map.get(name) ?? { pass: 0, total: 0 };
        map.set(name, { pass: prev.pass + pass, total: prev.total + total });
      }
    }
  }

  const bycat = report.by_category ?? report.by_bank_map ?? null;
  if (bycat && typeof bycat === 'object') {
    for (const [name, v] of Object.entries(bycat)) {
      const pass  = num(v.passed ?? v.pass ?? 0);
      const total = num(v.total  ?? 0);
      if (total > 0) {
        const prev = map.get(name) ?? { pass: 0, total: 0 };
        map.set(name, { pass: prev.pass + pass, total: prev.total + total });
      }
    }
  }
  return map;
}

/**
 * Extract adversarial category stats from a report.
 * Returns a map of categoryName → {pass, total}.
 * @param {Object} report
 * @returns {Map<string, {pass: number, total: number}>}
 */
function extractAdversarial(report) {
  const map = new Map();
  const adv = report.adversarial ?? report.adversarial_coverage ?? [];

  if (Array.isArray(adv)) {
    for (const a of adv) {
      const name  = String(a.category ?? a.name ?? 'unknown');
      const pass  = num(a.passed ?? a.pass ?? 0);
      const total = num(a.total  ?? 0);
      if (total > 0) {
        const prev = map.get(name) ?? { pass: 0, total: 0 };
        map.set(name, { pass: prev.pass + pass, total: prev.total + total });
      }
    }
  }

  // Fallback: banks whose names contain "adversar"
  const banks = report.banks ?? [];
  if (Array.isArray(banks)) {
    for (const b of banks) {
      const name = String(b.name ?? '');
      if (name.toLowerCase().includes('adversar')) {
        const pass  = num(b.passed ?? 0);
        const total = num(b.total  ?? 0);
        if (total > 0) {
          const prev = map.get(name) ?? { pass: 0, total: 0 };
          map.set(name, { pass: prev.pass + pass, total: prev.total + total });
        }
      }
    }
  }
  return map;
}

function num(v) {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

function pct(pass, total) {
  return total > 0 ? Math.round((pass / total) * 100) : 0;
}

// ── Rendering ─────────────────────────────────────────────────────────────────

function render() {
  if (sessions.length === 0) {
    $('dashboard').style.display   = 'none';
    $('empty-state').style.display = '';
    return;
  }
  $('empty-state').style.display = 'none';
  $('dashboard').style.display   = '';

  let totalPass = 0, totalFail = 0, totalSkip = 0, totalAll = 0;
  const trendLabels = [];
  const trendRates  = [];

  /** @type {Map<string, {pass: number, total: number}>} */
  const bankAgg = new Map();
  /** @type {Map<string, {pass: number, total: number}>} */
  const advAgg  = new Map();

  for (const { session, report } of sessions) {
    const { pass, fail, skip, total } = extractCounts(report);
    totalPass += pass;
    totalFail += fail;
    totalSkip += skip;
    totalAll  += total;

    trendLabels.push(session);
    trendRates.push(pct(pass, total));

    for (const [name, v] of extractBanks(report)) {
      const prev = bankAgg.get(name) ?? { pass: 0, total: 0 };
      bankAgg.set(name, { pass: prev.pass + v.pass, total: prev.total + v.total });
    }
    for (const [name, v] of extractAdversarial(report)) {
      const prev = advAgg.get(name) ?? { pass: 0, total: 0 };
      advAgg.set(name, { pass: prev.pass + v.pass, total: prev.total + v.total });
    }
  }

  setText('s-sessions', sessions.length);
  setText('s-total',    totalAll);
  setText('s-pass',     totalPass);
  setText('s-fail',     totalFail);
  setText('s-skip',     totalSkip);
  setText('s-rate',     `${pct(totalPass, totalAll)}%`);

  renderTrendChart(trendLabels, trendRates);
  renderBankChart(bankAgg);
  renderAdvTable(advAgg);
}

function chartDefaults() {
  return {
    animation: false,
    responsive: true,
    plugins: { legend: { labels: { color: '#94a3b8' } } },
  };
}

function renderTrendChart(labels, data) {
  const ctx = $('pass-rate-trend').getContext('2d');
  if (trendChart) trendChart.destroy();
  trendChart = new Chart(ctx, {
    type: 'line',
    data: {
      labels,
      datasets: [{
        label: 'Pass rate %',
        data,
        borderColor:     '#3b82f6',
        backgroundColor: 'rgba(59,130,246,0.12)',
        tension:          0.3,
        fill:             true,
        pointBackgroundColor: '#3b82f6',
      }],
    },
    options: {
      ...chartDefaults(),
      scales: {
        y: {
          min: 0, max: 100,
          grid:  { color: '#1e2a3a' },
          ticks: { color: '#64748b', callback: (v) => `${v}%` },
        },
        x: { grid: { color: '#1e2a3a' }, ticks: { color: '#64748b' } },
      },
      plugins: {
        ...chartDefaults().plugins,
        tooltip: { callbacks: { label: (c) => ` ${c.parsed.y}%` } },
      },
    },
  });
}

function renderBankChart(bankAgg) {
  const ctx = $('pass-rate-per-bank').getContext('2d');
  if (bankChart) bankChart.destroy();

  const entries = [...bankAgg.entries()]
    .sort((a, b) => b[1].total - a[1].total)
    .slice(0, 15);

  const labels = entries.map(([name]) => name);
  const data   = entries.map(([, v]) => pct(v.pass, v.total));
  const colors = data.map((p) =>
    p >= 90 ? '#4ade80' : p >= 60 ? '#facc15' : '#f87171'
  );

  bankChart = new Chart(ctx, {
    type: 'bar',
    data: {
      labels,
      datasets: [{
        label: 'Pass rate %',
        data,
        backgroundColor: colors,
        borderRadius:    4,
      }],
    },
    options: {
      ...chartDefaults(),
      indexAxis: 'y',
      scales: {
        x: {
          min: 0, max: 100,
          grid:  { color: '#1e2a3a' },
          ticks: { color: '#64748b', callback: (v) => `${v}%` },
        },
        y: {
          grid:  { color: 'transparent' },
          ticks: { color: '#94a3b8' },
        },
      },
      plugins: {
        legend:  { display: false },
        tooltip: { callbacks: { label: (c) => ` ${c.parsed.x}%` } },
      },
    },
  });
}

function renderAdvTable(advAgg) {
  const tbody = $('adv-table-body');
  if (!tbody) return;
  while (tbody.firstChild) tbody.removeChild(tbody.firstChild);

  if (advAgg.size === 0) {
    const tr = document.createElement('tr');
    const td = el('td', {
      text: 'No adversarial data found in loaded sessions.',
      style: { color: '#475569', textAlign: 'center', padding: '1.5rem' },
    });
    td.colSpan = 5;
    tr.appendChild(td);
    tbody.appendChild(tr);
    return;
  }

  const rows = [...advAgg.entries()]
    .sort((a, b) => pct(a[1].pass, a[1].total) - pct(b[1].pass, b[1].total));

  for (const [name, { pass, total }] of rows) {
    const rate  = pct(pass, total);
    const color = rate >= 90 ? '#4ade80' : rate >= 60 ? '#facc15' : '#f87171';
    const pillCls = rate >= 90
      ? 'pill pill-green' : rate >= 60
      ? 'pill pill-yellow' : 'pill pill-red';

    const tr = document.createElement('tr');

    const tdName = el('td', { text: name });

    const tdPass = el('td', { text: String(pass) });

    const tdTotal = el('td', { text: String(total) });

    const pillSpan = el('span', { text: `${rate}%`, cls: pillCls });
    const tdRate   = document.createElement('td');
    tdRate.appendChild(pillSpan);

    const barBg   = el('div', { cls: 'bar-bg' });
    const barFill = el('div', { cls: 'bar-fill' });
    barFill.style.width      = `${rate}%`;
    barFill.style.background = color;
    barBg.appendChild(barFill);
    const tdBar = el('td', { style: { width: '160px' } });
    tdBar.appendChild(barBg);

    tr.appendChild(tdName);
    tr.appendChild(tdPass);
    tr.appendChild(tdTotal);
    tr.appendChild(tdRate);
    tr.appendChild(tdBar);
    tbody.appendChild(tr);
  }
}

// ── File-picker wiring ────────────────────────────────────────────────────────

async function loadFiles(fileList) {
  const files = [...fileList].filter(
    (f) => f.name.endsWith('.json') || f.name === 'pipeline-report.json'
  );
  if (files.length === 0) {
    setStatus('No JSON files selected.');
    return;
  }
  setStatus(`Loading ${files.length} file(s)…`);
  sessions = [];

  const errors = [];
  await Promise.all(
    files.map(async (file, idx) => {
      try {
        const report = await readJSON(file);
        sessions.push({ session: sessionLabel(file, idx), report });
      } catch (err) {
        errors.push(err.message);
      }
    })
  );

  sessions.sort((a, b) => a.session.localeCompare(b.session));

  const parts = [`Loaded ${sessions.length} session(s).`];
  if (errors.length) parts.push(`${errors.length} error(s): ${errors.join('; ')}`);
  setStatus(parts.join(' '));
  render();
}

const filePicker = $('file-picker');
filePicker.addEventListener('change', () => loadFiles(filePicker.files));
$('btn-picker').addEventListener('click', () => filePicker.click());

// Directory picker via File System Access API (Chrome/Edge 86+)
const btnDir = $('btn-dir');
if (typeof window.showDirectoryPicker === 'function') {
  btnDir.style.display = '';
  btnDir.addEventListener('click', async () => {
    try {
      const dirHandle = await window.showDirectoryPicker({ mode: 'read' });
      const collected = [];
      await collectReportFiles(dirHandle, collected);
      if (collected.length === 0) {
        setStatus('No pipeline-report.json files found in selected directory.');
        return;
      }
      await loadFiles(collected);
    } catch (err) {
      if (err.name !== 'AbortError') setStatus(`Error: ${err.message}`);
    }
  });
}

/**
 * Recursively walk a FileSystemDirectoryHandle and collect all
 * pipeline-report.json File objects.
 * @param {FileSystemDirectoryHandle} dirHandle
 * @param {File[]} collected
 */
async function collectReportFiles(dirHandle, collected) {
  for await (const [name, handle] of dirHandle) {
    if (handle.kind === 'directory') {
      await collectReportFiles(handle, collected);
    } else if (name === 'pipeline-report.json') {
      try {
        const file = await handle.getFile();
        collected.push(file);
      } catch (_) { /* skip unreadable files */ }
    }
  }
}
