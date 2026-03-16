/* ============================================================
   STRACECTL — main.js
   ============================================================ */

(function () {
  'use strict';

  /* ──────────────────────────────────────────────────────────
   * 1. Navbar: add .scrolled class after scrolling
   * ──────────────────────────────────────────────────────────*/
  const header = document.getElementById('site-header');
  if (header) {
    const onScroll = () => {
      header.classList.toggle('scrolled', window.scrollY > 20);
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
  }

  /* ──────────────────────────────────────────────────────────
   * 2. Mobile nav toggle
   * ──────────────────────────────────────────────────────────*/
  const burger = document.getElementById('nav-burger');
  const navLinks = document.getElementById('nav-links');
  if (burger && navLinks) {
    burger.addEventListener('click', () => {
      const open = navLinks.classList.toggle('open');
      burger.setAttribute('aria-expanded', String(open));
    });
    // Close on nav link click
    navLinks.querySelectorAll('a').forEach(a => {
      a.addEventListener('click', () => {
        navLinks.classList.remove('open');
        burger.setAttribute('aria-expanded', 'false');
      });
    });
  }

  /* ──────────────────────────────────────────────────────────
   * 3. Scroll reveal (Intersection Observer)
   *    - observe header and global reveal elements, but exclude
   *      children of any `.reveal--delayed` containers so we can
   *      reveal them after the header animation with a stagger
   * ──────────────────────────────────────────────────────────*/
  const headerRevealEls = Array.from(document.querySelectorAll('.page-header .reveal, .page-header .reveal-right'));
  const delayedContainers = Array.from(document.querySelectorAll('.reveal--delayed'));
  const bodyRevealEls = delayedContainers.flatMap(container => Array.from(container.querySelectorAll('.reveal, .reveal-right')));
  const otherRevealEls = Array.from(document.querySelectorAll('.reveal, .reveal-right')).filter(el => !el.closest('.page-header') && !el.closest('.reveal--delayed'));
  const observedEls = headerRevealEls.concat(otherRevealEls);

  if (observedEls.length) {
    let delayedTriggered = false;
    const io = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          entry.target.classList.add('visible');
          // Trigger delayed reveal once when a header element becomes visible
          if (!delayedTriggered && entry.target.closest && entry.target.closest('.page-header')) {
            delayedTriggered = true;
            setTimeout(() => {
              // reveal container(s)
              delayedContainers.forEach(el => el.classList.add('visible'));
              // reveal children inside delayed containers all at once (no stagger)
              bodyRevealEls.forEach(el => el.classList.add('visible'));
            }, 320);
          }
          io.unobserve(entry.target);
        }
      });
    }, { threshold: 0.12, rootMargin: '0px 0px -40px 0px' });

    observedEls.forEach(el => io.observe(el));
  }

  /* ──────────────────────────────────────────────────────────
   * 4. Copy buttons
   * ──────────────────────────────────────────────────────────*/
  document.querySelectorAll('.copy-btn[data-copy]').forEach(btn => {
    btn.addEventListener('click', () => {
      const text = btn.dataset.copy;
      navigator.clipboard.writeText(text).then(() => {
        btn.classList.add('copied');
        const orig = btn.innerHTML;
        btn.innerHTML =
          '<svg width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">' +
          '<polyline points="20 6 9 17 4 12"/></svg>';
        setTimeout(() => {
          btn.innerHTML = orig;
          btn.classList.remove('copied');
        }, 2000);
      });
    });
  });

  /* ──────────────────────────────────────────────────────────
   * 5. Install tabs
   * ──────────────────────────────────────────────────────────*/
  const tabs = document.querySelectorAll('.install-tab');
  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      document.querySelectorAll('.install-tab-content').forEach(c => c.classList.remove('active'));
      tab.classList.add('active');
      const target = document.getElementById(tab.dataset.target);
      if (target) target.classList.add('active');
    });
  });

  /* ──────────────────────────────────────────────────────────
   * 6. Particle canvas (hero background)
   * ──────────────────────────────────────────────────────────*/
  const canvas = document.getElementById('particle-canvas');
  if (canvas) {
    const ctx = canvas.getContext('2d');
    const CHARS = '01アイウエオカキクケコサシスセソタチツテト<>{}[]()/*+-=;:,.';
    let W, H, particles;
    const PARTICLE_COUNT = 60;

    function resize() {
      W = canvas.width  = canvas.offsetWidth;
      H = canvas.height = canvas.offsetHeight;
    }

    function makeParticle() {
      return {
        x: Math.random() * W,
        y: Math.random() * H,
        char: CHARS[Math.floor(Math.random() * CHARS.length)],
        speed: 0.3 + Math.random() * 0.7,
        opacity: 0.03 + Math.random() * 0.08,
        size: 10 + Math.floor(Math.random() * 4),
        timer: 0,
        interval: 40 + Math.floor(Math.random() * 80),
      };
    }

    function init() {
      resize();
      particles = Array.from({ length: PARTICLE_COUNT }, makeParticle);
    }

    function animate() {
      ctx.clearRect(0, 0, W, H);
      particles.forEach(p => {
        p.y += p.speed;
        p.timer++;
        if (p.timer > p.interval) {
          p.char = CHARS[Math.floor(Math.random() * CHARS.length)];
          p.timer = 0;
        }
        if (p.y > H) {
          p.y = -20;
          p.x = Math.random() * W;
        }
        ctx.globalAlpha = p.opacity;
        ctx.fillStyle = '#00d4aa';
        ctx.font = `${p.size}px 'JetBrains Mono', monospace`;
        ctx.fillText(p.char, p.x, p.y);
      });
      ctx.globalAlpha = 1;
      requestAnimationFrame(animate);
    }

    window.addEventListener('resize', () => { resize(); });
    init();
    animate();
  }

  /* ──────────────────────────────────────────────────────────
   * 7. Terminal TUI animation
   * ──────────────────────────────────────────────────────────*/
  const terminalBody = document.getElementById('terminal-body');
  if (!terminalBody) return;

  const promptText = document.getElementById('prompt-text');
  const typingCursor = document.getElementById('typing-cursor');
  const tuiOutput = document.getElementById('tui-output');
  const tuiRows = document.getElementById('tui-rows');
  const tuiSyscalls = document.getElementById('tui-syscalls');
  const tuiRate = document.getElementById('tui-rate');
  const tuiErrors = document.getElementById('tui-errors');
  const tuiUnique = document.getElementById('tui-unique');
  const tuiElapsed = document.getElementById('tui-elapsed');
  const tuiAlertText = document.getElementById('tui-alert-text');

  const CMD = 'sudo stracectl run curl https://example.com';

  // TUI data frames (cycling animation)
  const DATA_ROWS = [
    { name: 'openat',  cat: 'I/O', calls: 0, freq: '████████░░░░', avg: '38.2µs', max: '3.1ms', errN: 8,  errP: '25%', cls: 'row-err' },
    { name: 'close',   cat: 'I/O', calls: 0, freq: '███████░░░░░', avg: '29.4µs', max: '412µs', errN: 0,  errP: '—',   cls: '' },
    { name: 'read',    cat: 'I/O', calls: 0, freq: '█████░░░░░░░', avg: '41.8µs', max: '1.9ms', errN: 1,  errP: '4%',  cls: '' },
    { name: 'connect', cat: 'NET', calls: 0, freq: '██░░░░░░░░░░', avg: '52.1µs', max: '312µs', errN: 2,  errP: '50%', cls: 'row-err' },
    { name: 'fstat',   cat: 'FS',  calls: 0, freq: '████░░░░░░░░', avg: '31.0µs', max: '580µs', errN: 0,  errP: '—',   cls: '' },
  ];

  const ALERTS = [
    'connect: 50% error rate — Happy Eyeballs: IPv4/IPv6 race, loser fails',
    'openat: 25% error rate — dynamic linker probes many paths (ENOENT expected)',
  ];

  let elapsed = 0;
  let syscalls = 0;
  let rate = 0;
  let errors = 0;

  // Build DOM rows once
  function buildRows() {
    tuiRows.innerHTML = '';
    DATA_ROWS.forEach((r, i) => {
      const div = document.createElement('div');
      div.className = 'tui-row ' + r.cls;
      div.id = 'row-' + i;
      div.innerHTML =
        `<span class="row-name"><span class="row-selector">${i === 0 ? '►' : ' '}</span><span>${r.name}</span></span>` +
        `<span class="row-cat">${r.cat}</span>` +
        `<span class="row-calls" id="rc-${i}">0</span>` +
        `<span class="row-freq">${r.freq}</span>` +
        `<span class="row-avg">${r.avg}</span>` +
        `<span class="row-max">${r.max}</span>` +
        `<span class="row-err-n">${r.errN > 0 ? r.errN : '—'}</span>` +
        `<span class="row-erp">${r.errP}</span>`;
      tuiRows.appendChild(div);
      // Staggered reveal
      setTimeout(() => div.classList.add('visible'), i * 120);
    });
  }

  let alertIdx = 0;
  let tuiTick = 0;

  function updateTUI() {
    tuiTick++;
    elapsed++;
    const newRate = 55 + Math.floor(Math.random() * 60);
    syscalls += newRate;
    errors += Math.floor(Math.random() * 3);
    rate = newRate;

    tuiElapsed.textContent = '+' + elapsed + 's';
    tuiSyscalls.textContent = syscalls.toLocaleString();
    tuiRate.textContent = rate;
    tuiErrors.textContent = errors;
    tuiUnique.textContent = 18 + Math.min(tuiTick, 5);

    DATA_ROWS.forEach((r, i) => {
      const newCalls = r.calls + Math.floor(newRate * (0.15 + Math.random() * 0.25));
      r.calls = newCalls;
      const el = document.getElementById('rc-' + i);
      if (el) el.textContent = newCalls;
    });

    // Rotate alert text every 5 ticks
    if (tuiTick % 5 === 0) {
      alertIdx = (alertIdx + 1) % ALERTS.length;
      if (tuiAlertText) tuiAlertText.textContent = ALERTS[alertIdx];
    }
  }

  // Typewriter
  let charIdx = 0;
  function typeChar() {
    if (charIdx < CMD.length) {
      promptText.textContent += CMD[charIdx++];
      setTimeout(typeChar, 45 + Math.random() * 35);
    } else {
      // Done typing — show TUI after a short pause
      setTimeout(showTUI, 600);
    }
  }

  function showTUI() {
    typingCursor.style.display = 'none';
    document.getElementById('terminal-prompt').style.opacity = '0.45';
    tuiOutput.classList.remove('hidden');
    buildRows();
    setInterval(updateTUI, 900);
  }

  // Start typewriter after 1.2s
  setTimeout(typeChar, 1200);
})();

/* ──────────────────────────────────────────────────────────
 * Screenshot Carousel (auto-advances every 2 s)
 * ──────────────────────────────────────────────────────────*/
(function () {
  'use strict';

  const INTERVAL_MS   = 5000;   // 5 seconds per slide
  const SLIDE_COUNT   = 4;

  const slides    = document.querySelectorAll('.carousel-slide');
  const dots      = document.querySelectorAll('.carousel-dot');
  const captions  = document.querySelectorAll('.carousel-caption');
  const progress  = document.getElementById('carousel-progress');
  const prevBtn   = document.getElementById('carousel-prev');
  const nextBtn   = document.getElementById('carousel-next');

  if (!slides.length) return;

  let current   = 0;
  let timer     = null;
  let startTime = null;
  let rafId     = null;
  let paused    = false;

  function goTo(idx, restart) {
    slides[current].classList.remove('active');
    dots[current].classList.remove('active');
    captions[current].classList.remove('active');

    current = (idx + SLIDE_COUNT) % SLIDE_COUNT;

    slides[current].classList.add('active');
    dots[current].classList.add('active');
    captions[current].classList.add('active');

    if (restart !== false) resetTimer();
  }

  /* Smooth progress bar via rAF */
  function animateProgress() {
    rafId = requestAnimationFrame(animateProgress);
    if (paused || !startTime) return;
    const elapsed = performance.now() - startTime;
    const pct = Math.min((elapsed / INTERVAL_MS) * 100, 100);
    if (progress) progress.style.width = pct + '%';
  }

  function resetTimer() {
    clearInterval(timer);
    startTime = performance.now();
    if (progress) progress.style.transition = 'none';
    if (progress) progress.style.width = '0%';
    // Force reflow to reset transition
    if (progress) void progress.offsetWidth;
    if (progress) progress.style.transition = '';

    timer = setInterval(() => {
      if (!paused) goTo(current + 1);
    }, INTERVAL_MS);
  }

  /* Pause on hover */
  const viewport = document.getElementById('carousel-viewport');
  if (viewport) {
    viewport.addEventListener('mouseenter', () => { paused = true; });
    viewport.addEventListener('mouseleave', () => {
      paused = false;
      startTime = performance.now();
    });
  }

  /* Prev / Next buttons */
  if (prevBtn) prevBtn.addEventListener('click', () => goTo(current - 1));
  if (nextBtn) nextBtn.addEventListener('click', () => goTo(current + 1));

  /* Dot navigation */
  dots.forEach(dot => {
    dot.addEventListener('click', () => goTo(Number(dot.dataset.idx)));
  });

  /* Touch / swipe support */
  let touchStartX = 0;
  if (viewport) {
    viewport.addEventListener('touchstart', e => {
      touchStartX = e.changedTouches[0].clientX;
    }, { passive: true });
    viewport.addEventListener('touchend', e => {
      const dx = e.changedTouches[0].clientX - touchStartX;
      if (Math.abs(dx) > 40) goTo(dx < 0 ? current + 1 : current - 1);
    }, { passive: true });
  }

  /* Kick off */
  slides[0].classList.add('active');
  rafId = requestAnimationFrame(animateProgress);
  resetTimer();
})();

// ── Latest GitHub release badge ──────────────────────────────────────────────
(function () {
  var label = document.getElementById('latest-release-label');
  var badge = document.getElementById('latest-release-badge');
  if (!label) return;

  fetch('https://api.github.com/repos/fabianoflorentino/stracectl/releases/latest', {
    headers: { Accept: 'application/vnd.github+json' }
  })
    .then(function (r) { return r.ok ? r.json() : Promise.reject(r.status); })
    .then(function (data) {
      var tag = data.tag_name || '';
      if (!tag) return;

      // Update badge
      label.textContent = tag + ' — Latest Release';
      if (badge && data.html_url) badge.href = data.html_url;

      // Update all go install snippets to use the pinned version
      var cmd = 'go install github.com/fabianoflorentino/stracectl@' + tag;
      ['hero-install-cmd', 'install-go-cmd'].forEach(function (id) {
        var el = document.getElementById(id);
        if (el) el.textContent = cmd;
      });
      ['hero-install-copy', 'install-go-copy'].forEach(function (id) {
        var el = document.getElementById(id);
        if (el) el.dataset.copy = cmd;
      });

      // Update docker pull snippet to use the pinned version
      var dockerCmd = 'docker pull fabianoflorentino/stracectl:' + tag;
      var dockerCmdEl = document.getElementById('install-docker-cmd');
      if (dockerCmdEl) dockerCmdEl.textContent = dockerCmd;
      var dockerCopyEl = document.getElementById('install-docker-copy');
      if (dockerCopyEl) dockerCopyEl.dataset.copy = dockerCmd;
    })
    .catch(function () { /* keep default text on error */ });
})();

/* ──────────────────────────────────────────────────────────
 * Back-to-top button
 * ──────────────────────────────────────────────────────────*/
(function () {
  'use strict';
  var btn = document.getElementById('back-to-top');
  if (!btn) return;

  var THRESHOLD = 300;

  window.addEventListener('scroll', function () {
    btn.classList.toggle('visible', window.scrollY >= THRESHOLD);
  }, { passive: true });

  btn.addEventListener('click', function () {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  });
})();
