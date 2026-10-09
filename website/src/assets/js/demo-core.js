(function () {
  // Pure stepper behind the hero demo: no DOM access, so it can be tested in isolation.
  function create(steps) {
    var i = 0;
    var last = steps.length - 1;
    function go(n) { i = Math.max(0, Math.min(last, n)); return i; }
    return {
      count: steps.length,
      index: function () { return i; },
      step: function () { return steps[i]; },
      go: go,
      next: function () { return go(i + 1); },
      prev: function () { return go(i - 1); },
      canNext: function () { return i < last; },
      canPrev: function () { return i > 0; },
      isLast: function () { return i === last; }
    };
  }
  window.TechlogDemo = { create: create };
})();
