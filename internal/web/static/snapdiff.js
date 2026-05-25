// snapdiff — small vanilla-JS bindings for the 5 diff modes.

(function () {
	function $(sel, root) { return (root || document).querySelector(sel); }
	function $$(sel, root) { return Array.from((root || document).querySelectorAll(sel)); }

	function initModeSwitcher() {
		var switcher = $(".mode-switcher");
		if (!switcher) return;
		var buttons = $$(".mode-btn", switcher);
		var panes   = $$(".mode-pane");

		function show(mode) {
			buttons.forEach(function (b) { b.classList.toggle("active", b.dataset.mode === mode); });
			panes.forEach(function (p) { p.hidden = p.dataset.mode !== mode; });
		}

		buttons.forEach(function (b) {
			b.addEventListener("click", function () { show(b.dataset.mode); });
		});
	}

	function initSwipe() {
		var slider = $(".swipe-slider");
		if (!slider) return;
		var top = $(".swipe-baseline");
		function update() {
			var v = slider.value;
			top.style.clipPath = "inset(0 " + (100 - v) + "% 0 0)";
		}
		slider.addEventListener("input", update);
		update();
	}

	function initToggle() {
		var pane = $(".mode-toggle");
		if (!pane) return;
		var base = $(".toggle-baseline", pane);
		var curr = $(".toggle-current", pane);
		var showCurrent = false;
		function set(c) {
			showCurrent = c;
			base.hidden = c;
			curr.hidden = !c;
		}
		pane.addEventListener("click", function () { set(!showCurrent); });
		document.addEventListener("keydown", function (e) {
			if (pane.hidden) return;
			if (e.key === "t" || e.key === "T") set(true);
		});
		document.addEventListener("keyup", function (e) {
			if (pane.hidden) return;
			if (e.key === "t" || e.key === "T") set(false);
		});
	}

	function initOnion() {
		var slider = $(".onion-slider");
		if (!slider) return;
		var top = $(".onion-current");
		function update() {
			top.style.opacity = (slider.value / 100).toFixed(2);
		}
		slider.addEventListener("input", update);
		update();
	}

	document.addEventListener("DOMContentLoaded", function () {
		initModeSwitcher();
		initSwipe();
		initToggle();
		initOnion();
	});
})();
