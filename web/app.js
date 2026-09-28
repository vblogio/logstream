'use strict';

/* ================= Translations ================= */

const I18N = {
  en: {
    locale: 'en-US',
    searchAria: 'Search logs',
    searchSimple: 'Search…  error -timeout "disk full"   (press /)',
    searchLogsql: 'LogsQL: error AND host:web-01 | sort by (_time)',
    modeTitle: 'Switch between simple search and LogsQL',
    live: 'Live',
    liveTitle: 'Show new messages live',
    liveUnavailable: 'Live view is not available in LogsQL mode',
    settings: 'Settings',
    theme: 'Light / dark theme',
    close: 'Close',
    rangeAria: 'Time range', severityAria: 'Severity', hostAria: 'Host', appAria: 'Application',
    histoAria: 'Log volume over time',
    r5m: '5 min', r15m: '15 min', r1h: '1 hour', r6h: '6 hours', r24h: '24 hours', r7d: '7 days', r30d: '30 days', rAll: 'All',
    sevAll: 'All severities', sev2: 'Critical and above', sev3: 'Errors and above',
    sev4: 'Warnings and above', sev5: 'Notice and above', sev6: 'Info and above',
    allHosts: 'All hosts', allApps: 'All apps',
    emptyTitle: 'No matching logs.',
    emptyHint: 'Widen the time range or send syslog messages to port 514 (UDP or TCP).',
    count: (n) => `${n} ${n === 1 ? 'log' : 'logs'} shown`,
    histoMax: (n) => `max ${n} / interval`,
    searchError: 'Search error: ',
    connLive: 'live', connConnecting: 'connecting…', connRetry: 'reconnecting…',
    connPaused: 'live paused', connLogsql: 'live unavailable (LogsQL)',
    newLogs: (n) => `↑ ${n} new ${n === 1 ? 'log' : 'logs'}`,
    skipped: (n) => `${n} messages not shown (rate too high)`,
    stats: (s) => `received ${s.received} · stored ${s.ingested} · dropped ${s.dropped} · queue ${s.queue}`,
    storageErr: 'storage: ',
    unreachable: 'server unreachable',
    fTime: 'timestamp', fMsg: 'message', fPid: 'pid', fSd: 'structured data',
    filterHost: 'Filter on this host', filterApp: 'Filter on this app',
    copyMsg: 'Copy message', copyJson: 'Copy as JSON', copied: 'Copied',
    language: 'Language',
    tagsTitle: 'Color tags',
    tagsHelp: 'Keywords are highlighted in messages. Changes are saved automatically. When tags overlap, the one highest in the list wins.',
    addTag: '+ Add tag', resetTags: 'Restore default tags',
    optWhole: 'whole word', optWholeTitle: 'Only color whole words (avoids "ok" in "token")',
    optCaseTitle: 'Match case', optRegexTitle: 'Regular expression',
    optActive: 'active', optActiveTitle: 'Enable this tag',
    colorAria: 'Color', keyword: 'keyword', preview: 'preview', del: 'Delete', noTags: 'No tags.',
    newTag: 'new',
    confirmDelete: (p) => `Delete tag "${p}"?`,
    confirmReset: 'Replace all tags with the defaults (warning, error, ok)?',
    tagsLoadErr: 'Tags: ',
    err_pattern_required: 'The keyword is required',
    err_invalid_color: 'Invalid color (expected #rrggbb)',
    err_invalid_regex: 'Invalid regular expression',
    err_tag_not_found: 'Tag not found',
    err_live_logsql: 'Live view is not available in LogsQL mode',
  },
  fr: {
    locale: 'fr-FR',
    searchAria: 'Rechercher dans les logs',
    searchSimple: 'Rechercher…  error -timeout "disk full"   (touche /)',
    searchLogsql: 'LogsQL : error AND host:web-01 | sort by (_time)',
    modeTitle: 'Basculer entre recherche simple et LogsQL',
    live: 'Direct',
    liveTitle: 'Afficher les nouveaux messages en direct',
    liveUnavailable: 'Le direct n\'est pas disponible en mode LogsQL',
    settings: 'Paramètres',
    theme: 'Thème clair / sombre',
    close: 'Fermer',
    rangeAria: 'Période', severityAria: 'Sévérité', hostAria: 'Hôte', appAria: 'Application',
    histoAria: 'Volume de logs dans le temps',
    r5m: '5 min', r15m: '15 min', r1h: '1 heure', r6h: '6 heures', r24h: '24 heures', r7d: '7 jours', r30d: '30 jours', rAll: 'Tout',
    sevAll: 'Toutes sévérités', sev2: 'Critiques et +', sev3: 'Erreurs et +',
    sev4: 'Warnings et +', sev5: 'Notice et +', sev6: 'Info et +',
    allHosts: 'Tous les hôtes', allApps: 'Toutes les applis',
    emptyTitle: 'Aucun log ne correspond.',
    emptyHint: 'Élargissez la période ou envoyez des messages syslog sur le port 514 (UDP ou TCP).',
    count: (n) => `${n} ${n > 1 ? 'logs affichés' : 'log affiché'}`,
    histoMax: (n) => `max ${n} / intervalle`,
    searchError: 'Erreur de recherche : ',
    connLive: 'direct', connConnecting: 'connexion…', connRetry: 'reconnexion…',
    connPaused: 'direct en pause', connLogsql: 'direct indisponible (LogsQL)',
    newLogs: (n) => `↑ ${n} ${n > 1 ? 'nouveaux logs' : 'nouveau log'}`,
    skipped: (n) => `${n} messages non affichés (débit trop élevé)`,
    stats: (s) => `reçus ${s.received} · stockés ${s.ingested} · perdus ${s.dropped} · file ${s.queue}`,
    storageErr: 'stockage : ',
    unreachable: 'serveur injoignable',
    fTime: 'horodatage', fMsg: 'message', fPid: 'pid', fSd: 'données structurées',
    filterHost: 'Filtrer sur cet hôte', filterApp: 'Filtrer sur cette appli',
    copyMsg: 'Copier le message', copyJson: 'Copier en JSON', copied: 'Copié',
    language: 'Langue',
    tagsTitle: 'Tags de couleur',
    tagsHelp: 'Les mots-clés sont surlignés dans les messages. Les modifications sont enregistrées automatiquement. En cas de chevauchement, le tag le plus haut dans la liste l\'emporte.',
    addTag: '+ Ajouter un tag', resetTags: 'Rétablir les tags par défaut',
    optWhole: 'mot entier', optWholeTitle: 'Ne colorer que les mots entiers (évite « ok » dans « token »)',
    optCaseTitle: 'Respecter la casse', optRegexTitle: 'Expression régulière',
    optActive: 'actif', optActiveTitle: 'Activer ce tag',
    colorAria: 'Couleur', keyword: 'mot-clé', preview: 'aperçu', del: 'Supprimer', noTags: 'Aucun tag.',
    newTag: 'nouveau',
    confirmDelete: (p) => `Supprimer le tag « ${p} » ?`,
    confirmReset: 'Remplacer tous les tags par les tags par défaut (warning, error, ok) ?',
    tagsLoadErr: 'Tags : ',
    err_pattern_required: 'Le mot-clé est obligatoire',
    err_invalid_color: 'Couleur invalide (format #rrggbb attendu)',
    err_invalid_regex: 'Expression régulière invalide',
    err_tag_not_found: 'Tag introuvable',
    err_live_logsql: 'Le direct n\'est pas disponible en mode LogsQL',
  },
};

/* ================= Utilities ================= */

const $ = (sel) => document.querySelector(sel);
const PAGE_SIZE = 500;   // rows loaded per search
const MAX_ROWS = 2000;   // rows kept on screen in live mode
const RANGE_MS = { '5m': 3e5, '15m': 9e5, '1h': 36e5, '6h': 216e5, '24h': 864e5, '7d': 6048e5, '30d': 2592e6 };

const store = {
  get(k, def) { try { const v = localStorage.getItem('logstream.' + k); return v === null ? def : v; } catch { return def; } },
  set(k, v) { try { localStorage.setItem('logstream.' + k, v); } catch { /* storage unavailable */ } },
};

let lang = I18N[store.get('lang', 'en')] ? store.get('lang', 'en') : 'en';
function t(key, arg) {
  const v = I18N[lang][key] ?? I18N.en[key] ?? key;
  return typeof v === 'function' ? v(arg) : v;
}

const ESC = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ESC[c]);
const pad = (n, w = 2) => String(n).padStart(w, '0');
const debounce = (fn, ms) => { let tm; return (...a) => { clearTimeout(tm); tm = setTimeout(() => fn(...a), ms); }; };
const fmtNum = (n) => Number(n || 0).toLocaleString(t('locale'));

// Date.parse only accepts 3 decimals everywhere: drop the nanoseconds.
const parseTime = (s) => new Date(String(s || '').replace(/(\.\d{3})\d+/, '$1'));

async function api(url, opts = {}) {
  if (opts.body && typeof opts.body !== 'string') {
    opts.body = JSON.stringify(opts.body);
    opts.headers = { 'Content-Type': 'application/json' };
  }
  const res = await fetch(url, opts);
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { /* not JSON */ }
  if (!res.ok) {
    // Known error codes are translated; otherwise show the server message.
    let msg = (data && data.error) || text || res.statusText;
    if (data && data.code && I18N[lang]['err_' + data.code]) {
      msg = t('err_' + data.code) + (data.detail ? (lang === 'fr' ? ' : ' : ': ') + data.detail : '');
    }
    throw new Error(msg);
  }
  return data;
}

function toast(msg) {
  const el = $('#toast');
  // A modal dialog sits above everything: move the toast into it so it stays visible.
  const dlg = $('#settingsDlg');
  (dlg.open ? dlg : document.body).append(el);
  el.textContent = msg;
  el.hidden = false;
  clearTimeout(toast.tm);
  toast.tm = setTimeout(() => { el.hidden = true; }, 1800);
}

/* ================= State ================= */

const state = {
  rows: [],          // displayed records, newest first
  pending: [],       // live messages received while scrolled down
  tags: [],
  matchers: [],      // compiled tags + search terms
  mode: store.get('mode', 'simple'),
  live: store.get('live', '1') === '1',
  es: null,
  reqId: 0,
  histo: null,
  tookMs: null,
  conn: ['', 'connPaused'],
  stats: null,
};
const recOf = new WeakMap(); // .row element -> record

const list = $('#list');

/* ================= Language ================= */

function applyLang() {
  document.documentElement.lang = lang;
  for (const el of document.querySelectorAll('[data-i18n]')) el.textContent = t(el.dataset.i18n);
  for (const el of document.querySelectorAll('[data-i18n-title]')) el.title = t(el.dataset.i18nTitle);
  for (const el of document.querySelectorAll('[data-i18n-aria]')) el.setAttribute('aria-label', t(el.dataset.i18nAria));
  for (const b of document.querySelectorAll('#langSwitch [data-lang]')) b.setAttribute('aria-checked', String(b.dataset.lang === lang));
}

function setLang(next) {
  if (next === lang || !I18N[next]) return;
  lang = next;
  store.set('lang', lang);
  applyLang();
  syncControls();
  setConn(...state.conn);
  renderList();
  updateCount(state.tookMs);
  renderHisto(state.histo);
  renderStats();
  renderTagList();
}

$('#langSwitch').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-lang]');
  if (b) setLang(b.dataset.lang);
});

/* ================= Theme ================= */

function effectiveTheme() {
  const th = document.documentElement.dataset.theme;
  if (th) return th;
  return matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}
$('#themeBtn').addEventListener('click', () => {
  const next = effectiveTheme() === 'dark' ? 'light' : 'dark';
  document.documentElement.dataset.theme = next;
  store.set('theme', next);
});

/* ================= Filters ================= */

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
  $('#q').placeholder = logsql ? t('searchLogsql') : t('searchSimple');
  const btn = $('#live');
  btn.disabled = !liveAvailable();
  btn.classList.toggle('on', state.live && liveAvailable());
  btn.title = liveAvailable() ? t('liveTitle') : t('liveUnavailable');
  for (const id of ['severity', 'host', 'app']) $('#' + id).classList.toggle('set', $('#' + id).value !== '');
}

/* ================= Tags and highlighting ================= */

// Readable text color (black or white) for a given background.
function textOn(hex) {
  const n = parseInt(hex.slice(1), 16);
  const yiq = (((n >> 16) & 255) * 299 + ((n >> 8) & 255) * 587 + (n & 255) * 114) / 1000;
  return yiq >= 150 ? '#111827' : '#ffffff';
}
const tagStyle = (color) => `background:${color};color:${textOn(color)}`;
const escapeRe = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

function compileMatchers() {
  const out = [];
  for (const tag of state.tags) {
    if (!tag.enabled || !tag.pattern) continue;
    let src = tag.regex ? tag.pattern : escapeRe(tag.pattern);
    if (tag.wholeWord) src = `(?<![\\p{L}\\p{N}_])(?:${src})(?![\\p{L}\\p{N}_])`;
    try {
      out.push({ re: new RegExp(src, 'gu' + (tag.caseSensitive ? '' : 'i')), html: `<mark class="tag" style="${tagStyle(tag.color)}">` });
    } catch (e) {
      console.warn('Tag skipped (invalid pattern):', tag.pattern, e.message);
    }
  }
  // Simple-search terms get a subtle highlight, with the lowest priority.
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
    if (sp.s < pos) continue; // overlap: first one wins
    out += esc(text.slice(pos, sp.s)) + sp.html + esc(text.slice(sp.s, sp.e)) + '</mark>';
    pos = sp.e;
  }
  return out + esc(text.slice(pos));
}

/* ================= List rendering ================= */

const SEV_CLASS = { emerg: 'crit', alert: 'crit', crit: 'crit', err: 'err', warning: 'warning', notice: 'notice', info: 'info', debug: 'debug' };

function fmtTime(s) {
  const d = parseTime(s);
  if (isNaN(d)) return esc(s);
  const day = lang === 'fr' ? `${pad(d.getDate())}/${pad(d.getMonth() + 1)}` : `${pad(d.getMonth() + 1)}/${pad(d.getDate())}`;
  return `${day} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}<span class="ms">.${pad(d.getMilliseconds(), 3)}</span>`;
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

// Map the .row elements starting at `first` to their records.
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
  $('#empty').hidden = state.rows.length > 0 || !$('#error').hidden;
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
  if (!$('#error').hidden) { $('#count').textContent = ''; return; }
  const took = tookMs != null ? ` · ${tookMs} ms` : '';
  $('#count').textContent = t('count', state.rows.length).replace(/^\d+/, fmtNum(state.rows.length)) + took;
}

function detailsHTML(r) {
  const order = ['_time', 'host', 'app', 'procid', 'severity', 'facility', 'source', 'proto', 'sd', '_msg'];
  const hidden = new Set(['_stream_id', '_stream', 'sevnum']);
  const keys = order.filter((k) => r[k] != null && r[k] !== '')
    .concat(Object.keys(r).filter((k) => !order.includes(k) && !hidden.has(k)).sort());
  const label = { _time: t('fTime'), _msg: t('fMsg'), procid: t('fPid'), sd: t('fSd') };
  const val = (k) => (k === '_time'
    ? `${esc(parseTime(r[k]).toLocaleString(t('locale')))} <span class="muted">(${esc(r[k])})</span>`
    : esc(r[k]));
  return '<div class="details"><dl>'
    + keys.map((k) => `<dt>${esc(label[k] || k)}</dt><dd>${val(k)}</dd>`).join('')
    + '</dl><div class="acts">'
    + (r.host ? `<button class="btn" data-act="host">${esc(t('filterHost'))}</button>` : '')
    + (r.app ? `<button class="btn" data-act="app">${esc(t('filterApp'))}</button>` : '')
    + `<button class="btn" data-act="copy">${esc(t('copyMsg'))}</button>`
    + `<button class="btn" data-act="json">${esc(t('copyJson'))}</button>`
    + '</div></div>';
}

async function copy(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // Fallback for pages served over plain HTTP (not localhost).
    const ta = Object.assign(document.createElement('textarea'), { value: text });
    document.body.append(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
  }
  toast(t('copied'));
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
  if (String(window.getSelection())) return; // selecting text: don't collapse
  const open = row.classList.toggle('open');
  const det = row.querySelector('.details');
  if (open && !det) row.insertAdjacentHTML('beforeend', detailsHTML(r));
  else if (det) det.hidden = !open;
});
list.addEventListener('keydown', (ev) => {
  if (ev.key === 'Enter' && ev.target.classList.contains('row')) ev.target.click();
});

/* ================= Histogram ================= */

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
  const loc = t('locale');
  const label = (k) => {
    const d = new Date(k * stepMs);
    return stepMs >= 864e5 ? d.toLocaleDateString(loc) : d.toLocaleString(loc, { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
  };
  let bars = '';
  for (let k = start; k <= end; k++) {
    const n = counts.get(k) || 0;
    const hgt = n ? Math.max(4, (n / max) * 100) : 0;
    bars += `<i style="height:${hgt}%" title="${label(k)}: ${fmtNum(n)}"></i>`;
  }
  el.innerHTML = `<div class="bars">${bars}</div><div class="axis"><span>${label(start)}</span><span>${t('histoMax', fmtNum(max))}</span><span>${label(end)}</span></div>`;
}

async function refreshHisto() {
  try { state.histo = await api('/api/histogram?' + params()); } catch { state.histo = null; }
  renderHisto(state.histo);
}

/* ================= Search ================= */

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
    setError(null);
    state.rows = res.rows || [];
    state.pending = [];
    state.tookMs = res.took_ms;
    $('#newPill').hidden = true;
    renderList();
    updateCount(state.tookMs);
  } catch (e) {
    if (id !== state.reqId) return;
    setError(t('searchError') + e.message);
    state.rows = [];
    renderList();
    updateCount();
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
  if (ev.key === '/' && !/INPUT|TEXTAREA|SELECT/.test(document.activeElement.tagName) && !$('#settingsDlg').open) {
    ev.preventDefault();
    $('#q').focus();
  }
});

/* ================= Live view (SSE) ================= */

function setConn(cls, key) {
  state.conn = [cls, key];
  const el = $('#conn');
  el.className = 'conn ' + cls;
  el.textContent = t(key);
}

function connectLive() {
  if (state.es) { state.es.close(); state.es = null; }
  if (!state.live || !liveAvailable()) {
    setConn('', state.live ? 'connLogsql' : 'connPaused');
    return;
  }
  const es = new EventSource('/api/stream?' + params());
  state.es = es;
  setConn('', 'connConnecting');
  es.onopen = () => setConn('ok', 'connLive');
  es.onerror = () => setConn('ko', 'connRetry');
  es.onmessage = (ev) => {
    const data = JSON.parse(ev.data);
    const rows = (data.rows || []).reverse(); // newest first
    if (data.skipped) toast(t('skipped', fmtNum(data.skipped)));
    if (window.scrollY > 80) {
      state.pending = rows.concat(state.pending).slice(0, MAX_ROWS);
      const pill = $('#newPill');
      pill.textContent = t('newLogs', state.pending.length).replace(/\d+/, fmtNum(state.pending.length));
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
  if (state.live) refresh(); // catch up on what arrived while paused
  else { syncControls(); connectLive(); }
});

/* ================= Host / app lists, stats ================= */

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
  } catch { /* VictoriaLogs not ready yet: retried later */ }
}

function renderStats() {
  const el = $('#stats');
  const s = state.stats;
  if (!s) { el.textContent = t('unreachable'); return; }
  const n = { received: fmtNum(s.received), ingested: fmtNum(s.ingested), dropped: '%DROPPED%', queue: fmtNum(s.queue) };
  const dropped = s.dropped ? `<span class="bad">${fmtNum(s.dropped)}</span>` : fmtNum(0);
  let html = esc(t('stats', n)).replace('%DROPPED%', dropped);
  if (s.lastError) html += ` · <span class="bad">${esc(t('storageErr') + s.lastError)}</span>`;
  el.innerHTML = html;
}

async function loadStats() {
  try { state.stats = await api('/api/stats'); } catch { state.stats = null; }
  renderStats();
}

/* ================= Tag management ================= */

const PALETTE = ['#6366f1', '#0ea5e9', '#14b8a6', '#a855f7', '#ec4899', '#eab308', '#64748b', '#f97316'];

async function loadTags() {
  try { state.tags = await api('/api/tags'); } catch (e) { toast(t('tagsLoadErr') + e.message); }
  compileMatchers();
}

function tagRowHTML(tag) {
  const opt = (field, label, title) =>
    `<label class="opt" title="${esc(title)}"><input type="checkbox" data-f="${field}"${tag[field] ? ' checked' : ''}>${esc(label)}</label>`;
  return `<div class="tag-row${tag.enabled ? '' : ' off'}" data-id="${esc(tag.id)}">
    <input type="color" value="${esc(tag.color)}" data-f="color" aria-label="${esc(t('colorAria'))}">
    <input type="text" value="${esc(tag.pattern)}" data-f="pattern" placeholder="${esc(t('keyword'))}" spellcheck="false" aria-label="${esc(t('keyword'))}">
    <span class="preview"><mark class="tag" style="${tagStyle(tag.color)}">${esc(tag.pattern || t('preview'))}</mark></span>
    <div class="opts">
      ${opt('wholeWord', t('optWhole'), t('optWholeTitle'))}
      ${opt('caseSensitive', 'Aa', t('optCaseTitle'))}
      ${opt('regex', '.*', t('optRegexTitle'))}
      ${opt('enabled', t('optActive'), t('optActiveTitle'))}
    </div>
    <button class="icon-btn danger" type="button" data-del title="${esc(t('del'))}" aria-label="${esc(t('del'))}">
      <svg viewBox="0 0 24 24"><path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/></svg>
    </button>
    <div class="err"></div>
  </div>`;
}

function renderTagList() {
  $('#tagList').innerHTML = state.tags.map(tagRowHTML).join('') || `<p class="muted small">${esc(t('noTags'))}</p>`;
}

const saveTimers = {};
function scheduleSave(tag, rowEl) {
  clearTimeout(saveTimers[tag.id]);
  saveTimers[tag.id] = setTimeout(async () => {
    const errEl = rowEl.querySelector('.err');
    try {
      await api('/api/tags/' + encodeURIComponent(tag.id), { method: 'PUT', body: tag });
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
  const tag = state.tags.find((x) => x.id === rowEl.dataset.id);
  if (!tag) return;
  tag[field] = ev.target.type === 'checkbox' ? ev.target.checked : ev.target.value;
  rowEl.classList.toggle('off', !tag.enabled);
  const mark = rowEl.querySelector('.preview mark');
  mark.textContent = tag.pattern || t('preview');
  mark.setAttribute('style', tagStyle(tag.color));
  scheduleSave(tag, rowEl);
  applyTags();
});

$('#tagList').addEventListener('click', async (ev) => {
  const del = ev.target.closest('[data-del]');
  if (!del) return;
  const rowEl = del.closest('.tag-row');
  const tag = state.tags.find((x) => x.id === rowEl.dataset.id);
  if (!tag || !confirm(t('confirmDelete', tag.pattern))) return;
  try {
    await api('/api/tags/' + encodeURIComponent(tag.id), { method: 'DELETE' });
    state.tags = state.tags.filter((x) => x !== tag);
    renderTagList();
    applyTags();
  } catch (e) { toast(e.message); }
});

$('#addTag').addEventListener('click', async () => {
  const color = PALETTE[state.tags.length % PALETTE.length];
  try {
    const tag = await api('/api/tags', { method: 'POST', body: { pattern: t('newTag'), color, wholeWord: true, enabled: true } });
    state.tags.push(tag);
    renderTagList();
    applyTags();
    const input = $('#tagList').lastElementChild.querySelector('input[type="text"]');
    input.focus();
    input.select();
  } catch (e) { toast(e.message); }
});

$('#resetTags').addEventListener('click', async () => {
  if (!confirm(t('confirmReset'))) return;
  try {
    state.tags = await api('/api/tags/reset', { method: 'POST' });
    renderTagList();
    applyTags();
  } catch (e) { toast(e.message); }
});

$('#settingsBtn').addEventListener('click', () => { renderTagList(); $('#settingsDlg').showModal(); });
$('#closeSettings').addEventListener('click', () => $('#settingsDlg').close());
$('#settingsDlg').addEventListener('click', (ev) => { if (ev.target === ev.currentTarget) ev.currentTarget.close(); });

/* ================= Startup ================= */

applyLang();
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
