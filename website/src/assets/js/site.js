(function () {
  // header shrinks smoothly after a little scrolling (hysteresis avoids flicker at the edge)
  var rootEl = document.documentElement;
  function onScroll() {
    var y = window.pageYOffset || 0;
    var on = rootEl.classList.contains('scrolled');
    if (!on && y > 40) rootEl.classList.add('scrolled');
    else if (on && y < 12) rootEl.classList.remove('scrolled');
  }
  window.addEventListener('scroll', onScroll, { passive: true });
  onScroll();

  // mobile nav
  var toggle = document.querySelector('.nav-toggle');
  var nav = document.getElementById('site-nav');
  if (toggle && nav) {
    toggle.addEventListener('click', function () {
      var open = nav.classList.toggle('open');
      toggle.setAttribute('aria-expanded', String(open));
    });
  }

  // docs menu on small screens
  var dToggle = document.querySelector('.docs-nav-toggle');
  var dList = document.getElementById('docs-nav-list');
  if (dToggle && dList) {
    dToggle.addEventListener('click', function () {
      var open = dList.classList.toggle('open');
      dToggle.setAttribute('aria-expanded', String(open));
    });
  }

  // copy buttons
  document.querySelectorAll('.codeblock .copy').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var code = btn.parentElement.querySelector('code');
      var text = code ? code.textContent : '';
      var done = function (label) { btn.textContent = label; setTimeout(function () { btn.textContent = 'Copy'; }, 1500); };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(function () { done('Copied'); }, function () { done('Press ⌘/Ctrl+C'); });
      } else { done('Press ⌘/Ctrl+C'); }
    });
  });

  // ARIA tabs with arrow-key navigation
  document.querySelectorAll('[data-tabs]').forEach(function (root) {
    var tabs = Array.prototype.slice.call(root.querySelectorAll('[role="tab"]'));
    function select(tab, focus) {
      tabs.forEach(function (t) {
        var on = t === tab;
        t.setAttribute('aria-selected', String(on));
        t.tabIndex = on ? 0 : -1;
        var panel = document.getElementById(t.getAttribute('aria-controls'));
        if (panel) panel.hidden = !on;
      });
      if (focus) tab.focus();
    }
    tabs.forEach(function (tab, i) {
      tab.addEventListener('click', function () { select(tab, false); });
      tab.addEventListener('keydown', function (e) {
        var n = null;
        if (e.key === 'ArrowRight') n = tabs[(i + 1) % tabs.length];
        else if (e.key === 'ArrowLeft') n = tabs[(i - 1 + tabs.length) % tabs.length];
        else if (e.key === 'Home') n = tabs[0];
        else if (e.key === 'End') n = tabs[tabs.length - 1];
        if (n) { e.preventDefault(); select(n, true); }
      });
    });
    var initial = tabs.filter(function (t) { return t.getAttribute('aria-selected') === 'true'; })[0] || tabs[0];
    if (initial) select(initial, false);
  });

  // Mermaid: loaded by the browser, only when a diagram exists. On any failure the
  // <pre class="mermaid"> source text simply stays visible.
  var diagrams = document.querySelectorAll('pre.mermaid');
  if (diagrams.length) {
    import('https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs').then(function (m) {
      var mermaid = m.default;
      mermaid.initialize({ startOnLoad: false, theme: 'dark', themeVariables: { fontSize: '16px' }, securityLevel: 'strict', fontFamily: 'ui-monospace, Menlo, monospace' });
      return mermaid.run({ nodes: diagrams });
    }).catch(function () { /* keep source text */ });
  }
})();
