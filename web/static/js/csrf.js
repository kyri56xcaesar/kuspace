// csrf.js - sends the csrf_token cookie back as X-CSRF-Token on every
// state-changing same-origin request (htmx, fetch, XMLHttpRequest).
// Load it before any other script. See internal/frontapp/csrf.go.
(function () {
  "use strict";
  var SAFE = { GET: 1, HEAD: 1, OPTIONS: 1 };

  function token() {
    var m = document.cookie.match(/(?:^|;\s*)csrf_token=([0-9a-f]+)/);
    return m ? m[1] : "";
  }
  function sameOrigin(url) {
    try { return new URL(url, location.href).origin === location.origin; } catch (e) { return false; }
  }

  // htmx
  document.addEventListener("htmx:configRequest", function (e) {
    if (!SAFE[(e.detail.verb || "").toUpperCase()]) e.detail.headers["X-CSRF-Token"] = token();
  });

  // fetch
  if (window.fetch) {
    var origFetch = window.fetch;
    window.fetch = function (input, init) {
      init = init || {};
      var method = (init.method || (input && input.method) || "GET").toUpperCase();
      var url = typeof input === "string" ? input : (input && input.url) || "";
      if (!SAFE[method] && sameOrigin(url)) {
        var h = new Headers(init.headers || (input && input.headers) || {});
        h.set("X-CSRF-Token", token());
        init.headers = h;
      }
      return origFetch.call(this, input, init);
    };
  }

  // XMLHttpRequest
  var open = XMLHttpRequest.prototype.open, send = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function (method, url) {
    this._csrf = !SAFE[String(method).toUpperCase()] && sameOrigin(url);
    return open.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function () {
    if (this._csrf) this.setRequestHeader("X-CSRF-Token", token());
    return send.apply(this, arguments);
  };
})();
