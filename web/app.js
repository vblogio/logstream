'use strict';

/* ================= Utilitaires ================= */

const $ = (sel) => document.querySelector(sel);
const PAGE_SIZE = 500;   // lignes chargées par recherche
const MAX_ROWS = 2000;   // lignes gardées à l'écran en mode direct
const RANGE_MS = { '5m': 3e5, '15m': 9e5, '1h': 36e5, '6h': 216e5, '24h': 864e5, '7d': 6048e5, '30d': 2592e6 };

const store = {
  get(k, def) { try { const v = localStorage.getItem('logstream.' + k); return v === null ? def : v; } catch { return def; } },
  set(k, v) { try { localStorage.setItem('logstream.' + k, v); } catch { /* stockage indisponible */ } },
};

const ESC = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ESC[c]);
const pad = (n, w = 2) => String(n).padStart(w, '0');
const debounce = (fn, ms) => { let t; return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); }; };
const fmtNum = (n) => Number(n || 0).toLocaleString('fr-FR');

// Date.parse n'accepte que 3 décimales partout : on tronque les nanosecondes.
const parseTime = (s) => new Date(String(s || '').replace(/(\.\d{3})\d+/, '$1'));

async function api(url, opts = {}) {
  if (opts.body && typeof opts.body !== 'string') {
    opts.body = JSON.stringify(opts.body);
    opts.headers = { 'Content-Type': 'application/json' };
  }
  const res = await fetch(url, opts);
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { /* réponse non JSON */ }
  if (!res.ok) throw new Error((data && data.error) || text || res.statusText);
  return data;
}

function toast(msg) {
  const el = $('#toast');
  // Une fenêtre modale passe au-dessus de tout : on y place le toast pour qu'il reste visible.
  const dlg = $('#tagsDlg');
  (dlg.open ? dlg : document.body).append(el);
  el.textContent = msg;
  el.hidden = false;
  clearTimeout(toast.t);
  toast.t = setTimeout(() => { el.hidden = true; }, 1800);
}

/* ================= État ================= */

const state = {
  rows: [],          // enregistrements affichés, du plus récent au plus ancien
  pending: [],       // messages reçus en direct pendant que l'on fait défiler
  tags: [],
  matchers: [],      // tags compilés + termes recherchés
  mode: store.get('mode', 'simple'),
  live: store.get('live', '1') === '1',
  es: null,
  reqId: 0,
};
const recOf = new WeakMap(); // élément .row -> enregistrement

const list = $('#list');

/* ================= Thème ================= */

function effectiveTheme() {
  const t = document.documentElement.dataset.theme;
  if (t) return t;
  return matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}
$('#themeBtn').addEventListener('click', () => {
  const next = effectiveTheme() === 'dark' ? 'light' : 'dark';
  document.documentElement.dataset.theme = next;
  store.set('theme', next);
});

/* ================= Filtres ================= */

function params() {
  const p = new URLSearchParams();
  const q = $('#q').value.trim();
  if (q) p.set('q', q);
  p.set('mode', state.mode);
  p.set('range', $('#range').value);
  for (const id of ['severity', 'host', 'app']) {
    const v = $('#' + id).value;
    if (v !== '') p.set(id, v);
  }
  return p;
}

function liveAvailable() {
  return !(state.mode === 'logsql' && $('#q').value.trim());
}

function syncControls() {
  const logsql = state.mode === 'logsql';
  $('#mode').textContent = logsql ? 'LogsQL' : 'Simple';
  $('#mode').classList.toggle('on', logsql);
  $('#q').placeholder = logsql
    ? 'LogsQL : error AND host:web-01 | sort by (_time)'
    : 'Rechercher…  error -timeout "disk full"   (touche /)';
  const btn = $('#live');
  btn.disabled = !liveAvailable();
  btn.classList.toggle('on', state.live && liveAvailable());
  btn.title = liveAvailable() ? 'Afficher les nouveaux messages en direct' : 'Le direct n\'est pas disponible en mode LogsQL';
  for (const id of ['severity', 'host', 'app']) $('#' + id).classList.toggle('set', $('#' + id).value !== '');
}

/* ================= Tags et surlignage ================= */

// Couleur de texte lisible (noir ou blanc) selon la luminosité du fond.
function textOn(hex) {
  const n = parseInt(hex.slice(1), 16);
  const yiq = (((n >> 16) & 255) * 299 + ((n >> 8) & 255) * 587 + (n & 255) * 114) / 1000;
  return yiq >= 150 ? '#111827' : '#ffffff';
}
const tagStyle = (color) => `background:${color};color:${textOn(color)}`;
const escapeRe = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

function compileMatchers() {
  const out = [];
  for (const t of state.tags) {
    if (!t.enabled || !t.pattern) continue;
    let src = t.regex ? t.pattern : escapeRe(t.pattern);
    if (t.wholeWord) src = `(?<![\\p{L}\\p{N}_])(?:${src})(?![\\p{L}\\p{N}_])`;
    try {
      out.push({ re: new RegExp(src, 'gu' + (t.caseSensitive ? '' : 'i')), html: `<mark class="tag" style="${tagStyle(t.color)}">` });
    } catch (e) {
      console.warn('Tag ignoré (expression invalide) :', t.pattern, e.message);
    }
  }
  // Les termes de la recherche simple sont soulignés discrètement, avec la priorité la plus basse.
  if (state.mode === 'simple') {
    for (const tok of $('#q').value.match(/-?"[^"]*"|\S+/g) || []) {
      if (tok.startsWith('-')) continue;
      const word = tok.replace(/^"|"$/g, '');
      if (word) out.push({ re: new RegExp(escapeRe(word), 'giu'), html: '<mark class="hit">' });
    }
  }
  state.matchers = out;
}

function highlight(text) {
  text = String(text ?? '');
  if (!state.matchers.length || !text) return esc(text);
  const spans = [];
  state.matchers.forEach((m, prio) => {
    m.re.lastIndex = 0;
    let x;
    while ((x = m.re.exec(text)) !== null) {
      if (x[0] === '') { m.re.lastIndex++; continue; }
      spans.push({ s: x.index, e: x.index + x[0].length, prio, html: m.html });
    }
  });
  if (!spans.length) return esc(text);
  spans.sort((a, b) => a.s - b.s || a.prio - b.prio);
  let out = '';
  let pos = 0;
  for (const sp of spans) {
    if (sp.s < pos) continue; // chevauchement : le premier arrivé l'emporte
    out += esc(text.slice(pos, sp.s)) + sp.html + esc(text.slice(sp.s, sp.e)) + '</mark>';
    pos = sp.e;
  }
  return out + esc(text.slice(pos));
}

/* ================= Rendu de la liste ================= */

const SEV_CLASS = { emerg: 'crit', alert: 'crit', crit: 'crit', err: 'err', warning: 'warning', notice: 'notice', info: 'info', debug: 'debug' };

function fmtTime(s) {
  const d = parseTime(s);
  if (isNaN(d)) return esc(s);
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}<span class="ms">.${pad(d.getMilliseconds(), 3)}</span>`;
}

function rowHTML(r, isNew) {
  const sev = r.severity || '';
  const cls = SEV_CLASS[sev] || 'info';
  return `<article class="row sev-${cls}${isNew ? ' new' : ''}" tabindex="0">`
    + `<time>${fmtTime(r._time)}</time>`
    + `<span class="sev">${esc(sev || '—')}</span>`
    + `<span class="host" title="${esc(r.host)}">${esc(r.host)}</span>`
    + `<span class="app" title="${esc(r.app)}">${esc(r.app)}</span>`
    + `<div class="msg">${highlight(r._msg)}</div>`
    + '</article>';
}

// Associe les n premiers (ou derniers) éléments .row à leurs enregistrements.
function bindRows(first, rows) {
  let el = first;
  for (const r of rows) {
    if (!el) break;
    recOf.set(el, r);
    el = el.nextElementSibling;
  }
}

function renderList() {
  list.innerHTML = state.rows.map((r) => rowHTML(r, false)).join('');
  bindRows(list.firstElementChild, state.rows);
  $('#empty').hidden = state.rows.length > 0;
}

function rehighlight() {
  for (const row of list.children) {
    const r = recOf.get(row);
    if (r) row.querySelector('.msg').innerHTML = highlight(r._msg);
  }
}

function prependRows(rows) {
  if (!rows.length) return;
  list.insertAdjacentHTML('afterbegin', rows.map((r) => rowHTML(r, true)).join(''));
  bindRows(list.firstElementChild, rows);
  state.rows = rows.concat(state.rows);
  while (state.rows.length > MAX_ROWS) {
    state.rows.pop();
    list.lastElementChild?.remove();
  }
  $('#empty').hidden = true;
  updateCount();
}

function updateCount(tookMs) {
  const n = state.rows.length;
  const took = tookMs != null ? ` · ${tookMs} ms` : '';
  $('#count').textContent = `${fmtNum(n)} ${n > 1 ? 'logs affichés' : 'log affiché'}${took}`;
}

function detailsHTML(r) {
  const order = ['_time', 'host', 'app', 'procid', 'severity', 'facility', 'source', 'proto', 'sd', '_msg'];
  const hidden = new Set(['_stream_id', '_stream', 'sevnum']);
  const keys = order.filter((k) => r[k] != null && r[k] !== '')
    .concat(Object.keys(r).filter((k) => !order.includes(k) && !hidden.has(k)).sort());
  const label = { _time: 'horodatage', _msg: 'message', procid: 'pid', sd: 'données structurées' };
  const val = (k) => (k === '_time' ? `${esc(parseTime(r[k]).toLocaleString('fr-FR'))} <span class="muted">(${esc(r[k])})</span>` : esc(r[k]));
  return '<div class="details"><dl>'
    + keys.map((k) => `<dt>${esc(label[k] || k)}</dt><dd>${val(k)}</dd>`).join('')
    + '</dl><div class="acts">'
    + (r.host ? '<button class="btn" data-act="host">Filtrer sur cet hôte</button>' : '')
    + (r.app ? '<button class="btn" data-act="app">Filtrer sur cette appli</button>' : '')
    + '<button class="btn" data-act="copy">Copier le message</button>'
    + '<button class="btn" data-act="json">Copier en JSON</button>'
    + '</div></div>';
}

async function copy(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // Repli pour les pages servies en HTTP simple (hors localhost).
    const ta = Object.assign(document.createElement('textarea'), { value: text });
    document.body.append(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
  }
  toast('Copié');
}

function setSelect(id, value) {
  const sel = $('#' + id);
  if (![...sel.options].some((o) => o.value === value)) sel.add(new Option(value, value));
  sel.value = value;
}

list.addEventListener('click', (ev) => {
  const row = ev.target.closest('.row');
  if (!row) return;
  const r = recOf.get(row);
  const act = ev.target.closest('[data-act]');
  if (act) {
    const a = act.dataset.act;
    if (a === 'host' || a === 'app') { setSelect(a, r[a]); onFilterChange(); }
    if (a === 'copy') copy(r._msg || '');
    if (a === 'json') copy(JSON.stringify(r, null, 2));
    return;
  }
  if (ev.target.closest('.details')) return;
  if (String(window.getSelection())) return; // on sélectionne du texte : ne pas replier
  const open = row.classList.toggle('open');
  const det = row.querySelector('.details');
  if (open && !det) row.insertAdjacentHTML('beforeend', detailsHTML(r));
  else if (det) det.hidden = !open;
});
list.addEventListener('keydown', (ev) => {
  if (ev.key === 'Enter' && ev.target.classList.contains('row')) ev.target.click();
});

/* ================= Histogramme ================= */

function renderHisto(h) {
  const el = $('#histo');
  if (!h || !h.buckets) { el.innerHTML = ''; return; }
  const stepMs = h.step * 1000;
  const counts = new Map();
  for (const b of h.buckets) counts.set(Math.floor(parseTime(b.t).getTime() / stepMs), b.n);

  const span = RANGE_MS[$('#range').value];
  const end = Math.floor(Date.now() / stepMs);
  let start = span ? Math.floor((Date.now() - span) / stepMs) + 1 : (counts.size ? Math.min(...counts.keys()) : end);
  if (end - start > 400) start = end - 400;

  let max = 1;
  for (let k = start; k <= end; k++) max = Math.max(max, counts.get(k) || 0);
  const label = (k) => {
    const d = new Date(k * stepMs);
    return stepMs >= 864e5 ? d.toLocaleDateString('fr-FR') : d.toLocaleString('fr-FR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
  };
  let bars = '';
  for (let k = start; k <= end; k++) {
    const n = counts.get(k) || 0;
    const hgt = n ? Math.max(4, (n / max) * 100) : 0;
    bars += `<i style="height:${hgt}%" title="${label(k)} : ${fmtNum(n)}"></i>`;
  }
  el.innerHTML = `<div class="bars">${bars}</div><div class="axis"><span>${label(start)}</span><span>max ${fmtNum(max)} / intervalle</span><span>${label(end)}</span></div>`;
}

async function refreshHisto() {
  try { renderHisto(await api('/api/histogram?' + params())); } catch { renderHisto(null); }
}

/* ================= Recherche ================= */

function setError(msg) {
  const el = $('#error');
  el.textContent = msg || '';
  el.hidden = !msg;
}

async function refresh() {
  const id = ++state.reqId;
  syncControls();
  compileMatchers();
  document.body.classList.add('busy');
  connectLive();
  try {
    const res = await api(`/api/logs?${params()}&limit=${PAGE_SIZE}`);
    if (id !== state.reqId) return;
    state.rows = res.rows || [];
    state.pending = [];
    $('#newPill').hidden = true;
    renderList();
    updateCount(res.took_ms);
    setError(null);
  } catch (e) {
    if (id !== state.reqId) return;
    state.rows = [];
    renderList();
    $('#empty').hidden = true;
    $('#count').textContent = '';
    setError('Erreur de recherche : ' + e.message);
  } finally {
    if (id === state.reqId) document.body.classList.remove('busy');
  }
  refreshHisto();
}

const onFilterChange = () => {
  store.set('range', $('#range').value);
  store.set('severity', $('#severity').value);
  refresh();
};

$('#q').addEventListener('input', debounce(refresh, 350));
$('#q').addEventListener('keydown', (ev) => {
  if (ev.key === 'Enter') refresh();
  if (ev.key === 'Escape') { ev.target.value = ''; refresh(); }
});
for (const id of ['range', 'severity', 'host', 'app']) $('#' + id).addEventListener('change', onFilterChange);

$('#mode').addEventListener('click', () => {
  state.mode = state.mode === 'simple' ? 'logsql' : 'simple';
  store.set('mode', state.mode);
  refresh();
  $('#q').focus();
});

document.addEventListener('keydown', (ev) => {
  if (ev.key === '/' && !/INPUT|TEXTAREA|SELECT/.test(document.activeElement.tagName) && !$('#tagsDlg').open) {
    ev.preventDefault();
    $('#q').focus();
  }
});

/* ================= Direct (SSE) ================= */

function setConn(cls, text) {
  const el = $('#conn');
  el.className = 'conn ' + cls;
  el.textContent = text;
}

function connectLive() {
  if (state.es) { state.es.close(); state.es = null; }
  if (!state.live || !liveAvailable()) {
    setConn('', state.live ? 'direct indisponible (LogsQL)' : 'direct en pause');
    return;
  }
  const es = new EventSource('/api/stream?' + params());
  state.es = es;
  setConn('', 'connexion…');
  es.onopen = () => setConn('ok', 'direct');
  es.onerror = () => setConn('ko', 'reconnexion…');
  es.onmessage = (ev) => {
    const data = JSON.parse(ev.data);
    const rows = (data.rows || []).reverse(); // le plus récent en premier
    if (data.skipped) toast(`${fmtNum(data.skipped)} messages non affichés (débit trop élevé)`);
    if (window.scrollY > 80) {
      state.pending = rows.concat(state.pending).slice(0, MAX_ROWS);
      const pill = $('#newPill');
      pill.textContent = `↑ ${fmtNum(state.pending.length)} nouveau${state.pending.length > 1 ? 'x' : ''} log${state.pending.length > 1 ? 's' : ''}`;
      pill.hidden = false;
    } else {
      prependRows(rows);
    }
  };
}

function flushPending() {
  if (!state.pending.length) return;
  prependRows(state.pending);
  state.pending = [];
  $('#newPill').hidden = true;
}

$('#newPill').addEventListener('click', () => {
  flushPending();
  window.scrollTo({ top: 0, behavior: 'smooth' });
});
window.addEventListener('scroll', debounce(() => { if (window.scrollY <= 80) flushPending(); }, 150), { passive: true });

$('#live').addEventListener('click', () => {
  state.live = !state.live;
  store.set('live', state.live ? '1' : '0');
  if (state.live) refresh(); // rattrape ce qui a été reçu pendant la pause
  else { syncControls(); connectLive(); }
});

/* ================= Listes hôtes / applis, statistiques ================= */

async function loadFacets() {
  try {
    const f = await api('/api/facets');
    for (const id of ['host', 'app']) {
      const sel = $('#' + id);
      const cur = sel.value;
      const first = sel.options[0];
      sel.replaceChildren(first, ...(f[id] || []).map((v) => new Option(v, v)));
      if (cur) setSelect(id, cur);
    }
  } catch { /* VictoriaLogs pas encore prêt : on réessaiera */ }
}

async function loadStats() {
  try {
    const s = await api('/api/stats');
    const err = s.lastError ? ` · <span class="bad">stockage : ${esc(s.lastError)}</span>` : '';
    const dropped = s.dropped ? `<span class="bad">perdus ${fmtNum(s.dropped)}</span>` : 'perdus 0';
    $('#stats').innerHTML = `reçus ${fmtNum(s.received)} · stockés ${fmtNum(s.ingested)} · ${dropped} · file ${fmtNum(s.queue)}${err}`;
  } catch {
    $('#stats').textContent = 'serveur injoignable';
  }
}

/* ================= Gestion des tags ================= */

const PALETTE = ['#6366f1', '#0ea5e9', '#14b8a6', '#a855f7', '#ec4899', '#eab308', '#64748b', '#f97316'];

async function loadTags() {
  try { state.tags = await api('/api/tags'); } catch (e) { toast('Tags : ' + e.message); }
  compileMatchers();
}

function tagRowHTML(t) {
  const opt = (field, label, title) =>
    `<label class="opt" title="${title}"><input type="checkbox" data-f="${field}"${t[field] ? ' checked' : ''}>${label}</label>`;
  return `<div class="tag-row${t.enabled ? '' : ' off'}" data-id="${esc(t.id)}">
    <input type="color" value="${esc(t.color)}" data-f="color" aria-label="Couleur">
    <input type="text" value="${esc(t.pattern)}" data-f="pattern" placeholder="mot-clé" spellcheck="false" aria-label="Mot-clé">
    <span class="preview"><mark class="tag" style="${tagStyle(t.color)}">${esc(t.pattern || 'aperçu')}</mark></span>
    <div class="opts">
      ${opt('wholeWord', 'mot entier', 'Ne colorer que les mots entiers (évite « ok » dans « token »)')}
      ${opt('caseSensitive', 'Aa', 'Respecter la casse')}
      ${opt('regex', '.*', 'Expression régulière')}
      ${opt('enabled', 'actif', 'Activer ce tag')}
    </div>
    <button class="icon-btn danger" type="button" data-del title="Supprimer" aria-label="Supprimer">
      <svg viewBox="0 0 24 24"><path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/></svg>
    </button>
    <div class="err"></div>
  </div>`;
}

function renderTagList() {
  $('#tagList').innerHTML = state.tags.map(tagRowHTML).join('') || '<p class="muted small">Aucun tag.</p>';
}

const saveTimers = {};
function scheduleSave(t, rowEl) {
  clearTimeout(saveTimers[t.id]);
  saveTimers[t.id] = setTimeout(async () => {
    const errEl = rowEl.querySelector('.err');
    try {
      await api('/api/tags/' + encodeURIComponent(t.id), { method: 'PUT', body: t });
      errEl.textContent = '';
    } catch (e) {
      errEl.textContent = e.message;
    }
  }, 400);
}

const applyTags = debounce(() => { compileMatchers(); rehighlight(); }, 150);

$('#tagList').addEventListener('input', (ev) => {
  const rowEl = ev.target.closest('.tag-row');
  const field = ev.target.dataset.f;
  if (!rowEl || !field) return;
  const t = state.tags.find((x) => x.id === rowEl.dataset.id);
  if (!t) return;
  t[field] = ev.target.type === 'checkbox' ? ev.target.checked : ev.target.value;
  rowEl.classList.toggle('off', !t.enabled);
  const mark = rowEl.querySelector('.preview mark');
  mark.textContent = t.pattern || 'aperçu';
  mark.setAttribute('style', tagStyle(t.color));
  scheduleSave(t, rowEl);
  applyTags();
});

$('#tagList').addEventListener('click', async (ev) => {
  const del = ev.target.closest('[data-del]');
  if (!del) return;
  const rowEl = del.closest('.tag-row');
  const t = state.tags.find((x) => x.id === rowEl.dataset.id);
  if (!t || !confirm(`Supprimer le tag « ${t.pattern} » ?`)) return;
  try {
    await api('/api/tags/' + encodeURIComponent(t.id), { method: 'DELETE' });
    state.tags = state.tags.filter((x) => x !== t);
    renderTagList();
    applyTags();
  } catch (e) { toast(e.message); }
});

$('#addTag').addEventListener('click', async () => {
  const color = PALETTE[state.tags.length % PALETTE.length];
  try {
    const t = await api('/api/tags', { method: 'POST', body: { pattern: 'nouveau', color, wholeWord: true, enabled: true } });
    state.tags.push(t);
    renderTagList();
    applyTags();
    const input = $('#tagList').lastElementChild.querySelector('input[type="text"]');
    input.focus();
    input.select();
  } catch (e) { toast(e.message); }
});

$('#resetTags').addEventListener('click', async () => {
  if (!confirm('Remplacer tous les tags par les tags par défaut (warning, error, ok) ?')) return;
  try {
    state.tags = await api('/api/tags/reset', { method: 'POST' });
    renderTagList();
    applyTags();
  } catch (e) { toast(e.message); }
});

$('#tagsBtn').addEventListener('click', () => { renderTagList(); $('#tagsDlg').showModal(); });
$('#closeTags').addEventListener('click', () => $('#tagsDlg').close());
$('#tagsDlg').addEventListener('click', (ev) => { if (ev.target === ev.currentTarget) ev.currentTarget.close(); });

/* ================= Démarrage ================= */

$('#range').value = store.get('range', '1h');
if (!$('#range').value) $('#range').value = '1h';
$('#severity').value = store.get('severity', '');

(async () => {
  await loadTags();
  loadFacets();
  loadStats();
  refresh();
  setInterval(loadStats, 5000);
  setInterval(loadFacets, 60000);
  setInterval(() => { if (state.live && liveAvailable() && !document.hidden) refreshHisto(); }, 30000);
})();
