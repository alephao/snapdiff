// snapdiff — interactivity for the index + diff pages.
(function () {
  function $(sel, root) { return (root || document).querySelector(sel); }
  function $$(sel, root) { return Array.from((root || document).querySelectorAll(sel)); }
  function inField(el) { return /input|textarea/i.test(el && el.tagName || ''); }

  // ---------- INDEX PAGE + DIFF SIDEBAR ----------

  // Group collapse (index .group + diff sidebar .sb-group)
  $$('.group-head, .sb-group-head').forEach(h => {
    h.addEventListener('click', () => h.parentElement.classList.toggle('is-collapsed'));
  });

  // "g" collapses/expands all groups on whichever page we're on
  document.addEventListener('keydown', e => {
    if (inField(e.target)) return;
    if (e.key === 'g') {
      const groups = $$('.group, .sb-group');
      if (groups.length === 0) return;
      const anyOpen = groups.some(g => !g.classList.contains('is-collapsed'));
      groups.forEach(g => g.classList.toggle('is-collapsed', anyOpen));
    }
  });

  // ---------- DIFF SIDEBAR (left rail) ----------

  // all / pending toggle: hide decided rows and any group that becomes empty.
  const sbList = $('.sb-list');
  const sbScopeAll     = $('.sb-filter button[data-scope="all"]');
  const sbScopePending = $('.sb-filter button[data-scope="pending"]');
  function setSidebarScope(scope) {
    if (!sbList) return;
    const pendingOnly = scope === 'pending';
    sbList.classList.toggle('is-pending-only', pendingOnly);
    if (sbScopeAll)     sbScopeAll.classList.toggle('is-on', !pendingOnly);
    if (sbScopePending) sbScopePending.classList.toggle('is-on',  pendingOnly);
    // CSS hides decided rows; here we hide groups that have no pending rows.
    $$('.sb-group').forEach(g => {
      const hasPending = !!g.querySelector('.sb-row:not(.is-approved):not(.is-rejected)');
      g.classList.toggle('is-empty-in-scope', pendingOnly && !hasPending);
    });
  }
  if (sbScopeAll)     sbScopeAll.addEventListener('click',     () => setSidebarScope('all'));
  if (sbScopePending) sbScopePending.addEventListener('click', () => setSidebarScope('pending'));

  // Scroll the current diff into view inside the sidebar on load.
  const sbCurrent = $('.sb-row.is-current');
  if (sbCurrent) sbCurrent.scrollIntoView({ block: 'nearest' });

  // ---------- DIFF PAGE ----------

  // Mode switcher (keyboard-driven; no visible mode bar)
  const panes = $$('.pane');
  function setMode(m) {
    panes.forEach(p => p.classList.toggle('is-active', p.dataset.mode === m));
  }

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

  // Click image to toggle zoom (fit ↔ 1×). stopPropagation so it doesn't
  // also trigger the toggle-mode lock handler bound on #toggleWrap.
  const stage = $('.stage');
  $$('.phone-frame img').forEach(img => {
    img.addEventListener('click', e => {
      e.stopPropagation();
      if (stage) stage.classList.toggle('is-zoomed');
    });
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

  // Verdict toggle. Approve auto-submits and advances to the next diff;
  // reject only updates visual state because a comment is required, so the
  // user finishes with ⌘↵ on the save button.
  const vApprove = $('#vApprove');
  const vReject  = $('#vReject');
  const vForm    = $('form.rb-verdict');
  const vRadioApprove = $('input[name="status"][value="approved"]');
  const vRadioReject  = $('input[name="status"][value="rejected"]');
  function pick(which) {
    if (vApprove) vApprove.classList.toggle('is-on', which === 'a');
    if (vReject)  vReject.classList.toggle('is-on', which === 'r');
    if (which === 'a' && vRadioApprove) vRadioApprove.checked = true;
    if (which === 'r' && vRadioReject)  vRadioReject.checked  = true;
  }
  function approveAndAdvance() {
    pick('a');
    if (vForm) vForm.submit();
  }
  if (vApprove) vApprove.addEventListener('click', e => { e.preventDefault(); approveAndAdvance(); });
  if (vReject)  vReject.addEventListener('click', e => { e.preventDefault(); pick('r'); });
  document.addEventListener('keydown', e => {
    if (inField(e.target)) return;
    if (e.key === 'a') approveAndAdvance();
    if (e.key === 'r') pick('r');
  });

  // j/k navigation (diff page)
  document.addEventListener('keydown', e => {
    if (inField(e.target)) return;
    if (e.key === 'j') { const n = $('.nav-next:not(.disabled)'); if (n) location.href = n.getAttribute('href'); }
    if (e.key === 'k') { const p = $('.nav-prev:not(.disabled)'); if (p) location.href = p.getAttribute('href'); }
  });
})();
