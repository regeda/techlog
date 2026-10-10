(function () {
  // Eight items from four systems, plus two agents. At progress 0 they are scattered and tangled by guessed
  // links, Claude only opens a pull request and Codex only writes the postmortem. The Techlog button starts
  // the change: the items fade into one looped lifecycle. The Techlog robot stays in the centre where it was,
  // and both agents connect to it. Each step is linked to exactly one agent, so their links never cross
  // or share a step, and Techlog itself is not connected to the steps, only to the agents.
  // Replay fades back to the scattered state.
  var root = document.getElementById('scatter');
  if (!root) return;
  var stage = root.querySelector('.sc-stage');
  var all = Array.prototype.slice.call(root.querySelectorAll('.sc-card'));
  var agents = Array.prototype.slice.call(root.querySelectorAll('.sc-agent')); // Claude on the left, Codex on the right
  var cards = all.filter(function (c) { return agents.indexOf(c) < 0; }); // the eight steps, in lifecycle order
  var OWNER = [0, 0, 1, 1, 1, 1, 0, 0]; // the one agent that controls each step: Claude 1, 2, 7, 8 and Codex 3 to 6
  var BEFORE = [{ agent: 0, step: 2, label: 'opens PR' }, { agent: 1, step: 6, label: 'writes postmortem' }];
  var svg = root.querySelector('.sc-lines');
  var caption = document.getElementById('sc-caption');
  var replay = document.getElementById('sc-replay');
  var go = document.getElementById('sc-go');
  var NS = 'http://www.w3.org/2000/svg';
  var CHAOS = [[0, 4], [1, 7], [2, 5], [3, 6], [0, 2], [1, 4], [5, 7], [3, 0], [6, 2], [4, 3]];
  var DURATION = 300; // every change, in either direction, takes 300 ms

  var pos = [];
  var progress = 0;
  var raf = 0;
  var chaosEls = [];
  var marks = [];
  var loopEls = [];
  var arrowEls = [];
  var spokeEls = [];
  var beforeEls = [];
  var hubLinks = [];

  function make(tag, cls) {
    var n = document.createElementNS(NS, tag);
    n.setAttribute('class', cls);
    svg.appendChild(n);
    return n;
  }
  CHAOS.forEach(function (pair, i) {
    chaosEls.push(make('path', 'sc-chaos'));
    if (i % 2 === 0) {
      var q = make('text', 'sc-q');
      q.textContent = '?';
      marks.push(q);
    } else marks.push(null);
  });
  cards.forEach(function () { loopEls.push(make('line', 'sc-loop')); });
  cards.forEach(function () { arrowEls.push(make('polygon', 'sc-arrow')); });
  cards.forEach(function (c, i) { spokeEls.push(make('line', 'sc-spoke sc-spoke-' + OWNER[i])); });
  agents.forEach(function (ag, i) { hubLinks.push(make('line', 'sc-hublink sc-hublink-' + i)); });
  BEFORE.forEach(function (b) {
    var l = make('line', 'sc-claude');
    var t = make('text', 'sc-claude-label');
    t.textContent = b.label;
    beforeEls.push({ line: l, label: t });
  });

  // the connected positions differ per screen size, so they come from the CSS
  function read() {
    pos = all.map(function (c) {
      var st = getComputedStyle(c);
      var n = function (k) { return parseFloat(st.getPropertyValue(k)); };
      return { sx: n('--sx'), sy: n('--sy'), sr: n('--sr'), cx: n('--cx'), cy: n('--cy') };
    });
  }
  function clamp(v) { return Math.max(0, Math.min(1, v)); }
  function smooth(a, b, v) { var t = clamp((v - a) / (b - a)); return t * t * (3 - 2 * t); }

  // Fade, not fly: the scattered layout fades out, then the lifecycle fades in at its own places.
  // At progress 0.5 every card is invisible, which is where the positions switch.
  function render(p) {
    progress = p;
    var ring = p >= 0.5;
    var fade = smooth(0, 1, Math.abs(1 - 2 * p));
    var w = stage.clientWidth;
    var h = stage.clientHeight;
    svg.setAttribute('viewBox', '0 0 ' + w + ' ' + h);
    var scat = [];
    var loopPts = [];
    pos.forEach(function (q, i) {
      var c = all[i];
      c.style.left = (ring ? q.cx : q.sx) + '%';
      c.style.top = (ring ? q.cy : q.sy) + '%';
      c.style.transform = 'translate(-50%, -50%) rotate(' + (ring ? 0 : q.sr) + 'deg)';
      c.style.opacity = fade.toFixed(3);
      scat.push([q.sx * w / 100, q.sy * h / 100]);
      loopPts.push([q.cx * w / 100, q.cy * h / 100]);
    });

    var tangle = clamp(1 - p / 0.45);
    CHAOS.forEach(function (pair, i) {
      var a = scat[pair[0]];
      var b = scat[pair[1]];
      var mx = (a[0] + b[0]) / 2;
      var my = (a[1] + b[1]) / 2;
      var dx = b[0] - a[0];
      var dy = b[1] - a[1];
      var k = (i % 2 ? 0.28 : -0.28);
      var cx = mx - dy * k;
      var cy = my + dx * k;
      chaosEls[i].setAttribute('d', 'M' + a[0] + ' ' + a[1] + ' Q' + cx + ' ' + cy + ' ' + b[0] + ' ' + b[1]);
      chaosEls[i].style.opacity = tangle * 0.85;
      if (marks[i]) {
        marks[i].setAttribute('x', (a[0] + 2 * cx + b[0]) / 4);
        marks[i].setAttribute('y', (a[1] + 2 * cy + b[1]) / 4);
        marks[i].style.opacity = tangle;
      }
    });

    // before: each agent does one thing, linked to one card
    BEFORE.forEach(function (bf, i) {
      var from = scat[cards.length + bf.agent];
      var to = scat[bf.step];
      var el = beforeEls[i];
      el.line.setAttribute('x1', from[0]); el.line.setAttribute('y1', from[1]);
      el.line.setAttribute('x2', to[0]); el.line.setAttribute('y2', to[1]);
      el.line.style.opacity = tangle;
      el.label.setAttribute('x', (from[0] + to[0]) / 2);
      el.label.setAttribute('y', (from[1] + to[1]) / 2 - 10);
      el.label.style.opacity = tangle;
    });

    var loop = smooth(0.55, 1, p);
    // the Techlog robot is in the centre in both states; only after the change do the agents connect to it
    var centre = [w / 2, h / 2];
    agents.forEach(function (ag, i) {
      var a = loopPts[cards.length + i];
      hubLinks[i].setAttribute('x1', a[0]); hubLinks[i].setAttribute('y1', a[1]);
      hubLinks[i].setAttribute('x2', centre[0]); hubLinks[i].setAttribute('y2', centre[1]);
      hubLinks[i].style.opacity = smooth(0.7, 1, p);
    });
    var ringPts = loopPts.slice(0, cards.length);
    ringPts.forEach(function (a, i) {
      var b = ringPts[(i + 1) % ringPts.length];
      var l = loopEls[i];
      l.setAttribute('x1', a[0]); l.setAttribute('y1', a[1]);
      l.setAttribute('x2', b[0]); l.setAttribute('y2', b[1]);
      l.style.opacity = loop;
      var mx = (a[0] + b[0]) / 2;
      var my = (a[1] + b[1]) / 2;
      var len = Math.hypot(b[0] - a[0], b[1] - a[1]) || 1;
      var ux = (b[0] - a[0]) / len;
      var uy = (b[1] - a[1]) / len;
      var nx = -uy;
      var ny = ux;
      arrowEls[i].setAttribute('points',
        (mx + ux * 9) + ',' + (my + uy * 9) + ' ' +
        (mx - ux * 6 + nx * 6) + ',' + (my - uy * 6 + ny * 6) + ' ' +
        (mx - ux * 6 - nx * 6) + ',' + (my - uy * 6 - ny * 6));
      arrowEls[i].style.opacity = loop;

      // each step is linked to exactly one agent
      var hub = loopPts[cards.length + OWNER[i]];
      spokeEls[i].setAttribute('x1', hub[0]); spokeEls[i].setAttribute('y1', hub[1]);
      spokeEls[i].setAttribute('x2', a[0]); spokeEls[i].setAttribute('y2', a[1]);
      spokeEls[i].style.opacity = smooth(0.7, 1, p);
    });
    agents.forEach(function (ag) { ag.querySelector('.sc-sub').textContent = ag.getAttribute(ring ? 'data-after' : 'data-before'); });

    stage.style.setProperty('--tag', smooth(0.75, 1, p).toFixed(3));
    // the whole pane takes a light green background as the lifecycle fades in, and loses it as it fades out
    root.style.setProperty('--tint', smooth(0.5, 1, p).toFixed(3));
    var phase = p >= 0.9 ? 'lifecycle' : 'scattered';
    root.setAttribute('data-phase', phase);
    caption.textContent = phase === 'lifecycle' ? 'Techlog: one lifecycle, looped' : 'Legacy Changelog';
    replay.hidden = p < 0.9;
    // the robot never moves: it only turns from grey and unconnected to green and connected
    go.classList.toggle('is-on', ring);
    go.disabled = ring;
  }

  function stop() {
    if (raf) cancelAnimationFrame(raf);
    raf = 0;
  }

  // animateTo moves the progress to a target, so every change can be played forward or back
  function animateTo(target, ms) {
    stop();
    // a fade is not motion, so it plays even with reduced motion; only the looping arrows stop (see the CSS)
    if (ms <= 0) { render(target); return; }
    var from = progress;
    var t0 = null;
    function frame(t) {
      if (t0 === null) t0 = t;
      var k = clamp((t - t0) / ms);
      render(from + (target - from) * k);
      if (k < 1) raf = requestAnimationFrame(frame); else raf = 0;
    }
    raf = requestAnimationFrame(frame);
  }

  // the button on the legacy pane starts the change, and Replay (top right) goes back to it
  go.addEventListener('click', function () { animateTo(1, DURATION); });
  replay.addEventListener('click', function () { animateTo(0, DURATION); });
  window.addEventListener('resize', function () { read(); render(progress); });

  read();
  render(0);
})();
