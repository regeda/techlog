(function () {
  // simulated agent session on a timeline: every command is typed in the box at the bottom, sent up into
  // the transcript, and its output follows. The whole run is a pure function of time, so the bar can seek.
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
  var TARGET_MS = 90000, SESSION_PAUSE_MS = 3000;
  var TYPE_MS = 30, LEAD_MS = 350;
  var items = [], steps = [], total = 0;
  var stepEls = fig.querySelectorAll('.sh-step');

  function layout(slow) {
    TYPE_MS = 30 * slow; LEAD_MS = 350 * slow;
    var gap = 600 * slow, t = 400;
    items = []; steps = [];
    stepEls.forEach(function (st) {
      // a new session's divider shows at once, right after the last line; typing starts SESSION_PAUSE_MS later
      if (st.querySelector('.sh-rule')) t -= gap;
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
          it.at = t;
          t += l.classList.contains('sh-rule') ? SESSION_PAUSE_MS - LEAD_MS : slow * (l.classList.contains('sh-tool') ? 550 : l.classList.contains('sh-say') ? 450 : 300);
        }
        if (step.first < 0) step.first = it.at;
        items.push(it);
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
    });
    steps.forEach(function (s) {
      var on = pos >= s.first;
      if (s.el.classList.contains('sh-on') !== on) s.el.classList.toggle('sh-on', on);
    });
    text.textContent = typing;
    body.scrollTop = body.scrollHeight;
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
