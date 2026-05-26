// snapdiff — interactivity for the index + diff pages.
(function () {
  function $(sel, root) { return (root || document).querySelector(sel); }
  function $$(sel, root) { return Array.from((root || document).querySelectorAll(sel)); }
  function inField(el) { return /input|textarea/i.test(el && el.tagName || ''); }

  // ---------- INDEX PAGE ----------

  // Group collapse
  $$('.group-head').forEach(h => {
    h.addEventListener('click', () => h.parentElement.classList.toggle('is-collapsed'));
  });

  // "g" collapses/expands all groups
  document.addEventListener('keydown', e => {
    if (inField(e.target)) return;
    if (e.key === 'g') {
      const anyOpen = !!$('.group:not(.is-collapsed)');
      $$('.group').forEach(g => g.classList.toggle('is-collapsed', anyOpen));
    }
  });

  // ---------- DIFF PAGE ----------

  // Mode switcher
  const modeBtns = $$('.mode-btn');
  const panes = $$('.pane');
  function setMode(m) {
    modeBtns.forEach(b => b.classList.toggle('is-active', b.dataset.mode === m));
    panes.forEach(p => p.classList.toggle('is-active', p.dataset.mode === m));
  }
  modeBtns.forEach(b => b.addEventListener('click', () => setMode(b.dataset.mode)));

  // Keyboard: 1-5 switch modes, esc back to index
  document.addEventListener('keydown', e => {
    if (inField(e.target)) return;
    const map = { '1': 'side', '2': 'swipe', '3': 'toggle', '4': 'pixel', '5': 'onion' };
    if (map[e.key]) setMode(map[e.key]);
    if (e.key === 'Escape') {
      const back = $('.back');
      if (back) location.href = back.getAttribute('href');
    }
  });

  // Swipe slider
  const swipeSlider = $('#swipeSlider');
  const swipeAbove  = $('#swipeAbove');
  const swipeSplit  = $('#swipeSplit');
  if (swipeSlider && swipeAbove && swipeSplit) {
    function updateSwipe() {
      const v = swipeSlider.value;
      swipeAbove.style.clipPath = `inset(0 ${100 - v}% 0 0)`;
      swipeSplit.style.left = v + '%';
    }
    swipeSlider.addEventListener('input', updateSwipe);
    updateSwipe();
  }

  // Toggle: hold T to flip, click to lock
  const toggleWrap = $('#toggleWrap');
  const toggleLabel = $('#toggleLabel');
  if (toggleWrap) {
    let locked = false;
    function show(current) {
      toggleWrap.classList.toggle('show-current', current);
      if (toggleLabel) toggleLabel.textContent = current ? 'current' : 'baseline';
    }
    show(false);
    toggleWrap.addEventListener('click', () => { locked = !locked; show(locked); });
    document.addEventListener('keydown', e => {
      if ((e.key === 't' || e.key === 'T') && !locked && !inField(e.target)) show(true);
    });
    document.addEventListener('keyup', e => {
      if ((e.key === 't' || e.key === 'T') && !locked && !inField(e.target)) show(false);
    });
  }

  // Onion slider
  const onionSlider = $('#onionSlider');
  const onionAbove  = $('#onionAbove');
  if (onionSlider && onionAbove) {
    onionSlider.addEventListener('input', () => {
      onionAbove.style.opacity = (onionSlider.value / 100).toFixed(2);
    });
  }

  // Verdict toggle (visual state only; the form actually posts)
  const vApprove = $('#vApprove');
  const vReject  = $('#vReject');
  const vRadioApprove = $('input[name="status"][value="approved"]');
  const vRadioReject  = $('input[name="status"][value="rejected"]');
  function pick(which) {
    if (vApprove) vApprove.classList.toggle('is-on', which === 'a');
    if (vReject)  vReject.classList.toggle('is-on', which === 'r');
    if (which === 'a' && vRadioApprove) vRadioApprove.checked = true;
    if (which === 'r' && vRadioReject)  vRadioReject.checked  = true;
  }
  if (vApprove) vApprove.addEventListener('click', e => { e.preventDefault(); pick('a'); });
  if (vReject)  vReject.addEventListener('click', e => { e.preventDefault(); pick('r'); });
  document.addEventListener('keydown', e => {
    if (inField(e.target)) return;
    if (e.key === 'a') pick('a');
    if (e.key === 'r') pick('r');
  });

  // j/k navigation (diff page)
  document.addEventListener('keydown', e => {
    if (inField(e.target)) return;
    if (e.key === 'j') { const n = $('.nav-next:not(.disabled)'); if (n) location.href = n.getAttribute('href'); }
    if (e.key === 'k') { const p = $('.nav-prev:not(.disabled)'); if (p) location.href = p.getAttribute('href'); }
  });
})();
