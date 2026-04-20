---
name: frontend-design
description: Create distinctive, production-grade frontend interfaces with high design quality. Use this skill when the user asks to build web components, pages, artifacts, posters, or applications (examples include websites, landing pages, dashboards, React components, HTML/CSS layouts, or when styling/beautifying any web UI). Generates creative, polished code and UI design that avoids generic AI aesthetics.
license: Complete terms in LICENSE.txt
---

# Frontend Design — stracectl Context

This project has **two distinct frontend contexts**. Understand which one you are working in before writing any code.

---

## Context A: Embedded Dashboard (`internal/server/static/`)

**Stack:** Vanilla JavaScript (ES2017+), no framework, no build step, no npm. Three files embedded into the Go binary via `//go:embed`.

- `dashboard.html` — main SPA shell with inline `<style>`
- `dashboard.js` — all logic
- `syscall_detail.html` — per-syscall detail page

**Constraints:**
- No TypeScript, no Webpack/Vite, no external CDN dependencies
- Files must remain self-contained — everything inline or in the same file
- Dark GitHub-inspired palette (`#0d1117` background, `#79c0ff` accents)
- All DOM updates via `innerHTML` with `escapeHtml()` / `esc()` for XSS safety, or via `document.createElement` for complex nodes
- Data flows from a WebSocket connection to `/stream` (JSON stat snapshots every ~1s)
- Secondary data via `fetch()` with polling

### Dashboard JS Patterns

**WebSocket data flow:**
```javascript
const ws = new WebSocket(`ws://${location.host}/stream`);
ws.onmessage = (e) => {
    const stats = JSON.parse(e.data);
    renderTable(stats);
    renderCategories(stats);
    detectAnomalies(stats);
};
ws.onclose = () => setTimeout(connect, 2000); // reconnect
```

**XSS-safe innerHTML rendering:**
```javascript
function esc(s) {
    return String(s)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

function renderRow(stat) {
    return `<tr>
        <td>${esc(stat.name)}</td>
        <td>${fmtN(stat.count)}</td>
        <td>${fmtDur(stat.avg_ns)}</td>
    </tr>`;
}
```

**Duration and count formatting:**
```javascript
function fmtDur(ns) {
    if (ns < 1_000) return `${ns}ns`;
    if (ns < 1_000_000) return `${(ns / 1_000).toFixed(1)}µs`;
    if (ns < 1_000_000_000) return `${(ns / 1_000_000).toFixed(1)}ms`;
    return `${(ns / 1_000_000_000).toFixed(2)}s`;
}

function fmtN(n) {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
    return String(n);
}
```

**Client-side sort:**
```javascript
let sortField = 'count';
let sortDir = -1; // -1 = desc

function sortData(data) {
    return [...data].sort((a, b) => {
        const av = a[sortField] ?? 0;
        const bv = b[sortField] ?? 0;
        return sortDir * (bv - av);
    });
}

document.querySelectorAll('th[data-sort]').forEach(th => {
    th.addEventListener('click', () => {
        const field = th.dataset.sort;
        if (sortField === field) {
            sortDir *= -1;
        } else {
            sortField = field;
            sortDir = -1;
        }
        renderTable(lastStats);
    });
});
```

**Tab switching:**
```javascript
function showTab(name) {
    document.querySelectorAll('.tab-panel').forEach(p => {
        p.hidden = p.dataset.tab !== name;
    });
    document.querySelectorAll('.tab-btn').forEach(b => {
        b.classList.toggle('active', b.dataset.tab === name);
    });
}
```

**Anomaly detection:**
```javascript
function detectAnomalies(stats) {
    stats.forEach(s => {
        const errPct = s.err_count / (s.count || 1);
        const avgMs = s.avg_ns / 1_000_000;
        if (errPct > 0.5 || avgMs > 5) {
            showAlert(s.name, errPct, avgMs);
        }
    });
}
```

### Dashboard CSS Conventions

All CSS is inline in `<style>` blocks within each HTML file. Use CSS custom properties for the color palette:

```css
:root {
    --bg:        #0d1117;
    --bg2:       #161b22;
    --border:    #30363d;
    --text:      #c9d1d9;
    --text-dim:  #8b949e;
    --primary:   #79c0ff;
    --success:   #3fb950;
    --warning:   #d29922;
    --danger:    #f85149;
    --accent:    #bc8cff;
}

body {
    background: var(--bg);
    color: var(--text);
    font-family: 'JetBrains Mono', 'Fira Code', monospace;
    font-size: 13px;
}

table {
    width: 100%;
    border-collapse: collapse;
}

th, td {
    padding: 6px 12px;
    border-bottom: 1px solid var(--border);
    text-align: left;
}

th {
    color: var(--text-dim);
    font-weight: 500;
    cursor: pointer;
    user-select: none;
}

th:hover { color: var(--primary); }

tr:hover td { background: var(--bg2); }
```

---

## Context B: Hugo Documentation Site (`site/`)

**Stack:** Hugo static site generator, custom theme `stracectl`, vanilla CSS (`main.css`), vanilla JS (`main.js`). Deployed to GitHub Pages at `https://fabianoflorentino.github.io/stracectl/`.

**Constraints:**
- No CSS preprocessor (no Sass/LESS) — plain CSS with custom properties
- No external CSS framework (no Tailwind, no Bootstrap)
- Dark terminal aesthetic — `--bg: #0a0e17`
- Fonts: `Inter` for body, `JetBrains Mono` / `Fira Code` for code/terminal
- Animations via pure CSS (`@keyframes fadeInUp`, `glow-pulse`, `blink-cursor`, etc.)
- Hugo templates use `.html` Go template syntax, not JSX
- `unsafe = true` in goldmark renderer — raw HTML in Markdown is allowed

### Hugo Template Conventions

```html
<!-- layouts/_default/baseof.html -->
<!DOCTYPE html>
<html lang="{{ .Site.LanguageCode }}">
<head>
    {{ partial "head.html" . }}
</head>
<body>
    {{ partial "header.html" . }}
    <main>
        {{ block "main" . }}{{ end }}
    </main>
    {{ partial "footer.html" . }}
    <script src="{{ "js/main.js" | relURL }}"></script>
</body>
</html>
```

```html
<!-- partials/head.html — SEO + meta -->
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{ if .IsHome }}{{ .Site.Title }}{{ else }}{{ .Title }} | {{ .Site.Title }}{{ end }}</title>
<meta name="description" content="{{ with .Description }}{{ . }}{{ else }}{{ .Site.Params.description }}{{ end }}">
<link rel="canonical" href="{{ .Permalink }}">
<link rel="stylesheet" href="{{ "css/main.css" | relURL }}">
```

### Hugo CSS Conventions

```css
/* site/themes/stracectl/static/css/main.css */
:root {
    --bg:         #0a0e17;
    --bg2:        #111827;
    --border:     #1f2937;
    --text:       #e2e8f0;
    --text-muted: #6b7280;
    --primary:    #60a5fa;
    --accent:     #a78bfa;
    --success:    #34d399;
    --mono:       'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace;
    --sans:       'Inter', system-ui, sans-serif;
}

/* Animation — used on hero section */
@keyframes fadeInUp {
    from { opacity: 0; transform: translateY(24px); }
    to   { opacity: 1; transform: translateY(0); }
}

@keyframes glow-pulse {
    0%, 100% { text-shadow: 0 0 8px var(--primary); }
    50%       { text-shadow: 0 0 24px var(--primary), 0 0 48px var(--accent); }
}

@keyframes blink-cursor {
    0%, 100% { opacity: 1; }
    50%       { opacity: 0; }
}

/* Scroll-reveal (toggled by IntersectionObserver in main.js) */
.reveal {
    opacity: 0;
    transform: translateY(20px);
    transition: opacity 0.6s ease, transform 0.6s ease;
}
.reveal.visible {
    opacity: 1;
    transform: none;
}
```

### Hugo JS Conventions

```javascript
// site/themes/stracectl/static/js/main.js

// Scroll reveal
const observer = new IntersectionObserver((entries) => {
    entries.forEach(e => {
        if (e.isIntersecting) {
            e.target.classList.add('visible');
            observer.unobserve(e.target);
        }
    });
}, { threshold: 0.1 });

document.querySelectorAll('.reveal, .reveal-right').forEach(el => {
    observer.observe(el);
});

// Typewriter effect on hero terminal
function typewriter(el, text, speed = 40) {
    let i = 0;
    el.textContent = '';
    const cursor = document.createElement('span');
    cursor.className = 'cursor';
    el.after(cursor);

    const interval = setInterval(() => {
        el.textContent += text[i++];
        if (i >= text.length) clearInterval(interval);
    }, speed);
}

// Latest release from GitHub API
async function fetchLatestRelease() {
    try {
        const res = await fetch('https://api.github.com/repos/fabianoflorentino/stracectl/releases/latest');
        const data = await res.json();
        document.querySelectorAll('.release-tag').forEach(el => {
            el.textContent = data.tag_name;
        });
    } catch (_) {
        // silently ignore — version badge is non-critical
    }
}
fetchLatestRelease();

// Copy to clipboard
document.querySelectorAll('[data-copy]').forEach(btn => {
    btn.addEventListener('click', () => {
        navigator.clipboard.writeText(btn.dataset.copy).then(() => {
            btn.textContent = 'Copied!';
            setTimeout(() => { btn.textContent = 'Copy'; }, 2000);
        });
    });
});
```

### Hugo Shortcode Conventions

```html
<!-- layouts/shortcodes/features.html -->
<section class="features reveal" id="features">
    <div class="features-grid">
        {{ range .Params.items }}
        <div class="feature-card">
            <div class="feature-icon">{{ .icon }}</div>
            <h3>{{ .title }}</h3>
            <p>{{ .desc }}</p>
        </div>
        {{ end }}
    </div>
</section>
```

```markdown
<!-- Usage in content -->
{{< features items='[
  {"icon": "⚡", "title": "Real-time TUI", "desc": "htop-style live syscall dashboard"},
  {"icon": "☸️", "title": "Kubernetes", "desc": "Sidecar mode for pod-level tracing"}
]' >}}
```

---

## Design Aesthetic Guidelines

Both frontends share a **dark terminal aesthetic**:
- Deep navy/charcoal backgrounds (`#0a0e17` – `#161b22`)
- Monospace fonts for all data and code
- Muted text for secondary info, bright accent for interactive elements
- No gradients on backgrounds — flat dark panels with subtle borders
- Syntax-highlight-inspired color coding for categories and status values
- Minimal chrome — data density is a feature, not a problem

**NEVER use:**
- Light themes or white backgrounds
- Rounded "card" designs with heavy shadows
- Purple-on-white gradients or "generic SaaS" aesthetics
- External font CDNs (self-host or use system stack fallbacks)
- CSS frameworks that would add external dependencies

**Commit to the terminal tool aesthetic:** this is a CLI/eBPF diagnostic tool, not a marketing dashboard. The UI should feel like it belongs in a terminal environment.
