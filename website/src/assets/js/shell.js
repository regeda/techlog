(function () {
  // simulated demo on a timeline: one question, then the answer of an agent without Techlog and the answer of an agent with it.
  // The question, the answers and the event rows are played in document order: text is typed, rows appear one after another.
  // The whole run is a pure function of time, so the bar can seek. Nothing is removed or disabled: every row can be opened at any time.
  var fig = document.getElementById('shell');
  if (!fig) return;
  var ctl = document.getElementById('sh-ctl');
  var playBtn = document.getElementById('sh-play');
  var seek = document.getElementById('sh-seek');
  var clock = document.getElementById('sh-time');
  // typing and gaps scale with one factor, solved so the whole run lasts TARGET_MS, the fixed pauses included
  var TARGET_MS = 45000;
  // fixed pauses (not scaled): after the question and after the left pane, so the visitor can read
  var PAUSE_MS = 3000;
  var LEAD_MS = 400, TYPE_MS = 18, BLOCK_GAP_MS = 350, EVENT_GAP_MS = 900, TAIL_MS = 600;
  var els = fig.querySelectorAll('.cmp-t, .cmp-event');
  var items = [], total = 0;

  // typed text is split once into the part typed so far and the rest: the rest stays in the layout (hidden),
  // so wrapping and height never change while it types
  function prepare(el) {
    if (el._it) return el._it;
    var full = el.textContent, on = document.createElement('span'), off = document.createElement('span');
    on.className = 'cmp-on'; off.className = 'cmp-off';
    el.textContent = ''; el.appendChild(on); el.appendChild(off); off.textContent = full;
    el._it = { on: on, off: off, full: full, chars: full.length };
    return el._it;
  }

  function layout(slow) {
    var t = LEAD_MS;
    items = [];
    els.forEach(function (el, i) {
      var next = els[i + 1], group = el.closest('.cmp-q, .cmp-without');
      var pause = group && !(next && group.contains(next)) ? PAUSE_MS : 0;
      if (el.classList.contains('cmp-event')) {
        items.push({ el: el, at: t });
        t += EVENT_GAP_MS * slow;
        return;
      }
      var p = prepare(el), it = { el: el, on: p.on, off: p.off, full: p.full, chars: p.chars, at: t, cms: TYPE_MS * slow };
      items.push(it);
      t += p.chars * it.cms + BLOCK_GAP_MS * slow + pause;
    });
    return t + TAIL_MS * slow;
  }
  var t1 = layout(1), t2 = layout(2);       // total time is linear in the factor
  var slope = t2 - t1, fixed = t1 - slope;
  total = layout(Math.max(0.5, (TARGET_MS - fixed) / slope));

  var pos = 0, playing = false, last = 0, raf = 0, wasPlaying = false;
  function fmt(ms) { var s = Math.round(ms / 1000); return Math.floor(s / 60) + ':' + ('0' + (s % 60)).slice(-2); }

  function render() {
    items.forEach(function (it) {
      var on = pos >= it.at;
      if (it.el.classList.contains('sh-on') !== on) it.el.classList.toggle('sh-on', on);
      if (it.on) {
        var n = Math.max(0, Math.min(it.chars, Math.floor((pos - it.at) / it.cms)));
        if (it.shown !== n) { it.on.textContent = it.full.slice(0, n); it.off.textContent = it.full.slice(n); it.shown = n; }
        var typingNow = on && n < it.chars;
        if (it.el.classList.contains('sh-typing') !== typingNow) it.el.classList.toggle('sh-typing', typingNow);
      }
    });
    seek.value = String(Math.round(pos / total * 1000));
    clock.textContent = fmt(pos) + ' / ' + fmt(total);
    seek.setAttribute('aria-valuetext', fmt(pos) + ' of ' + fmt(total));
  }
  function label() {
    var atEnd = pos >= total;
    var idle = !playing && pos === 0;
    fig.classList.toggle('sh-idle', idle);
    playBtn.textContent = playing ? '❚❚' : idle ? '▶ Run simulation' : atEnd ? '↺' : '▶';
    playBtn.setAttribute('aria-label', playing ? 'Pause' : idle ? 'Run simulation' : atEnd ? 'Replay' : 'Play');
  }
  function tick(now) {
    if (!playing) return;
    pos = Math.min(total, pos + (now - last)); last = now;
    render();
    if (pos >= total) { playing = false; label(); return; }
    raf = requestAnimationFrame(tick);
  }
  function play() {
    if (pos >= total) pos = 0;
    playing = true; label(); last = performance.now(); raf = requestAnimationFrame(tick);
  }
  function pause() { playing = false; cancelAnimationFrame(raf); label(); }

  playBtn.addEventListener('click', function () { if (playing) pause(); else play(); });
  seek.addEventListener('input', function () {
    if (playing) { wasPlaying = true; pause(); }
    pos = Number(seek.value) / 1000 * total; render(); label();
  });
  seek.addEventListener('change', function () { if (wasPlaying) { wasPlaying = false; play(); } });

  // starts idle: nothing is typed until the visitor presses the glowing button
  fig.classList.add('sh-js'); ctl.hidden = false;
  render(); label();
})();
