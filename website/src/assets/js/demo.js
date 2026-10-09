(function () {
  var root = document.getElementById('demo');
  var dataEl = document.getElementById('demo-data');
  if (!root || !dataEl || !window.TechlogDemo) return;
  var data;
  try { data = JSON.parse(dataEl.textContent); } catch (e) { return; }
  var demo = window.TechlogDemo.create(data.steps);

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  // ---- skeleton (built once)
  var bar = el('div', 'cs-bar');
  var title = el('div', 'cs-title');
  title.appendChild(el('strong', null, data.title));
  title.appendChild(el('span', null, data.subtitle));
  var dots = el('div', 'dm-dots');
  var dotBtns = data.steps.map(function (s, n) {
    var b = el('button', 'dm-dot');
    b.type = 'button';
    b.setAttribute('aria-label', 'Go to step ' + (n + 1) + ': ' + s.title);
    b.addEventListener('click', function () { demo.go(n); render(); });
    dots.appendChild(b);
    return b;
  });
  bar.appendChild(title);
  bar.appendChild(dots);
  bar.appendChild(el('span', 'badge badge-simulated', 'Simulated example'));

  var stage = el('div', 'dm-stage');
  stage.setAttribute('aria-live', 'polite');
  var stepTitle = el('h2', 'dm-title');
  var stepNote = el('p', 'dm-note');
  stage.appendChild(stepTitle);
  stage.appendChild(stepNote);

  var panels = el('div', 'dm-panels');
  function panel(label, cls) {
    var p = el('div', 'dm-panel ' + cls);
    p.appendChild(el('div', 'dm-label', label));
    var body = el('div', 'dm-lines');
    p.appendChild(body);
    panels.appendChild(p);
    return body;
  }
  var leftBody = panel(data.left_label, 'dm-old');
  var rightBody = panel(data.right_label, 'dm-new');

  var foot = el('div', 'dm-foot');
  var prevBtn = el('button', 'btn btn-grey dm-btn', '‹ Prev');
  prevBtn.type = 'button';
  var count = el('span', 'dm-count mono');
  var nextBtn = el('button', 'btn btn-primary dm-btn', 'Next ›');
  nextBtn.type = 'button';
  foot.appendChild(prevBtn);
  foot.appendChild(count);
  foot.appendChild(nextBtn);

  root.appendChild(bar);
  root.appendChild(stage);
  root.appendChild(panels);
  root.appendChild(foot);

  function fill(body, lines) {
    body.textContent = '';
    lines.forEach(function (l) { body.appendChild(el('div', 'dm-line dm-' + l.t, l.x)); });
  }

  function render() {
    var s = demo.step();
    stepTitle.textContent = s.title;
    stepNote.textContent = s.note;
    fill(leftBody, s.left.lines);
    fill(rightBody, s.right.lines);
    count.textContent = 'Step ' + (demo.index() + 1) + ' of ' + demo.count;
    prevBtn.disabled = !demo.canPrev();
    nextBtn.textContent = demo.isLast() ? 'Replay ↺' : 'Next ›';
    dotBtns.forEach(function (b, n) {
      if (n === demo.index()) b.setAttribute('aria-current', 'step'); else b.removeAttribute('aria-current');
    });
    root.classList.remove('is-ticking');
    void root.offsetWidth; // restart the line animation
    root.classList.add('is-ticking');
  }

  prevBtn.addEventListener('click', function () { demo.prev(); render(); });
  nextBtn.addEventListener('click', function () { if (demo.isLast()) demo.go(0); else demo.next(); render(); });
  root.addEventListener('keydown', function (e) {
    if (e.key === 'ArrowRight') { demo.next(); render(); e.preventDefault(); }
    else if (e.key === 'ArrowLeft') { demo.prev(); render(); e.preventDefault(); }
  });
  render();
})();
