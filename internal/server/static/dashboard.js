let currentTab = 'stats';
let logPaused = false;

function toggleLogPause() {
  logPaused = !logPaused;
  const btn = document.getElementById('log-pause-btn');
  const hint = document.getElementById('log-pause-hint');
  if (logPaused) {
    btn.textContent = 'RESUME';
    btn.classList.remove('active');
    btn.style.background = '#3d2f00';
    btn.style.color = '#e3b341';
    hint.textContent = 'pausado';
  } else {
    btn.textContent = 'PAUSE';
    btn.classList.add('active');
    btn.style.background = '';
    btn.style.color = '';
    hint.textContent = 'atualiza a cada 1s';
    fetchLog();
  }
}

function switchTab(name) {
  currentTab = name;
  document.getElementById('stats-panel').style.display = name === 'stats' ? '' : 'none';
  document.getElementById('search-bar').style.display = name === 'stats' ? 'flex' : 'none';
  document.getElementById('log-panel').style.display = name === 'log' ? 'block' : 'none';
  document.getElementById('tab-stats').classList.toggle('active', name === 'stats');
  document.getElementById('tab-log').classList.toggle('active', name === 'log');
  const tabFiles = document.getElementById('tab-files');
  if (tabFiles) tabFiles.classList.toggle('active', name === 'files');
  document.getElementById('files-panel') && (document.getElementById('files-panel').style.display = name === 'files' ? '' : 'none');
  document.getElementById('api-panel').style.display = name === 'api' ? '' : 'none';
  const tabApi = document.getElementById('tab-api');
  if (tabApi) tabApi.classList.toggle('active', name === 'api');
  if (name === 'log') fetchLog();
  if (name === 'api') fetchAPIs(1);
  if (name === 'files') {
    // initial fetch and start short polling while visible
    const lim = Number(document.getElementById('files-limit').value) || 50;
    const filter = document.getElementById('files-filter').value || '';
    fetchFiles(lim, filter);
  }
}

function fetchLog() {
  fetch('/api/log').then(r => r.json()).then(entries => {
    if (!entries) return;
    const tbody = document.getElementById('log-tbody');
    tbody.innerHTML = entries.slice(-500).reverse().map(e => {
      let ts = '';
      if (e.Time) {
        const d = new Date(e.Time);
        const pad2 = n => String(n).padStart(2, '0');
        const ms = String(d.getMilliseconds()).padStart(3, '0');
        ts = pad2(d.getHours()) + ':' + pad2(d.getMinutes()) + ':' + pad2(d.getSeconds()) + '.' + ms;
      }
      const args = e.Args ? (e.Args.length > 120 ? e.Args.slice(0, 117) + '\u2026' : e.Args) : '';
      return '<tr' + (e.Error ? ' class="error"' : '') + '>' +
        '<td class="l-ts">' + esc(ts) + '</td>' +
        '<td class="l-name">' + esc(e.Name) + '</td>' +
        '<td class="l-args">' + esc(args) + (e.Error ? ' \u2192 <b>' + esc(e.Error) + '</b>' : '') + '</td>' +
        '</tr>';
    }).join('');
  }).catch(() => { });
}

setInterval(() => {
  if (currentTab === 'log' && !logPaused) fetchLog();
}, 1000);

setInterval(() => {
  if (currentTab === 'files') {
    const lim = Number(document.getElementById('files-limit')?.value || 50);
    const filter = document.getElementById('files-filter')?.value || '';
    fetchFiles(lim, filter);
  }
}, 2000);

function fetchStatus() {
  fetch('/api/status').then(r => r.json()).then(s => {
    const p = s.Proc;
    const el = document.getElementById('proc-info');
    if (p && p.Comm) {
      let label = p.Comm + '[' + p.PID + ']';
      if (p.Cwd) label += '  \u2022  ' + p.Cwd;
      el.textContent = label;
      el.title = p.Cmdline || '';
    }
  }).catch(() => { });
}

let apiPage = 1;
let apiPerPage = 0; // 0 means 'all'
let apiTotal = 0;

function fetchAPIs(page) {
  page = page || 1;
  const perSel = document.getElementById('api-per-page');
  const per = perSel ? parseInt(perSel.value, 10) || 0 : 0;
  const url = (per === 0) ? '/api' : ('/api?page=' + page + '&per_page=' + per);
  fetch(url).then(r => r.json()).then(d => {
    if (!d) return;
    apiPage = d.page || page;
    apiPerPage = d.per_page || per;
    apiTotal = d.total || 0;
    renderAPIs(d.items || []);
    const pageInfo = document.getElementById('api-page-info');
    const prev = document.getElementById('api-prev');
    const next = document.getElementById('api-next');
    if (per === 0) {
      if (pageInfo) pageInfo.textContent = 'All (' + apiTotal + ' total)';
      if (prev) prev.disabled = true;
      if (next) next.disabled = true;
    } else {
      const pages = Math.max(1, Math.ceil(apiTotal / apiPerPage));
      if (pageInfo) pageInfo.textContent = 'Page ' + apiPage + ' of ' + pages + ' (' + apiTotal + ' total)';
      if (prev) prev.disabled = apiPage <= 1;
      if (next) next.disabled = apiPage >= pages;
    }
  }).catch(() => { });
}

function renderAPIs(items) {
  const container = document.getElementById('api-list');
  if (!container) return;
  container.innerHTML = items.map(i => {
    const method = escapeHtml(i.method || 'GET');
    const path = escapeHtml(i.path || '/');
    const desc = i.description ? '<div class="api-desc" style="color:#8b949e;margin-top:4px">' + escapeHtml(i.description) + '</div>' : '';
    return '<div class="api-item" style="padding:6px 8px;border-bottom:1px solid #161b22">'
      + '<a href="' + path + '" target="_blank" rel="noopener noreferrer" style="text-decoration:none;display:block;color:inherit">'
      + '<span style="color:#79c0ff;font-weight:700;margin-right:8px">' + method + '</span>'
      + '<span style="color:#e6edf3">' + path + '</span>'
      + desc
      + '</a>'
      + '</div>';
  }).join('');
}

document.addEventListener('DOMContentLoaded', () => {
  const prev = document.getElementById('api-prev');
  const next = document.getElementById('api-next');
  const per = document.getElementById('api-per-page');
  if (prev) prev.addEventListener('click', () => { if (apiPage > 1) fetchAPIs(apiPage - 1); });
  if (next) next.addEventListener('click', () => { const pages = Math.max(1, Math.ceil(apiTotal / apiPerPage)); if (apiPage < pages) fetchAPIs(apiPage + 1); });
  if (per) per.addEventListener('change', () => { fetchAPIs(1); });
  const tabFilesBtn = document.getElementById('tab-files');
  if (tabFilesBtn) tabFilesBtn.addEventListener('click', () => { switchTab('files'); });
  const filesRefresh = document.getElementById('files-refresh');
  if (filesRefresh) filesRefresh.addEventListener('click', () => {
    const lim = Number(document.getElementById('files-limit')?.value || 50);
    const filter = document.getElementById('files-filter')?.value || '';
    fetchFiles(lim, filter);
  });
  const filesFilter = document.getElementById('files-filter');
  if (filesFilter) filesFilter.addEventListener('input', () => {
    const lim = Number(document.getElementById('files-limit')?.value || 50);
    const filter = filesFilter.value || '';
    fetchFiles(lim, filter);
  });
});

async function fetchFiles(limit = 50, filter = '') {
  try {
    const res = await fetch('/api/files?limit=' + encodeURIComponent(limit));
    if (!res.ok) return;
    const files = await res.json();
    const tbody = document.querySelector('#files-table tbody');
    if (!tbody) return;
    tbody.innerHTML = '';
    for (const f of files) {
      const raw = f.path || f.Path || '';
      const count = (f.count != null) ? f.count : (f.Count != null ? f.Count : 0);
      const path = sanitizePath(raw);
      if (filter && !path.includes(filter)) continue;
      const tr = document.createElement('tr');
      const tdPath = document.createElement('td');
      tdPath.textContent = path.length > 200 ? path.slice(0, 197) + '\u2026' : path;
      tdPath.title = path;
      tdPath.className = 'name';
      tdPath.style.cursor = 'pointer';
      tdPath.addEventListener('click', () => { try { navigator.clipboard && navigator.clipboard.writeText(raw); } catch (e) {} });
      const tdCount = document.createElement('td');
      tdCount.className = 'num';
      tdCount.style.textAlign = 'right';
      tdCount.textContent = fmtN(count);
      tr.appendChild(tdPath);
      tr.appendChild(tdCount);
      tbody.appendChild(tr);
    }
  } catch (err) {
    console.error('failed to fetch files', err);
  }
}

// sanitizePath replaces control characters with visible \xNN escapes so that
// binary payloads do not break the dashboard layout.
function sanitizePath(s) {
  if (!s) return '';
  return String(s).replace(/[\x00-\x1F\x7F-\x9F]/g, function(ch) {
    return '\\x' + ch.charCodeAt(0).toString(16).padStart(2, '0');
  });
}
function alertExplanation(name) {
  const m = {
    ioctl: 'terminal control failed \u2014 process likely has no TTY (running under sudo or piped)',
    openat: 'files not found \u2014 often normal (dynamic linker searches multiple paths)',
    open: 'files not found \u2014 often normal (dynamic linker searches multiple paths)',
    access: 'optional files are missing \u2014 usually harmless (checking for config files)',
    faccessat: 'optional files are missing \u2014 usually harmless (checking for config files)',
    connect: 'connection attempts failed \u2014 may be Happy Eyeballs (IPv4/IPv6 race) or no route',
    recvfrom: 'EAGAIN on non-blocking socket \u2014 normal for async I/O, not a real error',
    recv: 'EAGAIN on non-blocking socket \u2014 normal for async I/O, not a real error',
    recvmsg: 'EAGAIN on non-blocking socket \u2014 normal for async I/O, not a real error',
    sendto: 'send failed \u2014 peer may have closed the connection',
    send: 'send failed \u2014 peer may have closed the connection',
    sendmsg: 'send failed \u2014 peer may have closed the connection',
    madvise: 'memory hint rejected by kernel \u2014 informational, not a real failure',
    prctl: 'process control rejected \u2014 may lack capabilities (seccomp or container policy)',
    statfs: 'filesystem stat failed \u2014 path may be on a special fs (proc, tmpfs)',
    fstatfs: 'filesystem stat failed \u2014 path may be on a special fs (proc, tmpfs)',
    unlink: 'tried to delete a non-existent file \u2014 may be cleanup of temp files',
    unlinkat: 'tried to delete a non-existent file \u2014 may be cleanup of temp files',
    mkdir: 'directory already exists \u2014 common during first-run initialisation',
    mkdirat: 'directory already exists \u2014 common during first-run initialisation'
  };
  return m[name] || '';
}

function updateAlerts(rows) {
  const alerts = [];
  rows.forEach(r => {
    const errPct = r.Count ? (r.Errors / r.Count * 100) : 0;
    const avgNs = r.Count ? Math.round(r.TotalTime / r.Count) : 0;
    if (errPct >= 50) {
      const expl = alertExplanation(r.Name);
      let msg = '\u26a0  ' + r.Name + ': ' + errPct.toFixed(0) + '% error rate (' + r.Errors + '/' + r.Count + ' calls)';
      if (expl) msg += ' \u2014 ' + expl;
      alerts.push({ type: 'hot', msg });
    } else if (avgNs >= 5e6) {
      alerts.push({ type: 'slow', msg: '\u26a1  ' + r.Name + ': slow avg ' + fmtDur(avgNs) + ' (max ' + fmtDur(r.MaxTime) + ') \u2014 kernel spending time in this call' });
    }
  });
  const section = document.getElementById('alerts-section');
  if (alerts.length === 0) {
    section.style.display = 'none';
    return;
  }
  section.style.display = 'block';
  document.getElementById('alerts-hdr').textContent = '\u26a0  ANOMALY ALERTS (' + alerts.length + ')';
  document.getElementById('alerts-list').innerHTML = alerts.map(a =>
    '<div class="alert-row ' + a.type + '">' + escapeHtml(a.msg) + '</div>'
  ).join('');
}

const fmtDur = ns => {
  if (!ns) return '\u2014';
  if (ns < 1e3) return ns + 'ns';
  if (ns < 1e6) return (ns / 1e3).toFixed(1) + '\u00b5s';
  if (ns < 1e9) return (ns / 1e6).toFixed(1) + 'ms';
  return (ns / 1e9).toFixed(2) + 's';
};

const fmtN = n => {
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'k';
  return '' + n;
};

const catClass = c => 'cat-' + c.replace('/', '');
const CAT_ORDER = ['I/O', 'FS', 'NET', 'MEM', 'PROC', 'SIG', 'OTHER'];

let sortCol = 'Count';
let sortDir = -1;
let lastData = [];
let filterQuery = '';

document.getElementById('search-input').addEventListener('input', e => {
  filterQuery = e.target.value.toLowerCase().trim();
  document.getElementById('search-clear').style.display = filterQuery ? '' : 'none';
  document.getElementById('search-count').textContent = '';
  render(lastData);
});

document.getElementById('search-clear').addEventListener('click', () => {
  filterQuery = '';
  document.getElementById('search-input').value = '';
  document.getElementById('search-clear').style.display = 'none';
  document.getElementById('search-count').textContent = '';
  render(lastData);
});

function sortData(rows) {
  return [...rows].sort((a, b) => {
    let av = a[sortCol];
    let bv = b[sortCol];
    if (sortCol === '_erp') {
      av = a.Count ? a.Errors / a.Count : 0;
      bv = b.Count ? b.Errors / b.Count : 0;
    }
    if (sortCol === 'AvgTime') {
      av = a.Count ? a.TotalTime / a.Count : 0;
      bv = b.Count ? b.TotalTime / b.Count : 0;
    }
    if (sortCol === '_bar') {
      av = a.Count;
      bv = b.Count;
    }
    if (sortCol === 'Category') {
      av = CAT_ORDER.indexOf(a.Category);
      bv = CAT_ORDER.indexOf(b.Category);
    }
    if (typeof av === 'string') return sortDir * av.localeCompare(bv);
    return sortDir * ((av || 0) - (bv || 0));
  });
}

function escapeHtml(str) {
  if (str == null) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
    .replace(/\//g, '&#x2F;');
}

const esc = escapeHtml;

function render(rows) {
  const filtered = filterQuery ? rows.filter(r => r.Name.toLowerCase().includes(filterQuery)) : rows;
  const countEl = document.getElementById('search-count');
  if (filterQuery) {
    countEl.textContent = filtered.length + ' / ' + rows.length;
  } else {
    countEl.textContent = '';
  }

  const maxCount = filtered.reduce((m, r) => Math.max(m, r.Count), 0);
  const tbody = document.getElementById('tbody');
  const sorted = sortData(filtered);

  // Clear existing rows
  tbody.textContent = '';

  sorted.forEach(r => {
    const errPct = r.Count ? (r.Errors / r.Count * 100) : 0;
    const avgNs = r.Count ? Math.round(r.TotalTime / r.Count) : 0;
    const minNs = r.MinTime || 0;
    const pct = maxCount ? Math.round(r.Count / maxCount * 100) : 0;
    const slow = avgNs >= 5e6;

    const tr = document.createElement('tr');
    // data-name attribute (use raw value; attribute assignment is not parsed as HTML)
    tr.setAttribute('data-name', r.Name != null ? String(r.Name) : '');

    // Name cell
    const tdName = document.createElement('td');
    tdName.className = 'name';
    tdName.textContent = r.Name != null ? String(r.Name) : '';
    tr.appendChild(tdName);

    // File cell (top observed file for this syscall, if any)
    const tdFile = document.createElement('td');
    tdFile.className = 'name file';
    const topFile = (r.Files && r.Files.length) ? (r.Files[0].path || r.Files[0].Path || '') : '';
    tdFile.textContent = topFile;
    tdFile.title = topFile;
    tr.appendChild(tdFile);

    // Category cell with pill
    const tdCat = document.createElement('td');
    const spanCat = document.createElement('span');
    spanCat.className = 'cat-pill ' + catClass(r.Category);
    spanCat.textContent = r.Category != null ? String(r.Category) : '';
    tdCat.appendChild(spanCat);
    tr.appendChild(tdCat);

    // Count cell
    const tdCount = document.createElement('td');
    tdCount.className = 'num';
    tdCount.textContent = fmtN(r.Count);
    tr.appendChild(tdCount);

    // Spark bar cell
    const tdSpark = document.createElement('td');
    const divSpark = document.createElement('div');
    divSpark.className = 'spark';
    const divSparkFill = document.createElement('div');
    divSparkFill.className = 'spark-fill';
    divSparkFill.style.width = pct + '%';
    divSpark.appendChild(divSparkFill);
    tdSpark.appendChild(divSpark);
    tr.appendChild(tdSpark);

    // Avg time cell
    const tdAvg = document.createElement('td');
    tdAvg.className = 'num' + (slow ? ' slow' : '');
    tdAvg.textContent = fmtDur(avgNs);
    tr.appendChild(tdAvg);

    // Min time cell
    const tdMin = document.createElement('td');
    tdMin.className = 'num';
    tdMin.textContent = fmtDur(minNs);
    tr.appendChild(tdMin);

    // Max time cell
    const tdMax = document.createElement('td');
    tdMax.className = 'num';
    tdMax.textContent = fmtDur(r.MaxTime);
    tr.appendChild(tdMax);

    // Total time cell
    const tdTotal = document.createElement('td');
    tdTotal.className = 'num';
    tdTotal.textContent = fmtDur(r.TotalTime);
    tr.appendChild(tdTotal);

    // Errors count cell
    const tdErrCount = document.createElement('td');
    tdErrCount.className = 'num err';
    if (r.Errors) {
      tdErrCount.textContent = r.Errors != null ? String(r.Errors) : '';
    } else {
      tdErrCount.textContent = '\u2014';
    }
    tr.appendChild(tdErrCount);

    // Error percentage cell
    const tdErrPct = document.createElement('td');
    tdErrPct.className = 'num err';
    if (r.Errors) {
      tdErrPct.textContent = errPct.toFixed(0) + '%';
    } else {
      tdErrPct.textContent = '\u2014';
    }
    tr.appendChild(tdErrPct);

    tbody.appendChild(tr);
  });
}

function updateMeta(rows) {
  const total = rows.reduce((s, r) => s + r.Count, 0);
  const errors = rows.reduce((s, r) => s + r.Errors, 0);
  document.getElementById('m-total').textContent = fmtN(total);
  document.getElementById('m-errors').textContent = fmtN(errors);
  document.getElementById('m-unique').textContent = rows.length;
}

function updateCatBar(rows) {
  const total = rows.reduce((s, r) => s + r.Count, 0);
  const cats = {};
  rows.forEach(r => {
    cats[r.Category] = (cats[r.Category] || 0) + r.Count;
  });
  document.getElementById('cat-bar').innerHTML = CAT_ORDER
    .filter(c => cats[c])
    .map(c => {
      const pct = total ? (cats[c] / total * 100).toFixed(0) : 0;
      return '<span class="cat-pill ' + catClass(c) + '">' + c + ' ' + pct + '%</span>';
    }).join('');
}

document.querySelector('thead').addEventListener('click', e => {
  const th = e.target.closest('th');
  if (!th || !th.dataset.col) return;
  if (sortCol === th.dataset.col) {
    sortDir *= -1;
  } else {
    sortCol = th.dataset.col;
    sortDir = -1;
  }
  document.querySelectorAll('thead th').forEach(t => t.classList.remove('asc', 'desc'));
  th.classList.add(sortDir === -1 ? 'desc' : 'asc');
  render(lastData);
});

document.getElementById('tbody').addEventListener('click', e => {
  const tr = e.target.closest('tr');
  if (tr && tr.dataset.name) location.href = '/syscall/' + encodeURIComponent(tr.dataset.name);
});

let prevTotal = 0;
let prevTs = Date.now();
let processExited = false;

function showDoneBanner() {
  processExited = true;
  const b = document.getElementById('done-banner');
  const ts = new Date().toLocaleTimeString();
  b.textContent = '\u23F9 Process exited at ' + ts + ' \u2014 trace complete. Data frozen.';
  b.style.display = 'block';
  document.getElementById('status').textContent = 'Process exited \u2014 data frozen';
  document.getElementById('status').classList.remove('err');
}

function connect() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(proto + '://' + location.host + '/stream');

  ws.onopen = () => {
    document.getElementById('status').textContent = 'Connected \u2014 live updates every second';
    document.getElementById('status').classList.remove('err');
    fetchStatus();
  };

  ws.onmessage = e => {
    const msg = JSON.parse(e.data);
    if (msg && !Array.isArray(msg) && msg.done === true) {
      showDoneBanner();
      ws.close();
      return;
    }

    const rows = Array.isArray(msg) ? msg : [];
    lastData = rows;
    const now = Date.now();
    const total = rows.reduce((s, r) => s + r.Count, 0);
    const rate = prevTs !== now ? Math.round((total - prevTotal) / ((now - prevTs) / 1000)) : 0;
    prevTotal = total;
    prevTs = now;
    document.getElementById('m-rate').textContent = fmtN(Math.max(0, rate));
    updateMeta(rows);
    updateCatBar(rows);
    updateAlerts(rows);
    render(rows);
  };

  ws.onerror = () => {
    document.getElementById('status').textContent = 'WebSocket error \u2014 retrying\u2026';
    document.getElementById('status').classList.add('err');
  };

  ws.onclose = () => {
    if (processExited) return;
    document.getElementById('status').textContent = 'Disconnected \u2014 reconnecting in 2 s\u2026';
    document.getElementById('status').classList.add('err');
    setTimeout(connect, 2000);
  };
}

connect();
