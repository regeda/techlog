(function () {
  // simulated agent session on a timeline: a question, an answer without Techlog, /techlog:welcome, a better answer.
  // Prompts are typed in the input at the bottom and then sent into the log; answers are typed in the log; rows appear one after another.
  // The whole run is a pure function of time, so the bar can seek. Nothing is removed or disabled: every row can be opened at any time.
  var fig = document.getElementById('shell');
  if (!fig) return;
  var ctl = document.getElementById('sh-ctl');
  var playBtn = document.getElementById('sh-play');
  var seek = document.getElementById('sh-seek');
  var clock = document.getElementById('sh-time');
  var input = document.getElementById('cmp-input');
  var inputText = document.getElementById('cmp-input-text');
  var ghost = document.getElementById('cmp-input-ghost');
  // typing and gaps scale with one factor, solved so the whole run lasts TARGET_MS, the fixed pauses included
  var TARGET_MS = 45000;
  // fixed pauses (not scaled): after the first prompt and after the first answer, so the visitor can read
  var PAUSE_MS = 3000, SEND_MS = 300;
  var LEAD_MS = 400, TYPE_MS = 18, BLOCK_GAP_MS = 350, EVENT_GAP_MS = 900, TAIL_MS = 600;
  var log = fig.querySelector('.cmp-log');
  var els = fig.querySelectorAll('.cmp-t, .cmp-event');
  var items = [], groups = [], total = 0;

  // typed text is split once into the part typed so far and the rest (not shown yet)
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
    els.forEach(function (el) {
      var pause = el.hasAttribute('data-pause') ? PAUSE_MS : 0;
      if (el.classList.contains('cmp-event')) {
        items.push({ el: el, at: t });
        t += EVENT_GAP_MS * slow;
        return;
      }
      if (el.classList.contains('cmp-send')) {
        // a prompt: typed in the input first, then it lands in the log whole
        var full = el.textContent, cms = TYPE_MS * slow, sendAt = t + full.length * cms + SEND_MS * slow;
        items.push({ el: el, send: true, full: full, chars: full.length, at: t, cms: cms, sendAt: sendAt });
        t = sendAt + BLOCK_GAP_MS * slow + pause;
        return;
      }
      var p = prepare(el), it = { el: el, on: p.on, off: p.off, full: p.full, chars: p.chars, at: t, cms: TYPE_MS * slow };
      items.push(it);
      t += p.chars * it.cms + BLOCK_GAP_MS * slow + pause;
    });
    // a message, a list or the event line shows when its first item does
    groups = [];
    items.forEach(function (it) {
      var shows = it.send ? it.sendAt : it.at;
      for (var g = it.el.closest('.cmp-msg, .cmp-list, .cmp-events'); g; g = g.parentElement && g.parentElement.closest('.cmp-msg, .cmp-list, .cmp-events')) {
        var known = groups.filter(function (x) { return x.el === g; })[0];
        if (!known) groups.push({ el: g, at: shows }); else if (shows < known.at) known.at = shows;
      }
    });
    return t + TAIL_MS * slow;
  }
  // the input reserves the room of the longest prompt, so it never grows while a prompt is typed
  fig.querySelectorAll('.cmp-send').forEach(function (el) { if (el.textContent.length > ghost.textContent.length) ghost.textContent = el.textContent; });
  var t1 = layout(1), t2 = layout(2);       // total time is linear in the factor
  var slope = t2 - t1, fixed = t1 - slope;
  total = layout(Math.max(0.5, (TARGET_MS - fixed) / slope));

  var pos = 0, playing = false, last = 0, raf = 0, wasPlaying = false;
  function fmt(ms) { var s = Math.round(ms / 1000); return Math.floor(s / 60) + ':' + ('0' + (s % 60)).slice(-2); }

  function render() {
    var stick = log.scrollHeight - log.scrollTop - log.clientHeight < 40;   // follow the output unless the visitor scrolled up
    var typed = '', typingIn = false;
    items.forEach(function (it) {
      if (it.send) {
        var sent = pos >= it.sendAt;
        if (it.el.classList.contains('sh-on') !== sent) it.el.classList.toggle('sh-on', sent);
        if (pos >= it.at && !sent) { typed = it.full.slice(0, Math.min(it.chars, Math.floor((pos - it.at) / it.cms))); typingIn = true; }
        return;
      }
      var on = pos >= it.at;
      if (it.el.classList.contains('sh-on') !== on) it.el.classList.toggle('sh-on', on);
      if (it.on) {
        var n = Math.max(0, Math.min(it.chars, Math.floor((pos - it.at) / it.cms)));
        if (it.shown !== n) { it.on.textContent = it.full.slice(0, n); it.off.textContent = it.full.slice(n); it.shown = n; }
        var typingNow = on && n < it.chars;
        if (it.el.classList.contains('sh-typing') !== typingNow) it.el.classList.toggle('sh-typing', typingNow);
      }
    });
    groups.forEach(function (g) {
      var on = pos >= g.at;
      if (g.el.classList.contains('sh-on') !== on) g.el.classList.toggle('sh-on', on);
    });
    if (stick) log.scrollTop = log.scrollHeight;
    if (inputText.textContent !== typed) inputText.textContent = typed;
    if (input.classList.contains('sh-typing') !== typingIn) input.classList.toggle('sh-typing', typingIn);
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
  fig.classList.add('sh-js'); ctl.hidden = false; input.hidden = false;
  render(); label();
})();
