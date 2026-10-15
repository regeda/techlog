(function () {
  // simulated demo on a timeline: three agent sessions on the left, the records they leave in Techlog on the right.
  // The agent pane shows one session at a time (a new session starts empty); the record cards only ever accumulate.
  // The whole run is a pure function of time, so the bar can seek.
  var fig = document.getElementById('shell');
  if (!fig) return;
  var body = fig.querySelector('.sh-body');
  var input = document.getElementById('sh-input');
  var text = document.getElementById('sh-text');
  var ctl = document.getElementById('sh-ctl');
  var playBtn = document.getElementById('sh-play');
  var seek = document.getElementById('sh-seek');
  var clock = document.getElementById('sh-time');
  // typing and output delays scale with one factor; the pause after a new-session divider is absolute.
  // The factor is solved so the whole run lasts TARGET_MS, pauses included.
  var TARGET_MS = 45000, SESSION_PAUSE_MS = 2000;
  // the session stays on screen this long after its record lands, so the card and the wipe are two beats
  var CARD_DWELL_MS = 1000;
  var TYPE_MS = 30, LEAD_MS = 350, OUT_TYPE_MS = 12, DIVIDER_TYPE_MS = 45;   // per character: commands, output lines, session dividers
  var items = [], steps = [], total = 0;
  var stepEls = fig.querySelectorAll('.sh-step');
  var cardEls = fig.querySelectorAll('.rec-card');
  var list = fig.querySelector('.rec-list');

  // the text nodes of an output line with their full text, so the line can be filled in character by character
  function textNodes(el) {
    if (el._nodes) return el._nodes;
    var out = [], w = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    while (w.nextNode()) out.push({ node: w.currentNode, full: w.currentNode.data });
    el._nodes = out;
    return out;
  }
  function setTyped(it, n) {
    var left = n;
    it.nodes.forEach(function (p) {
      var k = Math.max(0, Math.min(p.full.length, left));
      p.node.data = p.full.slice(0, k); left -= k;
    });
  }

  function layout(slow) {
    TYPE_MS = 30 * slow; LEAD_MS = 350 * slow; OUT_TYPE_MS = 12 * slow; DIVIDER_TYPE_MS = 45 * slow;
    var gap = 600 * slow, t = 400;
    items = []; steps = [];
    stepEls.forEach(function (st) {
      // a new session's divider shows at once, right after the last line; typing starts SESSION_PAUSE_MS later
      if (st.querySelector('.sh-rule')) t += CARD_DWELL_MS - gap;
      var step = { el: st, first: -1 };
      st.querySelectorAll('.sh-l').forEach(function (l) {
        var it = { el: l, step: step };
        if (l.classList.contains('sh-user')) {
          it.cmd = true;
          it.full = l.querySelector('.sh-typed').textContent.trim();
          it.start = t + LEAD_MS;
          it.send = it.start + it.full.length * TYPE_MS + 300 * slow;
          it.at = it.send;
          t = it.send + 450 * slow;
        } else {
          // output is typed too: the line appears empty at it.at and fills in over it.dur
          it.at = t;
          it.nodes = textNodes(l);
          it.chars = it.nodes.reduce(function (n, p) { return n + p.full.length; }, 0);
          it.cms = l.classList.contains('sh-rule') ? DIVIDER_TYPE_MS : OUT_TYPE_MS;
          it.dur = it.chars * it.cms;
          t += it.dur + (l.classList.contains('sh-rule') ? SESSION_PAUSE_MS - LEAD_MS : slow * (l.classList.contains('sh-tool') ? 550 : l.classList.contains('sh-say') ? 450 : 300));
        }
        if (step.first < 0) step.first = it.at;
        items.push(it);
        // the record card appears just after the line that says the event was recorded
        var rec = l.getAttribute('data-rec');
        if (rec && cardEls[Number(rec) - 1]) items.push({ el: cardEls[Number(rec) - 1], at: it.at + 300 * slow });
      });
      t += gap; steps.push(step);
    });
    return t;
  }
  var t1 = layout(1), t2 = layout(2);       // total time is linear in the factor
  var slope = t2 - t1, fixed = t1 - slope;
  total = layout(Math.max(0.5, (TARGET_MS - fixed) / slope));

  var pos = 0, playing = false, last = 0, raf = 0, wasPlaying = false;
  function fmt(ms) { var s = Math.round(ms / 1000); return Math.floor(s / 60) + ':' + ('0' + (s % 60)).slice(-2); }

  function render() {
    var typing = '';
    items.forEach(function (it) {
      var on = pos >= it.at;
      if (it.el.classList.contains('sh-on') !== on) it.el.classList.toggle('sh-on', on);
      if (it.cmd && pos >= it.start && pos < it.send) typing = it.full.slice(0, Math.floor((pos - it.start) / TYPE_MS));
      if (it.nodes) {
        var n = Math.max(0, Math.min(it.chars, Math.floor((pos - it.at) / it.cms)));
        if (it.shown !== n) { setTyped(it, n); it.shown = n; }
        var typingNow = on && n < it.chars;
        if (it.el.classList.contains('sh-typing') !== typingNow) it.el.classList.toggle('sh-typing', typingNow);
      }
    });
    var cur = -1;
    steps.forEach(function (s, i) { if (pos >= s.first) cur = i; });
    steps.forEach(function (s, i) {
      if (s.el.classList.contains('sh-on') !== (i <= cur)) s.el.classList.toggle('sh-on', i <= cur);
      if (s.el.classList.contains('sh-cur') !== (i === cur)) s.el.classList.toggle('sh-cur', i === cur);
    });
    text.textContent = typing;
    body.scrollTop = body.scrollHeight;
    list.scrollTop = list.scrollHeight;
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

  // starts idle: nothing is shown until the visitor presses the glowing button
  fig.classList.add('sh-js'); ctl.hidden = false; input.hidden = false;
  render(); label();
})();
