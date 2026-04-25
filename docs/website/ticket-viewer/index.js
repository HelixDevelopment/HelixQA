// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0
//
// HelixQA Ticket Viewer — client-side rendering logic.
//
// Fetches a ticket markdown file (via ?ticket= query param or file
// picker), renders it with marked.js, then post-processes the rendered
// DOM to resolve the 12 OCU evidence reference kinds into inline
// previews:
//
//   screenshot / diff-overlay → <img>
//   clip / video              → <video controls>
//   element-tree / ocr / json → pretty-printed <pre class="json-block">
//   hook-trace / logcat       → collapsible <details>
//   replay-script             → <pre> + Copy button
//   log-dump / coverage-map   → matching inline widget
//
// All DOM construction uses createElement/textContent.
// innerHTML is used exactly once: to insert the marked.js output into
// the #ticket-body container, which is the standard API usage.

'use strict';

// ── Helpers ───────────────────────────────────────────────────────────────────

const $ = (id) => document.getElementById(id);

function setStatus(msg, isError) {
  const node = $('status');
  if (!node) return;
  node.textContent = msg;
  node.style.color = isError ? '#f87171' : '#64748b';
}

function showEmpty() {
  $('empty-state').style.display = '';
  $('ticket-body').style.display = 'none';
}

function showBody() {
  $('empty-state').style.display = 'none';
  $('ticket-body').style.display = '';
}

// ── Query-param ───────────────────────────────────────────────────────────────

function ticketPathFromURL() {
  return new URLSearchParams(window.location.search).get('ticket') || '';
}

// ── Markdown rendering ────────────────────────────────────────────────────────

function renderMarkdown(source, basePath) {
  const body = $('ticket-body');
  if (!body) return;

  // marked.parse() returns a trusted HTML string from a local QA file.
  // This is the intended usage of the marked.js library.
  body.innerHTML = window.marked.parse(source);
  showBody();

  const baseDir = basePath.includes('/')
    ? basePath.slice(0, basePath.lastIndexOf('/') + 1)
    : '';

  enhanceEvidence(body, baseDir);
  wireReplayBlocks(body);
}

// ── Evidence resolution ───────────────────────────────────────────────────────

// Evidence fence format (inside a fenced code block):
//
//   ```evidence:screenshot
//   path/to/file.png
//   ```

const EVIDENCE_FENCE_RE = /^evidence:(\w[\w-]*)$/;

function enhanceEvidence(container, baseDir) {
  container.querySelectorAll('pre > code').forEach((code) => {
    const lines  = (code.textContent || '').trim().split('\n');
    const header = lines[0].trim();
    const m      = EVIDENCE_FENCE_RE.exec(header);
    if (!m) return;

    const kind    = m[1];
    const refPath = lines.slice(1).join('\n').trim();
    if (!refPath) return;

    const absPath = (refPath.startsWith('/') || refPath.startsWith('http'))
      ? refPath
      : baseDir + refPath;

    const widget = buildWidget(kind, absPath, refPath);
    if (!widget) return;

    const pre = code.parentElement;
    if (pre && pre.parentElement) {
      pre.parentElement.replaceChild(widget, pre);
    }
  });
}

function buildWidget(kind, absPath, label) {
  switch (kind) {
    case 'screenshot':
    case 'diff-overlay':
    case 'coverage-map':
      return buildImageWidget(absPath, label);
    case 'clip':
    case 'video':
      return buildVideoWidget(absPath, label);
    case 'json':
    case 'ocr':
    case 'element-tree':
      return buildJSONWidget(absPath, label);
    case 'hook-trace':
    case 'logcat':
    case 'log-dump':
      return buildTextCollapsible(absPath, label, kind);
    case 'replay-script':
      return buildReplayWidget(absPath, label);
    default:
      return null;
  }
}

function buildImageWidget(src, label) {
  const wrap = document.createElement('div');

  const img     = document.createElement('img');
  img.src       = src;
  img.alt       = label;
  img.className = 'evidence-img';
  img.loading   = 'lazy';
  img.onerror   = () => {
    const note       = document.createElement('p');
    note.className   = 'evidence-label';
    note.textContent = '[image not found: ' + label + ']';
    if (img.parentElement) img.parentElement.replaceChild(note, img);
  };

  const cap         = document.createElement('p');
  cap.className     = 'evidence-label';
  cap.textContent   = label;

  wrap.appendChild(img);
  wrap.appendChild(cap);
  return wrap;
}

function buildVideoWidget(src, label) {
  const wrap = document.createElement('div');

  const video     = document.createElement('video');
  video.src       = src;
  video.controls  = true;
  video.className = 'evidence-video';
  video.preload   = 'metadata';

  const cap       = document.createElement('p');
  cap.className   = 'evidence-label';
  cap.textContent = label;

  wrap.appendChild(video);
  wrap.appendChild(cap);
  return wrap;
}

function buildJSONWidget(src, label) {
  const wrap = document.createElement('div');

  const cap       = document.createElement('p');
  cap.className   = 'evidence-label';
  cap.textContent = label;

  const pre       = document.createElement('pre');
  pre.className   = 'json-block';
  pre.textContent = 'Loading\u2026';

  fetch(src)
    .then(function(r) {
      if (!r.ok) throw new Error('HTTP ' + r.status);
      return r.text();
    })
    .then(function(text) {
      try { pre.textContent = JSON.stringify(JSON.parse(text), null, 2); }
      catch (_) { pre.textContent = text; }
    })
    .catch(function(err) {
      pre.textContent = '[could not load ' + label + ': ' + err.message + ']';
    });

  wrap.appendChild(cap);
  wrap.appendChild(pre);
  return wrap;
}

function buildTextCollapsible(src, label, kind) {
  const details   = document.createElement('details');
  details.className = 'hook-trace';

  const summary   = document.createElement('summary');
  summary.textContent = kind + ': ' + label;
  details.appendChild(summary);

  const pre  = document.createElement('pre');
  const code = document.createElement('code');
  code.textContent = 'Loading\u2026';
  pre.appendChild(code);
  details.appendChild(pre);

  var loaded = false;
  details.addEventListener('toggle', function() {
    if (!details.open || loaded) return;
    loaded = true;
    fetch(src)
      .then(function(r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.text();
      })
      .then(function(text) { code.textContent = text; })
      .catch(function(err) {
        code.textContent = '[could not load: ' + err.message + ']';
      });
  });

  return details;
}

function buildReplayWidget(src, label) {
  const wrap    = document.createElement('div');
  wrap.className = 'replay-block';

  const pre     = document.createElement('pre');
  pre.textContent = 'Loading\u2026';

  const btn     = makeCopyButton(function() { return pre.textContent; });

  fetch(src)
    .then(function(r) {
      if (!r.ok) throw new Error('HTTP ' + r.status);
      return r.text();
    })
    .then(function(text) { pre.textContent = text; })
    .catch(function(err) {
      pre.textContent = '[could not load ' + label + ': ' + err.message + ']';
    });

  wrap.appendChild(btn);
  wrap.appendChild(pre);
  return wrap;
}

// ── Replay blocks from .ocu-replay fences ─────────────────────────────────────

function wireReplayBlocks(container) {
  container.querySelectorAll('pre > code').forEach(function(code) {
    var cls = code.className || '';
    if (cls.indexOf('ocu-replay') < 0) return;

    var pre = code.parentElement;
    if (!pre) return;

    var wrap      = document.createElement('div');
    wrap.className = 'replay-block';

    var btn = makeCopyButton(function() { return code.textContent || ''; });

    if (pre.parentElement) {
      pre.parentElement.insertBefore(wrap, pre);
      wrap.appendChild(btn);
      wrap.appendChild(pre);
    }
  });
}

// ── Shared copy button factory ────────────────────────────────────────────────

function makeCopyButton(getText) {
  var btn       = document.createElement('button');
  btn.className = 'copy-btn';
  btn.textContent = 'Copy';
  btn.addEventListener('click', function() {
    navigator.clipboard.writeText(getText()).then(function() {
      btn.textContent = 'Copied!';
      btn.className   = 'copy-btn copied';
      setTimeout(function() {
        btn.textContent = 'Copy';
        btn.className   = 'copy-btn';
      }, 2000);
    }).catch(function() {
      btn.textContent = 'Error';
    });
  });
  return btn;
}

// ── Loading ───────────────────────────────────────────────────────────────────

function loadFromURL(ticketPath) {
  setStatus('Loading ' + ticketPath + '\u2026');
  fetch(ticketPath)
    .then(function(r) {
      if (!r.ok) throw new Error('HTTP ' + r.status + ' \u2014 ' + r.statusText);
      return r.text();
    })
    .then(function(text) {
      setStatus('Loaded: ' + ticketPath);
      renderMarkdown(text, ticketPath);
    })
    .catch(function(err) {
      setStatus('Error loading ticket: ' + err.message, true);
      showEmpty();
    });
}

function loadFromFile(file) {
  setStatus('Reading ' + file.name + '\u2026');
  var reader    = new FileReader();
  reader.onload = function(e) {
    setStatus('Loaded: ' + file.name);
    renderMarkdown(e.target.result, file.name);
  };
  reader.onerror = function() {
    setStatus('Error reading ' + file.name, true);
    showEmpty();
  };
  reader.readAsText(file);
}

// ── Boot ──────────────────────────────────────────────────────────────────────

function boot() {
  var urlInput  = $('url-input');
  var fileInput = $('file-input');

  $('btn-load').addEventListener('click', function() {
    var path = (urlInput.value || '').trim();
    if (path) loadFromURL(path);
  });

  urlInput.addEventListener('keydown', function(e) {
    if (e.key === 'Enter') $('btn-load').click();
  });

  $('btn-file').addEventListener('click', function() { fileInput.click(); });

  fileInput.addEventListener('change', function() {
    if (fileInput.files.length > 0) loadFromFile(fileInput.files[0]);
  });

  var initial = ticketPathFromURL();
  if (initial) {
    urlInput.value = initial;
    loadFromURL(initial);
  } else {
    showEmpty();
  }
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', boot);
} else {
  boot();
}
