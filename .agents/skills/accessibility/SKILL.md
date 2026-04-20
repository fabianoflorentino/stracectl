---
name: accessibility
description: Audit and improve web accessibility following WCAG 2.2 guidelines. Use when asked to "improve accessibility", "a11y audit", "WCAG compliance", "screen reader support", "keyboard navigation", or "make accessible".
license: MIT
metadata:
  author: web-quality-skills
  version: "1.2"
---

# Accessibility (a11y)

Comprehensive accessibility guidelines based on WCAG 2.2. Goal: make content usable by everyone, including people with disabilities.

This project has **two frontend contexts** with different a11y concerns:

- **Embedded dashboard** (`internal/server/static/`) — vanilla JS, `innerHTML`-heavy, dark theme, data tables, WebSocket live updates
- **Hugo site** (`site/`) — static documentation site, Hugo templates, markdown content

---

## WCAG Principles: POUR

| Principle | Description |
|-----------|-------------|
| **P**erceivable | Content can be perceived through different senses |
| **O**perable | Interface can be operated by all users |
| **U**nderstandable | Content and interface are understandable |
| **R**obust | Content works with assistive technologies |

## Conformance Levels

| Level | Requirement |
|-------|-------------|
| **A** | Minimum — must pass |
| **AA** | Standard — legal requirement in many jurisdictions |
| **AAA** | Enhanced — aim for where feasible |

---

## Dashboard-Specific Patterns

### Safe innerHTML with ARIA

The dashboard renders all content via `innerHTML`. Every dynamic insertion must use `esc()` to prevent XSS **and** include proper ARIA attributes for screen readers:

```javascript
// ❌ No ARIA, no escaping
function renderRow(stat) {
    return `<tr><td>${stat.name}</td><td>${stat.count}</td></tr>`;
}

// ✅ Escaped values + ARIA roles on the table
function renderRow(stat) {
    const errPct = (stat.err_count / (stat.count || 1) * 100).toFixed(1);
    return `<tr>
        <td>${esc(stat.name)}</td>
        <td aria-label="${esc(fmtN(stat.count))} calls">${esc(fmtN(stat.count))}</td>
        <td aria-label="${esc(fmtDur(stat.avg_ns))} average">${esc(fmtDur(stat.avg_ns))}</td>
        <td aria-label="${errPct}% error rate">${errPct}%</td>
    </tr>`;
}
```

The surrounding table must declare its role and caption:

```html
<table role="grid" aria-label="Syscall statistics" aria-live="polite" aria-atomic="false">
    <caption class="visually-hidden">Live syscall statistics, updated every second</caption>
    <thead>
        <tr>
            <th scope="col" aria-sort="none" data-sort="name">Syscall</th>
            <th scope="col" aria-sort="descending" data-sort="count">Count</th>
            <th scope="col" aria-sort="none" data-sort="avg_ns">Avg Latency</th>
            <th scope="col" aria-sort="none" data-sort="err_pct">Errors</th>
        </tr>
    </thead>
    <tbody id="stats-body"></tbody>
</table>
```

Update `aria-sort` on column headers when the user sorts:

```javascript
function updateSortAria(field, dir) {
    document.querySelectorAll('th[data-sort]').forEach(th => {
        if (th.dataset.sort === field) {
            th.setAttribute('aria-sort', dir === -1 ? 'descending' : 'ascending');
        } else {
            th.setAttribute('aria-sort', 'none');
        }
    });
}
```

### Live Regions for WebSocket Updates

Avoid announcing every row change (too noisy). Use a status live region for connection state and anomaly alerts only:

```html
<!-- Announce connection status and alerts — not the whole table -->
<div role="status" aria-live="polite" aria-atomic="true" id="a11y-status" class="visually-hidden"></div>
<div role="alert"  aria-live="assertive" aria-atomic="true" id="a11y-alert" class="visually-hidden"></div>
```

```javascript
function announceStatus(msg) {
    document.getElementById('a11y-status').textContent = msg;
}

function announceAlert(msg) {
    document.getElementById('a11y-alert').textContent = '';
    // Force re-announcement on repeated alerts
    requestAnimationFrame(() => {
        document.getElementById('a11y-alert').textContent = msg;
    });
}

ws.onopen  = () => announceStatus('Connected. Receiving live syscall data.');
ws.onclose = () => announceStatus('Connection lost. Reconnecting…');

function detectAnomalies(stats) {
    const flagged = stats.filter(s => s.err_count / (s.count || 1) > 0.5);
    if (flagged.length > 0) {
        announceAlert(`Anomaly detected: ${flagged.map(s => s.name).join(', ')} has high error rate.`);
    }
}
```

### Tab Panel Accessibility

```html
<div role="tablist" aria-label="Dashboard sections">
    <button role="tab" aria-selected="true"  aria-controls="panel-stats"  id="tab-stats"  tabindex="0">Stats</button>
    <button role="tab" aria-selected="false" aria-controls="panel-log"    id="tab-log"    tabindex="-1">Log</button>
    <button role="tab" aria-selected="false" aria-controls="panel-files"  id="tab-files"  tabindex="-1">Files</button>
</div>

<div role="tabpanel" id="panel-stats"  aria-labelledby="tab-stats"  tabindex="0">...</div>
<div role="tabpanel" id="panel-log"    aria-labelledby="tab-log"    tabindex="0" hidden>...</div>
<div role="tabpanel" id="panel-files"  aria-labelledby="tab-files"  tabindex="0" hidden>...</div>
```

```javascript
// Keyboard navigation for tabs (arrow keys + Home/End)
tablist.addEventListener('keydown', e => {
    const tabs = [...tablist.querySelectorAll('[role="tab"]')];
    const idx = tabs.indexOf(document.activeElement);

    let next = -1;
    if (e.key === 'ArrowRight') next = (idx + 1) % tabs.length;
    if (e.key === 'ArrowLeft')  next = (idx - 1 + tabs.length) % tabs.length;
    if (e.key === 'Home')       next = 0;
    if (e.key === 'End')        next = tabs.length - 1;

    if (next >= 0) {
        e.preventDefault();
        activateTab(tabs[next]);
        tabs[next].focus();
    }
});
```

### Search/Filter Input

```html
<label for="filter-input" class="visually-hidden">Filter syscalls</label>
<input
    id="filter-input"
    type="search"
    placeholder="Filter syscalls…"
    aria-controls="stats-body"
    aria-label="Filter syscalls by name"
    autocomplete="off"
    spellcheck="false">
<span id="filter-count" aria-live="polite" class="visually-hidden"></span>
```

```javascript
filterInput.addEventListener('input', () => {
    const visible = filterTable(filterInput.value);
    document.getElementById('filter-count').textContent =
        `${visible} syscall${visible !== 1 ? 's' : ''} shown`;
});
```

---

## Perceivable

### Color Contrast — Dark Theme

The dashboard and Hugo site both use dark backgrounds. Verify contrast ratios:

| Token | Background | Text | Ratio | AA Pass |
|-------|-----------|------|-------|---------|
| `--text` `#c9d1d9` on `--bg` `#0d1117` | 12.5:1 | ✅ |
| `--text-dim` `#8b949e` on `--bg` `#0d1117` | 5.8:1 | ✅ |
| `--primary` `#79c0ff` on `--bg` `#0d1117` | 7.2:1 | ✅ |
| `--danger` `#f85149` on `--bg` `#0d1117` | 4.6:1 | ✅ |
| `--warning` `#d29922` on `--bg` `#0d1117` | 3.8:1 | ⚠️ Borderline for normal text |

**Warning color rule:** Use `--warning` only for large text, icons, or borders — never for body text below 18px.

### Don't Rely on Color Alone

```javascript
// ❌ Only color differentiates error rows
return `<tr style="color: ${stat.err_pct > 50 ? 'red' : 'inherit'}">`;

// ✅ Color + icon + ARIA
const icon = stat.err_pct > 50 ? '⚠ ' : '';
return `<tr aria-label="${esc(stat.name)}${stat.err_pct > 50 ? ', high error rate' : ''}">
    <td>${icon}${esc(stat.name)}</td>
</tr>`;
```

### Images — Hugo Site

```html
<!-- layouts/shortcodes/hero.html -->
<picture>
    <source srcset="/img/hero.gif" type="image/gif">
    <!-- ✅ Descriptive alt — describes what the image shows -->
    <img src="/img/hero.svg"
         alt="stracectl TUI showing live syscall dashboard with sortable table of read, write, and open calls"
         width="900"
         height="500"
         loading="lazy">
</picture>

<!-- Screenshots — decorative ones can use empty alt -->
<img src="/img/tui_1.jpg"
     alt="stracectl TUI displaying syscall statistics with latency histogram"
     loading="lazy">
```

---

## Operable

### Keyboard Accessible

```javascript
// ❌ Only handles click
sortBtn.addEventListener('click', handleSort);

// ✅ Handles click and keyboard (Enter/Space)
sortBtn.addEventListener('click', handleSort);
sortBtn.addEventListener('keydown', e => {
    if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        handleSort();
    }
});
```

Interactive elements that are not native `<button>` or `<a>` must have `tabindex="0"` and explicit keyboard handling. **Prefer native elements whenever possible.**

### Focus Visible — Dark Theme

```css
/* Works on both the dashboard and Hugo site */
:focus {
    outline: none;
}

:focus-visible {
    outline: 2px solid var(--primary, #79c0ff);
    outline-offset: 2px;
    border-radius: 2px;
}

/* Buttons with custom backgrounds */
button:focus-visible {
    box-shadow: 0 0 0 3px rgba(121, 192, 255, 0.4);
    outline: none;
}
```

### Target Size (WCAG 2.5.8)

Interactive elements must be at least **24×24 CSS pixels**:

```css
/* Dashboard buttons */
button, [role="button"], [role="tab"] {
    min-width: 24px;
    min-height: 24px;
    padding: 4px 8px;
}

/* Comfortable size for primary actions */
.btn-primary {
    min-height: 36px;
    padding: 6px 16px;
}
```

### Skip Links — Hugo Site

Add a skip link in `baseof.html` before any navigation:

```html
<!-- layouts/_default/baseof.html -->
<a class="skip-link" href="#main-content">Skip to main content</a>
<header>...</header>
<main id="main-content" tabindex="-1">
    {{ block "main" . }}{{ end }}
</main>
```

```css
/* main.css */
.skip-link {
    position: absolute;
    top: -40px;
    left: 0;
    background: var(--primary);
    color: var(--bg);
    padding: 8px 16px;
    z-index: 9999;
    font-weight: 600;
    text-decoration: none;
    border-radius: 0 0 4px 0;
    transition: top 0.1s;
}

.skip-link:focus {
    top: 0;
}
```

### Reduced Motion

```css
/* Respect user preference — applies to both dashboard and Hugo site */
@media (prefers-reduced-motion: reduce) {
    *,
    *::before,
    *::after {
        animation-duration: 0.01ms !important;
        animation-iteration-count: 1 !important;
        transition-duration: 0.01ms !important;
        scroll-behavior: auto !important;
    }
}
```

---

## Understandable

### Page Language

```html
<!-- Hugo baseof.html -->
<html lang="{{ .Site.LanguageCode }}">

<!-- hugo.toml -->
<!-- languageCode = "en-us" -->
```

### Form Labels — Dashboard Filter

Every input must have a programmatically associated label:

```html
<!-- ✅ Explicit label via for/id -->
<label for="filter-input">Filter syscalls</label>
<input id="filter-input" type="search" placeholder="e.g. read, open">

<!-- ✅ visually-hidden label when design needs no visible label -->
<label for="limit-input" class="visually-hidden">Results limit</label>
<input id="limit-input" type="number" min="10" max="500" value="100">
```

### Error Identification — Dashboard

If an input has validation (e.g., numeric limit out of range):

```html
<input id="limit-input" type="number"
       aria-describedby="limit-error"
       aria-invalid="false">
<span id="limit-error" role="alert" class="visually-hidden"></span>
```

```javascript
function validateLimit(val) {
    const input = document.getElementById('limit-input');
    const error = document.getElementById('limit-error');
    if (val < 10 || val > 10000) {
        input.setAttribute('aria-invalid', 'true');
        error.textContent = 'Limit must be between 10 and 10,000.';
    } else {
        input.setAttribute('aria-invalid', 'false');
        error.textContent = '';
    }
}
```

---

## Robust

### Prefer Native Elements

```html
<!-- ❌ ARIA div-button -->
<div role="button" tabindex="0" onclick="handleSort()">Sort</div>

<!-- ✅ Native button -->
<button type="button" onclick="handleSort()">Sort</button>

<!-- ❌ ARIA checkbox -->
<div role="checkbox" aria-checked="false">Pause log</div>

<!-- ✅ Native checkbox -->
<label><input type="checkbox" id="pause-log"> Pause log</label>
```

### ARIA on Dynamic Content

When updating content via `innerHTML`, ensure ARIA container attributes are on the **static** wrapper, not on the dynamically inserted rows:

```html
<!-- ✅ aria-live on the static container -->
<tbody id="stats-body" aria-live="polite" aria-relevant="all">
    <!-- rows inserted by JS here -->
</tbody>
```

---

## Visually Hidden Utility Class

Used throughout both frontends to hide content visually while keeping it accessible to screen readers:

```css
.visually-hidden {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
}
```

---

## Testing Checklist

### Automated Testing

```bash
# Lighthouse accessibility audit on Hugo site
npx lighthouse https://fabianoflorentino.github.io/stracectl/ --only-categories=accessibility

# axe-core on the embedded dashboard
npm install -g @axe-core/cli
axe http://localhost:8080
```

### Manual Testing

- [ ] **Keyboard navigation:** Tab through entire dashboard; all interactive elements reachable
- [ ] **Tab panels:** Arrow keys navigate between tabs; Enter/Space activates
- [ ] **Sort columns:** Keyboard-operable; `aria-sort` updates correctly
- [ ] **Filter input:** Screen reader announces result count changes
- [ ] **WebSocket alerts:** Anomaly alerts announced without focus moving
- [ ] **Skip link:** Visible on focus; skips to `#main-content`
- [ ] **Zoom:** Dashboard usable at 200% zoom (no horizontal scroll in table)
- [ ] **Reduced motion:** Animations disabled when `prefers-reduced-motion: reduce`
- [ ] **High contrast:** Dark theme tokens maintain contrast in Windows High Contrast mode
- [ ] **Focus indicators:** Visible on all focusable elements in both frontends

---

## Common Issues by Impact

### Critical (fix immediately)
1. Missing `aria-label` on icon-only buttons (e.g., sort indicators)
2. `innerHTML` updates without `aria-live` regions
3. Tab panels without keyboard arrow-key navigation
4. Color-only error indicators (no text/icon backup)
5. No focus indicators on dark background

### Serious (fix before launch)
1. Missing skip link on Hugo site
2. Table columns without `scope="col"` / `aria-sort`
3. Search input without label
4. Missing `lang` attribute on `<html>`

### Moderate (fix soon)
1. Filter result count not announced to screen readers
2. WebSocket reconnect status not announced
3. Inconsistent focus order after tab switching
4. Missing `alt` text on screenshot images in Hugo docs

## References

- [WCAG 2.2 Quick Reference](https://www.w3.org/WAI/WCAG22/quickref/)
- [WAI-ARIA Authoring Practices](https://www.w3.org/WAI/ARIA/apg/)
- [Deque axe Rules](https://dequeuniversity.com/rules/axe/)
