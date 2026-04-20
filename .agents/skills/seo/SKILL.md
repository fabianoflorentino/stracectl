---
name: seo
description: Optimize for search engine visibility and ranking. Use when asked to "improve SEO", "optimize for search", "fix meta tags", "add structured data", "sitemap optimization", or "search engine optimization".
license: MIT
metadata:
  author: web-quality-skills
  version: "1.1"
---

# SEO optimization

Search engine optimization based on Lighthouse SEO audits and Google Search guidelines. Focus on technical SEO, on-page optimization, and structured data.

This project's public site is a **Hugo static site** deployed to GitHub Pages at `https://fabianoflorentino.github.io/stracectl/`. All SEO improvements must be applied within the Hugo template and content system.

## SEO fundamentals

Search ranking factors (approximate influence):

| Factor | Influence | This Skill |
|--------|-----------|------------|
| Content quality & relevance | ~40% | Partial (structure) |
| Backlinks & authority | ~25% | ✗ |
| Technical SEO | ~15% | ✓ |
| Page experience (Core Web Vitals) | ~10% | Partial |
| On-page SEO | ~10% | ✓ |

---

## Hugo-Specific SEO

### Meta Tags via Partials

All meta tags live in `site/themes/stracectl/layouts/partials/head.html`. Hugo provides page variables for dynamic values:

```html
<!-- partials/head.html -->
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">

<!-- Title: homepage vs inner pages -->
<title>
  {{ if .IsHome }}
    {{ .Site.Title }} — {{ .Site.Params.tagline }}
  {{ else }}
    {{ .Title }} | {{ .Site.Title }}
  {{ end }}
</title>

<!-- Description: page-level or site fallback -->
<meta name="description" content="{{ with .Description }}{{ . }}{{ else }}{{ .Site.Params.description }}{{ end }}">

<!-- Canonical URL -->
<link rel="canonical" href="{{ .Permalink }}">

<!-- Open Graph -->
<meta property="og:title"       content="{{ .Title }}">
<meta property="og:description" content="{{ with .Description }}{{ . }}{{ else }}{{ .Site.Params.description }}{{ end }}">
<meta property="og:url"         content="{{ .Permalink }}">
<meta property="og:type"        content="{{ if .IsHome }}website{{ else }}article{{ end }}">
<meta property="og:image"       content="{{ .Site.BaseURL }}img/hero.svg">

<!-- Twitter Card -->
<meta name="twitter:card"        content="summary_large_image">
<meta name="twitter:title"       content="{{ .Title }}">
<meta name="twitter:description" content="{{ with .Description }}{{ . }}{{ else }}{{ .Site.Params.description }}{{ end }}">
<meta name="twitter:image"       content="{{ .Site.BaseURL }}img/hero.svg">
```

### Page-Level SEO via Front Matter

Set `description` in each content file's front matter to override the site default:

```markdown
---
title: "Quickstart"
description: "Install stracectl and trace your first process in under 2 minutes. Supports strace and eBPF backends."
---
```

### Hugo Sitemap

Hugo generates `/sitemap.xml` automatically. Ensure it is enabled and submitted to Google Search Console. Control per-page inclusion via front matter:

```markdown
---
title: "Privacy Policy"
sitemap:
  priority: 0.2
  changefreq: yearly
---
```

Global sitemap config in `hugo.toml`:

```toml
[sitemap]
  changefreq = "weekly"
  priority   = 0.5
  filename   = "sitemap.xml"
```

### robots.txt for Hugo

Create `site/static/robots.txt` (Hugo copies static files as-is):

```text
User-agent: *
Allow: /

Sitemap: https://fabianoflorentino.github.io/stracectl/sitemap.xml
```

---

## Technical SEO

### Crawlability

**Meta robots** — add to `head.html` for pages that should not be indexed:

```html
{{ if .Params.noindex }}
<meta name="robots" content="noindex, nofollow">
{{ else }}
<meta name="robots" content="index, follow">
{{ end }}
```

### Canonical URLs

Hugo's `.Permalink` is always the canonical URL. Always render it:

```html
<link rel="canonical" href="{{ .Permalink }}">
```

For GitHub Pages with a subpath (`/stracectl/`), ensure `baseURL` in `hugo.toml` is set correctly:

```toml
baseURL = "https://fabianoflorentino.github.io/stracectl/"
```

### HTTPS

GitHub Pages enforces HTTPS. Always use `https://` in `baseURL`. Ensure all internal links use `{{ relURL }}` or `{{ absURL }}` — never hardcode `http://`.

---

## On-page SEO

### Title Tags

```html
<!-- ❌ Generic -->
<title>stracectl</title>

<!-- ✅ Descriptive with primary keyword -->
<title>stracectl — Modern strace with real-time TUI and Kubernetes sidecar</title>

<!-- ✅ Inner page -->
<title>Quickstart | stracectl</title>
```

**Guidelines:**
- 50–60 characters
- Primary keyword near the beginning
- Unique for every page
- Brand name at end (except homepage)

### Meta Descriptions

```toml
# hugo.toml
[params]
  description = "A modern strace with real-time htop-style TUI and Kubernetes sidecar mode. Aggregate syscalls live — counts, latencies, errors, anomalies."
```

**Guidelines:**
- 150–160 characters
- Include primary keyword naturally
- Compelling call-to-action
- Unique for every page

### Heading Structure

```markdown
# stracectl — Modern Syscall Tracer     ← single h1 per page

## Features                              ← h2 for major sections
### Real-time TUI                        ← h3 for subsections
### Kubernetes Sidecar Mode

## Installation
```

### Image SEO

```html
<!-- hero shortcode — layouts/shortcodes/hero.html -->
<picture>
  <source srcset="/img/hero.gif" type="image/gif">
  <!-- ✅ Descriptive alt text -->
  <img src="/img/hero.svg"
       alt="stracectl TUI showing live syscall dashboard with counts, latency, and error rates"
       width="900"
       height="500"
       loading="lazy">
</picture>
```

### Internal Linking

```markdown
<!-- ❌ Non-descriptive -->
[Click here](/docs/quickstart/) to get started.

<!-- ✅ Descriptive anchor text -->
Follow the [quickstart guide](/docs/quickstart/) to trace your first process.
See [Kubernetes sidecar mode](/docs/kubernetes/) for pod-level tracing.
```

---

## Structured Data (JSON-LD)

### Software Application

Add to `partials/head.html` for the homepage:

```html
{{ if .IsHome }}
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "stracectl",
  "applicationCategory": "DeveloperApplication",
  "operatingSystem": "Linux",
  "description": "{{ .Site.Params.description }}",
  "url": "{{ .Site.BaseURL }}",
  "author": {
    "@type": "Person",
    "name": "{{ .Site.Params.author }}"
  },
  "license": "{{ .Site.Params.license_url }}",
  "codeRepository": "{{ .Site.Params.github }}"
}
</script>
{{ end }}
```

### Breadcrumbs for Docs

Add to `layouts/_default/single.html` for doc pages:

```html
{{ if .Section }}
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "BreadcrumbList",
  "itemListElement": [
    {
      "@type": "ListItem",
      "position": 1,
      "name": "Home",
      "item": "{{ .Site.BaseURL }}"
    },
    {
      "@type": "ListItem",
      "position": 2,
      "name": "Docs",
      "item": "{{ .Site.BaseURL }}docs/"
    },
    {
      "@type": "ListItem",
      "position": 3,
      "name": "{{ .Title }}",
      "item": "{{ .Permalink }}"
    }
  ]
}
</script>
{{ end }}
```

### Validation

- [Google Rich Results Test](https://search.google.com/test/rich-results)
- [Schema.org Validator](https://validator.schema.org/)

---

## Mobile SEO

### Responsive Viewport

Already in `head.html`:

```html
<meta name="viewport" content="width=device-width, initial-scale=1">
```

### Font Sizes

```css
/* site/themes/stracectl/static/css/main.css */
body {
    font-size: 16px;
    line-height: 1.6;
}

/* Readable on mobile without pinch-zoom */
@media (max-width: 600px) {
    body { font-size: 15px; }
    pre, code { font-size: 13px; }
}
```

---

## SEO Audit Checklist

### Critical
- [ ] `baseURL` set correctly in `hugo.toml`
- [ ] HTTPS enforced (GitHub Pages default)
- [ ] `robots.txt` allows crawling
- [ ] `sitemap.xml` generated by Hugo
- [ ] Title tags present and unique on all pages
- [ ] Single `<h1>` per page

### High Priority
- [ ] Meta descriptions in front matter for key pages
- [ ] Canonical `<link>` rendered via `.Permalink`
- [ ] Open Graph tags in `head.html`
- [ ] Descriptive alt text on all images
- [ ] Mobile-responsive (viewport meta present)

### Medium Priority
- [ ] JSON-LD structured data on homepage
- [ ] Breadcrumb schema on doc pages
- [ ] Descriptive internal link anchor text
- [ ] `sitemap` front matter priority on low-value pages

### Ongoing
- [ ] Submit sitemap to Google Search Console
- [ ] Monitor `robots.txt` hasn't blocked key paths after Hugo updates
- [ ] Review page titles when content changes

---

## Tools

| Tool | Use |
|------|-----|
| Google Search Console | Monitor indexing, submit sitemap |
| Google PageSpeed Insights | Performance + Core Web Vitals |
| Rich Results Test | Validate structured data |
| Lighthouse | Full SEO + performance audit |
| Hugo built-in | `hugo --minify` for smaller HTML output |

## References

- [Google Search Central](https://developers.google.com/search)
- [Schema.org](https://schema.org/)
- [Hugo SEO docs](https://gohugo.io/templates/sitemap-template/)
