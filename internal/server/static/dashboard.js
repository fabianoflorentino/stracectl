let currentTab = 'stats';

function switchTab(name) {
  currentTab = name;
  document.getElementById('stats-panel').style.display = name === 'stats' ? '' : 'none';
  document.getElementById('search-bar').style.display = name === 'stats' ? 'flex' : 'none';
  document.getElementById('log-panel').style.display = name === 'log' ? 'block' : 'none';
  document.getElementById('tab-stats').classList.toggle('active', name === 'stats');
  document.getElementById('tab-log').classList.toggle('active', name === 'log');
  if (name === 'log') fetchLog();
}

function fetchLog() {
  fetch('/api/log').then(r => r.json()).then(entries => {
    if (!entries) return;
    const tbody = document.getElementById('log-tbody');
    tbody.innerHTML = entries.slice(-500).map(e => {
      const ts = e.Time ? new Date(e.Time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 }) : '';
      const args = e.Args ? (e.Args.length > 120 ? e.Args.slice(0, 117) + '\u2026' : e.Args) : '';
      return '<tr' + (e.Error ? ' class="error"' : '') + '>' +
        '<td class="l-ts">' + esc(ts) + '</td>' +
        '<td class="l-name">' + esc(e.Name) + '</td>' +
        '<td class="l-args">' + esc(args) + (e.Error ? ' \u2192 <b>' + esc(e.Error) + '</b>' : '') + '</td>' +
        '</tr>';
    }).join('');
    const wrap = document.getElementById('log-wrap');
    wrap.scrollTop = wrap.scrollHeight;
  }).catch(() => {});
}

setInterval(() => {
  if (currentTab === 'log') fetchLog();
}, 1000);

function fetchStatus() {
  fetch('/api/status').then(r => r.json()).then(s => {
    const p = s.Proc;
    const el = document.getElementById('proc-info');
    if (p && p.Comm) {
      let label = p.Comm + '[' + p.PID + ']';
      if (p.Cwd) label += '  \u2022  ' + escapeHtml(p.Cwd);
      el.textContent = label;
      el.title = p.Cmdline || '';
    }
  }).catch(() => {});
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

  tbody.innerHTML = sorted.map(r => {
    const errPct = r.Count ? (r.Errors / r.Count * 100) : 0;
    const avgNs = r.Count ? Math.round(r.TotalTime / r.Count) : 0;
    const pct = maxCount ? Math.round(r.Count / maxCount * 100) : 0;
    const slow = avgNs >= 5e6;
    const safeName = escapeHtml(r.Name);
    const safeCategory = escapeHtml(r.Category);
    const safeErrors = r.Errors != null ? escapeHtml(String(r.Errors)) : '';
    return '<tr data-name="' + safeName + '">' +
      '<td class="name">' + safeName + '</td>' +
      '<td><span class="cat-pill ' + catClass(r.Category) + '">' + safeCategory + '</span></td>' +
      '<td class="num">' + fmtN(r.Count) + '</td>' +
      '<td><div class="spark"><div class="spark-fill" style="width:' + pct + '%"></div></div></td>' +
      '<td class="num' + (slow ? ' slow' : '') + '">' + fmtDur(avgNs) + '</td>' +
      '<td class="num">' + fmtDur(r.MaxTime) + '</td>' +
      '<td class="num">' + fmtDur(r.TotalTime) + '</td>' +
      '<td class="num err">' + (r.Errors ? safeErrors : '\u2014') + '</td>' +
      '<td class="num err">' + (r.Errors ? errPct.toFixed(0) + '%' : '\u2014') + '</td>' +
      '</tr>';
  }).join('');
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
