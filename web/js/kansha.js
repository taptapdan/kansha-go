// kansha.js: theme toggle, mobile navigation, copy buttons, search and the
// "On this page" highlight. No dependencies.
(function () {
  "use strict";

  var root = document.documentElement;
  var siteRoot = document.body.dataset.root || "";
  var $ = function (sel) { return document.querySelector(sel); };
  var $$ = function (sel) { return Array.prototype.slice.call(document.querySelectorAll(sel)); };

  // Theme toggle. Stored per browser; falls back to the OS setting.
  $$("[data-theme-toggle]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var dark = root.dataset.theme
        ? root.dataset.theme === "dark"
        : matchMedia("(prefers-color-scheme: dark)").matches;
      root.dataset.theme = dark ? "light" : "dark";
      try { localStorage.setItem("kansha-theme", root.dataset.theme); } catch (e) {}
    });
  });

  // Mobile navigation drawer.
  var drawer = $("[data-drawer]");
  $$("[data-drawer-open]").forEach(function (b) { b.addEventListener("click", function () { drawer.hidden = false; }); });
  $$("[data-drawer-close]").forEach(function (b) { b.addEventListener("click", function () { drawer.hidden = true; }); });
  if (drawer) drawer.addEventListener("click", function (e) { if (e.target.closest("a")) drawer.hidden = true; });

  // Copy buttons.
  $$("[data-copy]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var done = function () {
        var label = btn.textContent;
        btn.textContent = "Copied";
        setTimeout(function () { btn.textContent = label; }, 1500);
      };
      if (navigator.clipboard) navigator.clipboard.writeText(btn.dataset.copy).then(done, function () {});
    });
  });

  // Search. The index is loaded the first time the dialog opens.
  var dialog = $("[data-search]");
  var input = $("#kansha-search");
  var list = $("[data-search-results]");
  var index = null;
  var results = [];
  var selected = 0;

  function openSearch() {
    dialog.hidden = false;
    input.value = "";
    input.focus();
    if (!index) {
      fetch(siteRoot + "search.json")
        .then(function (r) { return r.json(); })
        .then(function (data) { index = data; update(); })
        .catch(function () { index = []; update(); });
    } else {
      update();
    }
  }
  function closeSearch() { dialog.hidden = true; }

  function score(item, q) {
    var name = item.name.toLowerCase();
    var short = name.slice(name.lastIndexOf(".") + 1);
    if (short === q || name === q) return 100;
    if (short.indexOf(q) === 0) return 80;
    if (name.indexOf(q) === 0) return 70;
    if (name.indexOf(q) >= 0) return 50;
    if (item.pkg.toLowerCase().indexOf(q) >= 0) return 20;
    if ((item.summary || "").toLowerCase().indexOf(q) >= 0) return 10;
    return 0;
  }

  function update() {
    if (!index) return;
    var q = input.value.trim().toLowerCase();
    results = index
      .map(function (it) { return { it: it, s: q ? score(it, q) : (it.kind === "package" ? 1 : 0) }; })
      .filter(function (r) { return r.s > 0; })
      .sort(function (a, b) { return b.s - a.s || a.it.name.length - b.it.name.length; })
      .slice(0, 30)
      .map(function (r) { return r.it; });
    selected = 0;
    render();
  }

  function render() {
    list.innerHTML = "";
    if (!results.length) {
      var empty = document.createElement("li");
      empty.className = "px-3 py-6 text-center text-sm text-faint";
      empty.textContent = "No matches";
      list.appendChild(empty);
      return;
    }
    results.forEach(function (it, i) {
      var li = document.createElement("li");
      var a = document.createElement("a");
      a.href = siteRoot + it.url;
      a.className = "flex items-baseline gap-3 rounded-lg px-3 py-2 text-sm " + (i === selected ? "bg-accentsoft" : "hover:bg-panel");
      var kind = document.createElement("span");
      kind.className = "w-14 shrink-0 text-[11px] uppercase tracking-wide text-faint";
      kind.textContent = it.kind;
      var name = document.createElement("span");
      name.className = "shrink-0 font-mono " + (i === selected ? "text-accent" : "text-fg");
      name.textContent = it.name;
      var meta = document.createElement("span");
      meta.className = "min-w-0 truncate text-faint";
      meta.textContent = it.kind === "package" ? (it.summary || it.pkg) : it.pkg;
      a.append(kind, name, meta);
      a.addEventListener("click", closeSearch);
      li.appendChild(a);
      list.appendChild(li);
    });
  }

  if (dialog) {
    $$("[data-search-open]").forEach(function (b) { b.addEventListener("click", openSearch); });
    $$("[data-search-close]").forEach(function (b) { b.addEventListener("click", closeSearch); });
    input.addEventListener("input", update);
    input.addEventListener("keydown", function (e) {
      if (e.key === "ArrowDown") { selected = Math.min(selected + 1, results.length - 1); render(); e.preventDefault(); }
      else if (e.key === "ArrowUp") { selected = Math.max(selected - 1, 0); render(); e.preventDefault(); }
      else if (e.key === "Enter" && results[selected]) { location.href = siteRoot + results[selected].url; closeSearch(); }
    });
    document.addEventListener("keydown", function (e) {
      var typing = /INPUT|TEXTAREA|SELECT/.test(document.activeElement.tagName);
      if ((e.key === "k" && (e.metaKey || e.ctrlKey)) || (e.key === "/" && !typing)) {
        e.preventDefault();
        dialog.hidden ? openSearch() : closeSearch();
      } else if (e.key === "Escape" && !dialog.hidden) {
        closeSearch();
      }
    });
  }

  // Highlight the "On this page" entry for the section in view.
  // The active entry is the last one whose section starts above the header.
  var links = $$("[data-toc]");
  if (links.length) {
    var targets = links.map(function (l) { return document.getElementById(l.dataset.toc); });
    var ticking = false;
    var spy = function () {
      ticking = false;
      var active = links[0];
      for (var i = 0; i < targets.length; i++) {
        if (targets[i] && targets[i].getBoundingClientRect().top < 140) active = links[i];
      }
      if (window.innerHeight + window.scrollY >= document.body.scrollHeight - 2) active = links[links.length - 1];
      links.forEach(function (l) { l.classList.toggle("is-active", l === active); });
    };
    window.addEventListener("scroll", function () {
      if (!ticking) { ticking = true; requestAnimationFrame(spy); }
    }, { passive: true });
    spy();
  }
})();
