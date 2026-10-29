// The one script every page loads.
//
// Every page reads without it: the contents of a volume are a link to the
// volume page, a reference is a link, and prev and next are links. What it
// adds is the volume's contents beside the text, the page's own outline, a
// preview of a reference on hover, dark mode, and the keys.
(function () {
  "use strict";
  var root = document.documentElement;

  // Theme. The page head sets the saved choice before the first paint; this
  // is the button.
  function theme() {
    var t = root.dataset.theme;
    if (!t) t = matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    return t;
  }
  document.querySelectorAll("[data-theme-toggle]").forEach(function (b) {
    b.addEventListener("click", function () {
      var t = theme() === "dark" ? "light" : "dark";
      root.dataset.theme = t;
      try { localStorage.setItem("theme", t); } catch (e) {}
    });
  });

  // The volume's contents, which are one file per volume rather than a copy
  // on every page, so that a volume of a thousand pages carries its contents
  // once and the browser caches it.
  var nav = document.querySelector(".side-nav[data-src]");
  if (nav) {
    fetch(nav.dataset.src, { credentials: "same-origin" })
      .then(function (r) { return r.ok ? r.text() : Promise.reject(r.status); })
      .then(function (html) {
        nav.innerHTML = html;
        markCurrent(nav);
      })
      .catch(function () {});
  }
  function markCurrent(nav) {
    var here = nav.dataset.here || location.pathname, best = null, bestLen = 0;
    nav.querySelectorAll("a[href]").forEach(function (a) {
      var p = new URL(a.href, location.href).pathname;
      // The longest link the page is under: an exercise is under its section.
      if (here.indexOf(p) === 0 && p.length > bestLen) { best = a; bestLen = p.length; }
    });
    if (!best) return;
    best.setAttribute("aria-current", "page");
    for (var d = best.closest("details"); d; d = d.parentElement && d.parentElement.closest("details")) d.open = true;
    var side = document.querySelector(".side");
    if (side) {
      var r = best.getBoundingClientRect(), s = side.getBoundingClientRect();
      if (r.top < s.top || r.bottom > s.bottom) side.scrollTop += r.top - s.top - s.height / 3;
    }
  }

  // The contents button on a phone.
  document.querySelectorAll("[data-toggle=side]").forEach(function (b) {
    b.addEventListener("click", function () { document.body.classList.toggle("side-open"); });
  });
  document.addEventListener("click", function (e) {
    if (document.body.classList.contains("side-open") && !e.target.closest(".side, [data-toggle=side]")) {
      document.body.classList.remove("side-open");
    }
  });

  // The page's own outline: its headings and its statements, in order.
  var outline = document.querySelector(".outline");
  var doc = document.querySelector(".doc");
  var heads = [];
  if (outline && doc) {
    doc.querySelectorAll(".doc > h2[id], .doc > h3[id], .doc > h2, .stmt > :is(h2, h3, h4, h5, h6):first-child").forEach(function (h) {
      if (heads.indexOf(h) >= 0) return;
      var box = h.closest(".stmt");
      var target = box || h;
      if (!target.id) {
        if (h.id) { target.id = h.id; h.removeAttribute("id"); }
        else target.id = "s-" + heads.length;
      }
      heads.push(h);
    });
    if (heads.length >= 2) {
      var ol = document.createElement("ol");
      heads.forEach(function (h) {
        var box = h.closest(".stmt");
        var li = document.createElement("li");
        if (box || h.tagName === "H3") li.className = "sub";
        var a = document.createElement("a");
        a.href = "#" + (box || h).id;
        if (box) {
          var dot = document.createElement("span");
          dot.className = "dot";
          dot.style.setProperty("--k", getComputedStyle(box).getPropertyValue("--k"));
          a.appendChild(dot);
        }
        var t = document.createElement("span");
        t.textContent = label(h);
        a.appendChild(t);
        li.appendChild(a);
        ol.appendChild(li);
      });
      var title = document.createElement("h2");
      title.textContent = document.body.dataset.outline || "On this page";
      outline.appendChild(title);
      outline.appendChild(ol);
      var top = document.createElement("a");
      top.href = "#";
      top.className = "top";
      top.textContent = document.body.dataset.top || "Back to top ↑";
      outline.appendChild(top);
      spy(ol);
    }
  }
  function label(h) {
    var c = h.cloneNode(true);
    c.querySelectorAll(".tag, .katex-mathml").forEach(function (n) { n.remove(); });
    var t = c.textContent.replace(/\s+/g, " ").trim();
    return t.length > 60 ? t.slice(0, 58) + "…" : t;
  }
  function spy(ol) {
    var links = ol.querySelectorAll("a");
    var targets = Array.prototype.map.call(links, function (a) { return document.getElementById(a.hash.slice(1)); });
    var on = -1;
    function update() {
      var y = window.scrollY + 120, i = -1;
      for (var k = 0; k < targets.length; k++) if (targets[k] && targets[k].getBoundingClientRect().top + window.scrollY <= y) i = k;
      if (i === on) return;
      if (on >= 0) links[on].classList.remove("on");
      if (i >= 0) {
        links[i].classList.add("on");
        var o = outline.getBoundingClientRect(), r = links[i].getBoundingClientRect();
        if (r.top < o.top || r.bottom > o.bottom) outline.scrollTop += r.top - o.top - o.height / 2;
      }
      on = i;
    }
    var queued = false;
    window.addEventListener("scroll", function () {
      if (queued) return;
      queued = true;
      requestAnimationFrame(function () { queued = false; update(); });
    }, { passive: true });
    update();
  }

  // A preview of what a reference points at, on hover, so that a reader can
  // see what Lemma 3 says without losing their place.
  var peek = document.querySelector(".peek");
  var cache = {};
  var timer = 0, current = null, overPeek = false;
  function previewable(a) {
    if (!a || !a.href || a.closest(".peek, .side, .topbar, .pager, .crumbs")) return false;
    var u = new URL(a.href, location.href);
    if (u.origin !== location.origin) return false;
    if (u.pathname === location.pathname && u.hash) return !!document.getElementById(u.hash.slice(1));
    var p = u.pathname.replace(BASE(), "/");
    return /^\/tag\/[^/]+\/?/.test(p) || /\/ex\/\d+\/?$/.test(p) || (!!u.hash && /^\/[a-z-]+\/[a-z]+\//.test(p)) || a.classList.contains("ref");
  }
  function BASE() {
    var l = document.querySelector("link[rel=stylesheet][href$='style.css']");
    return l ? new URL(l.href).pathname.replace(/style\.css$/, "") : "/";
  }
  function extract(d, hash) {
    var out = document.createElement("div");
    var node = hash ? d.getElementById(hash) : null;
    if (node && !node.classList.contains("stmt")) node = node.closest(".stmt") || node;
    if (node) { out.appendChild(node.cloneNode(true)); return out; }
    var where = d.querySelector(".eyebrow");
    var h1 = d.querySelector("main h1");
    var body = d.querySelector(".doc .stmt, .doc .statement-body");
    if (where) { var w = document.createElement("div"); w.className = "peek-where"; w.textContent = where.textContent; out.appendChild(w); }
    if (h1) { var t = document.createElement("div"); t.className = "peek-title"; t.innerHTML = h1.innerHTML; out.appendChild(t); }
    if (body) out.appendChild(body.cloneNode(true));
    else {
      var ps = d.querySelectorAll(".doc > p, .doc > .math");
      for (var i = 0; i < ps.length && i < 3; i++) out.appendChild(ps[i].cloneNode(true));
    }
    out.querySelectorAll(".tag").forEach(function (n) { n.remove(); });
    return out;
  }
  function show(a) {
    var u = new URL(a.href, location.href);
    var key = u.pathname + u.hash;
    var place = function (content) {
      if (current !== a) return;
      peek.innerHTML = "";
      peek.appendChild(content.cloneNode(true));
      peek.hidden = false;
      var r = a.getBoundingClientRect();
      var w = peek.offsetWidth, h = peek.offsetHeight;
      var left = Math.min(Math.max(8, r.left + window.scrollX - 20), window.scrollX + document.documentElement.clientWidth - w - 8);
      var top = r.bottom + window.scrollY + 8;
      if (r.bottom + h + 16 > window.innerHeight && r.top > h + 16) top = r.top + window.scrollY - h - 8;
      peek.style.left = left + "px";
      peek.style.top = top + "px";
    };
    if (u.pathname === location.pathname && u.hash) {
      place(extract(document, u.hash.slice(1)));
      return;
    }
    if (cache[key]) { place(cache[key]); return; }
    fetch(u.pathname, { credentials: "same-origin" })
      .then(function (r) { return r.ok ? r.text() : Promise.reject(r.status); })
      .then(function (html) {
        var d = new DOMParser().parseFromString(html, "text/html");
        cache[key] = extract(d, u.hash.slice(1));
        place(cache[key]);
      })
      .catch(function () {});
  }
  function hide() { peek.hidden = true; current = null; }
  if (peek && matchMedia("(hover: hover)").matches) {
    document.addEventListener("mouseover", function (e) {
      var a = e.target.closest && e.target.closest("a[href]");
      if (e.target.closest && e.target.closest(".peek")) { overPeek = true; clearTimeout(timer); return; }
      if (!a || !previewable(a)) return;
      if (current === a) { clearTimeout(timer); return; }
      clearTimeout(timer);
      timer = setTimeout(function () { current = a; show(a); }, 280);
    });
    document.addEventListener("mouseout", function (e) {
      var to = e.relatedTarget;
      if (to && to.closest && (to.closest(".peek") || (current && to.closest("a") === current))) return;
      if (e.target.closest && (e.target.closest(".peek") || e.target.closest("a[href]"))) {
        clearTimeout(timer);
        timer = setTimeout(hide, 220);
      }
    });
    document.addEventListener("keydown", function (e) { if (e.key === "Escape") hide(); });
  }

  // Keys: / for search, [ and ] for the previous and next page.
  document.addEventListener("keydown", function (e) {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    var t = e.target;
    if (t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName))) {
      if (e.key === "Escape") t.blur();
      return;
    }
    if (e.key === "/") {
      var q = document.querySelector(".find input");
      if (q) { e.preventDefault(); q.focus(); q.select(); }
    } else if (e.key === "[" || e.key === "]") {
      var a = document.querySelector(e.key === "[" ? ".pager .prev" : ".pager .next");
      if (a) location.href = a.href;
    }
  });

  // The library filter.
  var filter = document.querySelector("[data-filter]");
  if (filter) {
    var items = document.querySelectorAll(filter.dataset.filter);
    var shown = document.querySelector("[data-filter-count]");
    var run = function () {
      var words = filter.value.toLowerCase().split(/\s+/).filter(Boolean), n = 0;
      items.forEach(function (it) {
        var hay = (it.dataset.text || it.textContent).toLowerCase();
        var ok = words.every(function (w) { return hay.indexOf(w) >= 0; });
        it.hidden = !ok;
        if (ok) n++;
      });
      if (shown) shown.textContent = n + (n === 1 ? " volume" : " volumes");
    };
    filter.addEventListener("input", run);
    run();
  }
})();
