(function () {
  function norm(s) { return String(s == null ? '' : s).toLowerCase(); }

  // Plain substring matching only (no RegExp), so user input can never throw.
  function search(index, query, limit) {
    if (!Array.isArray(index)) return [];
    var terms = norm(query).split(/\s+/).filter(Boolean);
    if (!terms.length) return [];
    var scored = [];
    index.forEach(function (doc) {
      var title = norm(doc.title);
      var heads = norm((doc.headings || []).join(' '));
      var text = norm(doc.text);
      var score = 0;
      for (var i = 0; i < terms.length; i++) {
        var s = 0;
        if (title.indexOf(terms[i]) !== -1) s += 5;
        if (heads.indexOf(terms[i]) !== -1) s += 3;
        if (text.indexOf(terms[i]) !== -1) s += 1;
        if (!s) return;
        score += s;
      }
      scored.push({ doc: doc, score: score });
    });
    scored.sort(function (a, b) {
      return b.score - a.score || String(a.doc.title).localeCompare(String(b.doc.title));
    });
    return scored.slice(0, limit || 8).map(function (x) { return x.doc; });
  }

  window.TechlogSearch = { search: search };

  function init() {
    var input = document.getElementById('doc-search');
    var list = document.getElementById('doc-search-results');
    if (!input || !list || !window.TECHLOG_SEARCH) return;
    var root = document.documentElement.getAttribute('data-root') || '';
    input.addEventListener('input', function () {
      var results = search(window.TECHLOG_SEARCH, input.value, 8);
      list.textContent = '';
      results.forEach(function (doc) {
        var li = document.createElement('li');
        var a = document.createElement('a');
        a.href = root + doc.url;
        a.textContent = doc.title;
        li.appendChild(a);
        list.appendChild(li);
      });
      if (input.value.trim() && !results.length) {
        var li = document.createElement('li');
        li.className = 'muted';
        li.textContent = 'No results';
        list.appendChild(li);
      }
    });
  }
  if (document.addEventListener) document.addEventListener('DOMContentLoaded', init);
})();
