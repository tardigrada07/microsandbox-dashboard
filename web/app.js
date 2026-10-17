'use strict';

const $ = (s) => document.querySelector(s);
const state = { boxes: [], metrics: {}, selected: null, editing: null };

// h builds a DOM element: on* props become listeners, data-* become attributes,
// children are appended as text (never parsed as HTML, so untrusted names are safe).
function h(tag, props, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else if (k === 'class') el.className = v;
    else if (k.startsWith('data-')) el.setAttribute(k, v);
    else throw new Error(`h(): unsupported prop ${k}`); // allowlist: no arbitrary property writes
  }
  for (const k of kids.flat()) el.append(k instanceof Node ? k : String(k ?? ''));
  return el;
}

const mib = (b) => (b / 1048576).toFixed(0) + ' MiB';
const fmtBytes = (b) => b > 1 << 30 ? (b / (1 << 30)).toFixed(2) + ' GiB' : mib(b);
const fmtUp = (s) => s >= 3600 ? Math.floor(s / 3600) + 'h ' + Math.floor(s % 3600 / 60) + 'm' : s >= 60 ? Math.floor(s / 60) + 'm ' + Math.floor(s % 60) + 's' : Math.floor(s) + 's';

function showError(msg) {
  const e = $('#error');
  e.hidden = !msg;
  e.textContent = msg || '';
}

// sbUrl builds a sandbox API path; the name is encoded so it can never alter the path.
const sbUrl = (name, action = '') => '/api/sandboxes/' + encodeURIComponent(name) + action;

// api calls the dashboard's JSON API; non-2xx responses throw with the server's error text.
async function api(method, url, body) {
  const res = await fetch(url, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

// act runs a user action, shows any error in the banner, then reloads the list.
async function act(fn) {
  try { showError(''); await fn(); await refresh(); } catch (e) { showError(e.message); }
}

async function refresh() {
  try {
    state.boxes = await api('GET', '/api/sandboxes');
    state.metrics = await api('GET', '/api/metrics');
    showError('');
  } catch (e) { showError(e.message); }
  render();
}

function bar(frac) {
  const i = h('i');
  i.style.width = Math.min(100, Math.max(0, frac * 100)) + '%';
  return h('span', { class: 'bar' }, i);
}

function cpuNodes(b) {
  const m = state.metrics[b.name];
  return m ? [bar(m.cpuPercent / (100 * b.cpus)), m.cpuPercent.toFixed(0) + '%'] : ['-'];
}

function memNodes(b) {
  const m = state.metrics[b.name];
  return m ? [bar(m.memoryBytes / (m.memoryLimitBytes || 1)), mib(m.memoryBytes) + ' / ' + b.memoryMiB + ' MiB'] : ['-'];
}

// Live metrics only touch the usage cells, so buttons are not rebuilt under the cursor.
function updateMetrics() {
  for (const tr of $('#list tbody').children) {
    const b = state.boxes.find((x) => x.name === tr.dataset.name);
    if (!b) continue;
    tr.querySelector('.u-cpu').replaceChildren(...cpuNodes(b));
    tr.querySelector('.u-mem').replaceChildren(...memNodes(b));
  }
  renderDetail();
}

// render rebuilds the table and detail panel from state. Used after real changes;
// live metrics use updateMetrics so buttons are not replaced under the cursor.
function render() {
  const tbody = $('#list tbody');
  tbody.replaceChildren(...state.boxes.map((b) => {
    const running = b.status === 'running';
    return h('tr', { class: 'row', 'data-name': b.name, onclick: () => { state.selected = state.selected === b.name ? null : b.name; render(); } },
      h('td', {}, b.name), h('td', {}, b.image),
      h('td', {}, h('span', { class: 'badge ' + b.status }, b.status)),
      h('td', { class: 'u-cpu' }, cpuNodes(b)),
      h('td', { class: 'u-mem' }, memNodes(b)),
      h('td', {}, String(b.mounts.length)),
      h('td', {},
        running
          ? h('button', { onclick: (e) => { e.stopPropagation(); act(() => api('POST', sbUrl(b.name, "/stop"))); } }, 'Stop')
          : h('button', { onclick: (e) => { e.stopPropagation(); act(() => api('POST', sbUrl(b.name, "/start"))); } }, 'Start'),
        h('button', { onclick: (e) => { e.stopPropagation(); openForm(b); } }, 'Edit'),
        h('button', { class: 'danger', onclick: (e) => { e.stopPropagation(); del(b); } }, 'Delete')));
  }));
  $('#empty').hidden = state.boxes.length > 0;
  renderDetail();
}

function renderDetail() {
  const d = $('#detail');
  const b = state.boxes.find((x) => x.name === state.selected);
  d.hidden = !b;
  if (!b) return;
  const m = state.metrics[b.name];
  const env = Object.entries(b.env || {}).map(([k, v]) => `${k}=${v}`).join('\n') || '-';
  d.replaceChildren(
    h('h2', {}, b.name),
    h('dl', {},
      h('dt', {}, 'Image'), h('dd', {}, b.image),
      h('dt', {}, 'Status'), h('dd', {}, b.status),
      h('dt', {}, 'CPUs / Memory'), h('dd', {}, `${b.cpus} / ${b.memoryMiB} MiB`),
      h('dt', {}, 'Workdir'), h('dd', {}, b.workdir || '-'),
      h('dt', {}, 'Created'), h('dd', {}, new Date(b.createdAt).toLocaleString()),
      h('dt', {}, 'Env'), h('dd', {}, h('code', {}, env)),
      ...(m ? [
        h('dt', {}, 'Uptime'), h('dd', {}, fmtUp(m.uptimeSeconds)),
        h('dt', {}, 'CPU'), h('dd', {}, m.cpuPercent.toFixed(1) + '%'),
        h('dt', {}, 'Memory'), h('dd', {}, `${fmtBytes(m.memoryBytes)} of ${fmtBytes(m.memoryLimitBytes)}`),
        h('dt', {}, 'Disk read / write'), h('dd', {}, `${fmtBytes(m.diskReadBytes)} / ${fmtBytes(m.diskWriteBytes)}`),
        h('dt', {}, 'Net rx / tx'), h('dd', {}, `${fmtBytes(m.netRxBytes)} / ${fmtBytes(m.netTxBytes)}`),
      ] : [])),
    h('h3', {}, 'Mounted directories'),
    b.mounts.length
      ? h('table', {},
          h('thead', {}, h('tr', {}, h('th', {}, 'Host / source'), h('th', {}, 'Guest path'), h('th', {}, 'Kind'), h('th', {}, 'Mode'))),
          h('tbody', {}, b.mounts.map((x) => h('tr', {},
            h('td', {}, h('code', {}, x.source || '-')), h('td', {}, h('code', {}, x.guestPath)),
            h('td', {}, x.kind), h('td', {}, x.readonly ? 'read-only' : 'read-write')))))
      : h('p', {}, 'None'));
}

function del(b) {
  if (!confirm(`Delete sandbox "${b.name}"? This cannot be undone.`)) return;
  act(async () => {
    if (b.status === 'running') await api('POST', sbUrl(b.name, "/stop"));
    await api('DELETE', sbUrl(b.name));
    if (state.selected === b.name) state.selected = null;
  });
}

// ---- create / edit form ----

const form = $('#form');
const dialog = $('#form-dialog');

function openForm(box) {
  state.editing = box || null;
  form.reset();
  $('#plan').hidden = true;
  $('#form-title').textContent = box ? `Edit ${box.name}` : 'New sandbox';
  $('#form-preview').hidden = !box;
  for (const n of ['name', 'image', 'mounts']) form.elements[n].disabled = !!box;
  if (box) {
    form.elements.name.value = box.name;
    form.elements.image.value = box.image;
    form.elements.cpus.value = box.cpus;
    form.elements.memoryMiB.value = box.memoryMiB;
    form.elements.workdir.value = box.workdir || '';
    form.elements.env.value = Object.entries(box.env || {}).map(([k, v]) => `${k}=${v}`).join('\n');
    form.elements.mounts.value = box.mounts.filter((m) => m.kind === 'bind').map((m) => `${m.source}:${m.guestPath}${m.readonly ? ':ro' : ''}`).join('\n');
  }
  dialog.showModal();
}

function parseEnv(text) {
  const env = {};
  for (const line of text.split('\n').map((l) => l.trim()).filter(Boolean)) {
    const i = line.indexOf('=');
    if (i < 1) throw new Error(`Bad env line: ${line}`);
    env[line.slice(0, i)] = line.slice(i + 1);
  }
  return env;
}

function parseMounts(text) {
  return text.split('\n').map((l) => l.trim()).filter(Boolean).map((line) => {
    const p = line.split(':');
    const readonly = p[p.length - 1] === 'ro';
    if (readonly) p.pop();
    if (p.length !== 2) throw new Error(`Bad mount line: ${line}`);
    return { hostPath: p[0], guestPath: p[1], readonly };
  });
}

// modifyBody turns the edit form into a PATCH body containing only what changed
// (env keys removed from the textarea go into envRemove).
function modifyBody(dryRun) {
  const f = form.elements;
  const old = state.editing;
  const env = parseEnv(f.env.value);
  const body = { dryRun, env, envRemove: Object.keys(old.env || {}).filter((k) => !(k in env)) };
  if (+f.cpus.value !== old.cpus) body.cpus = +f.cpus.value;
  if (+f.memoryMiB.value !== old.memoryMiB) body.memoryMiB = +f.memoryMiB.value;
  if (f.workdir.value !== (old.workdir || '')) body.workdir = f.workdir.value;
  return body;
}

$('#form-cancel').addEventListener('click', () => dialog.close());

$('#form-preview').addEventListener('click', async () => {
  try {
    const p = await api('PATCH', sbUrl(state.editing.name), modifyBody(true));
    const lines = [...p.changes, ...p.conflicts.map((c) => 'CONFLICT ' + c), ...p.warnings.map((w) => 'warning: ' + w)];
    $('#plan').textContent = lines.length ? lines.join('\n') : 'No changes.';
    $('#plan').hidden = false;
  } catch (e) { $('#plan').textContent = e.message; $('#plan').hidden = false; }
});

form.addEventListener('submit', (ev) => {
  ev.preventDefault();
  const f = form.elements;
  act(async () => {
    if (state.editing) {
      await api('PATCH', sbUrl(state.editing.name), modifyBody(false));
    } else {
      await api('POST', '/api/sandboxes', {
        name: f.name.value, image: f.image.value, cpus: +f.cpus.value, memoryMiB: +f.memoryMiB.value,
        workdir: f.workdir.value, env: parseEnv(f.env.value), mounts: parseMounts(f.mounts.value),
      });
    }
    dialog.close();
  });
});

$('#new-btn').addEventListener('click', () => openForm(null));

// ---- live metrics ----

function stream() {
  const es = new EventSource('/api/metrics/stream');
  es.onmessage = (e) => { state.metrics = JSON.parse(e.data); updateMetrics(); };
}

refresh().then(stream);
setInterval(refresh, 10000); // picks up status/config changes made outside the dashboard
