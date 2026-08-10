(function () {
  var cfg = window.__diaPluginConfig || {};
  var baseUrl = (cfg.base_url || '').replace(/\/+$/, '');

  var STORAGE_KEY = 'dia-api-playground-templates';

  var root = document.getElementById('root');
  if (!root) return;

  var bgColor = '#1e1e2e';
  var bgAlt = '#181825';
  var textColor = '#cdd6f4';
  var mutedColor = '#585b70';
  var accentColor = '#89b4fa';
  var greenColor = '#a6e3a1';
  var redColor = '#f38ba8';
  var warnColor = '#f9e2af';
  var borderColor = '#313244';

  var state = {
    method: 'GET',
    url: '',
    headers: '',
    body: '',
  };
  var activeResponse = null;
  var sendButton = null;

  function loadTemplates() {
    try { return JSON.parse(localStorage.getItem(STORAGE_KEY)) || {}; }
    catch (e) { return {}; }
  }
  function saveTemplates(t) {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(t)); } catch (e) {}
  }
  function expandBase(url) {
    var u = (url || '').trim();
    if (!u) return u;
    if (!baseUrl) return u;
    if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(u)) return u;
    return baseUrl + (u.charAt(0) === '/' ? u : '/' + u);
  }

  function parseHeaders(raw) {
    var out = {};
    if (!raw.trim()) return out;
    try {
      var obj = JSON.parse(raw);
      for (var k in obj) {
        if (Object.prototype.hasOwnProperty.call(obj, k)) out[k] = String(obj[k]);
      }
      return out;
    } catch (e) {
      raw.split('\n').forEach(function (line) {
        var idx = line.indexOf(':');
        if (idx > 0) out[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
      });
      return out;
    }
  }

  function send() {
    var full = expandBase(state.url);
    if (!full) { showToast('Enter a URL'); return; }
    var headers = parseHeaders(state.headers);
    var opts = { method: state.method, headers: headers };
    if (state.method !== 'GET' && state.body && state.body.trim()) {
      opts.body = state.body;
    }

    if (sendButton) {
      sendButton.disabled = true;
      sendButton.textContent = 'sending...';
      sendButton.style.opacity = '0.6';
    }

    var started = Date.now();
    window.dia.call('fetch', [full, opts]).then(function (data) {
      activeResponse = { ok: true, latency: Date.now() - started, data: data };
      renderResponse();
    }).catch(function (err) {
      activeResponse = { ok: false, latency: Date.now() - started, error: String(err && err.message ? err.message : err) };
      renderResponse();
    }).then(function () {
      if (sendButton) {
        sendButton.disabled = false;
        sendButton.textContent = 'send';
        sendButton.style.opacity = '1';
      }
    });
  }

  function renderResponse() {
    root.innerHTML = '';
    root.appendChild(layout(composerPane(), responsePane()));
  }

  // ---- composer ----
  function composerPane() {
    var pane = document.createElement('div');
    pane.style.cssText = 'flex:1;display:flex;flex-direction:column;gap:10px;overflow-y:auto;padding:14px;min-width:340px;';

    pane.appendChild(heading('Request'));

    var urlRow = document.createElement('div');
    urlRow.style.cssText = 'display:flex;gap:8px;';

    var methodSel = document.createElement('select');
    ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].forEach(function (m) {
      var opt = document.createElement('option');
      opt.value = m;
      opt.textContent = m;
      if (m === state.method) opt.selected = true;
      methodSel.appendChild(opt);
    });
    styleSelect(methodSel);
    methodSel.addEventListener('change', function () {
      state.method = methodSel.value;
      methodSel.style.borderColor = methodColor(state.method);
    });
    urlRow.appendChild(methodSel);

    var urlInput = document.createElement('input');
    urlInput.type = 'text';
    urlInput.placeholder = baseUrl ? '/' : 'https://api.example.com/v1/users';
    urlInput.value = state.url;
    urlInput.style.cssText = styleInput();
    urlInput.style.flex = '1';
    urlInput.addEventListener('input', function () { state.url = urlInput.value; });
    urlRow.appendChild(urlInput);
    pane.appendChild(urlRow);

    var targetHint = document.createElement('div');
    targetHint.style.cssText = 'font-size:11px;color:' + mutedColor + ';word-break:break-all;margin-top:-4px;';
    targetHint.textContent = expandBase(state.url);
    pane.appendChild(targetHint);

    pane.appendChild(smallLabel('Headers (JSON or "Key: Value" lines)'));
    var headersArea = document.createElement('textarea');
    headersArea.value = state.headers;
    headersArea.rows = 3;
    headersArea.placeholder = '{"Content-Type": "application/json"}\nor\nContent-Type: application/json';
    headersArea.style.cssText = styleTextarea();
    headersArea.addEventListener('input', function () { state.headers = headersArea.value; });
    pane.appendChild(headersArea);

    pane.appendChild(smallLabel('Body'));
    var bodyArea = document.createElement('textarea');
    bodyArea.value = state.body;
    bodyArea.rows = 7;
    bodyArea.placeholder = '{"name": "ada"}';
    bodyArea.style.cssText = styleTextarea();
    bodyArea.addEventListener('input', function () { state.body = bodyArea.value; });
    pane.appendChild(bodyArea);

    var actions = document.createElement('div');
    actions.style.cssText = 'display:flex;gap:8px;';
    sendButton = actionButton('send', greenColor);
    sendButton.addEventListener('click', send);
    actions.appendChild(sendButton);

    var clearBtn = actionButton('clear', borderColor);
    clearBtn.style.background = 'transparent';
    clearBtn.style.color = mutedColor;
    clearBtn.addEventListener('click', function () {
      state.method = 'GET';
      state.url = '';
      state.headers = '';
      state.body = '';
      activeResponse = null;
      renderResponse();
    });
    actions.appendChild(clearBtn);
    pane.appendChild(actions);

    return pane;
  }

  function responsePane() {
    var pane = document.createElement('div');
    pane.style.cssText = 'flex:1;display:flex;flex-direction:column;gap:10px;overflow-y:auto;padding:14px;min-width:0;';

    pane.appendChild(heading('Response'));
    if (!activeResponse) {
      pane.appendChild(emptyHint('Hit send to make a request.'));
      return pane;
    }

    var meta = document.createElement('div');
    meta.style.cssText = 'display:flex;gap:10px;align-items:center;font-size:12px;color:' + mutedColor + ';';
    var lat = document.createElement('span');
    lat.textContent = activeResponse.latency + ' ms';
    meta.appendChild(lat);

    if (!activeResponse.ok) {
      var err = document.createElement('span');
      err.textContent = 'error';
      err.style.cssText = 'color:' + redColor + ';font-weight:700;';
      meta.appendChild(err);
      pane.appendChild(meta);

      var errPre = document.createElement('pre');
      errPre.style.cssText = stylePre(redColor);
      errPre.textContent = activeResponse.error || 'request failed';
      pane.appendChild(errPre);
      return pane;
    }

    var ok = document.createElement('span');
    ok.textContent = '200-ish';
    ok.style.cssText = 'color:' + greenColor + ';font-weight:700;font-family:monospace;';
    meta.appendChild(ok);
    pane.appendChild(meta);

    var pretty;
    try { pretty = JSON.stringify(activeResponse.data, null, 2); }
    catch (e) { pretty = String(activeResponse.data); }
    var pre = document.createElement('pre');
    pre.style.cssText = stylePre(greenColor);
    pre.textContent = pretty;
    pane.appendChild(pre);

    return pane;
  }

  // ---- saved templates ----
  function templatesPane() {
    var pane = document.createElement('div');
    pane.style.cssText = 'flex:1;display:flex;flex-direction:column;gap:10px;overflow-y:auto;padding:14px;';
    pane.appendChild(heading('Saved requests (' + Object.keys(loadTemplates()).length + ')'));

    var formRow = document.createElement('div');
    formRow.style.cssText = 'display:flex;gap:8px;';
    var nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.placeholder = 'template name';
    nameInput.style.cssText = styleInput();
    nameInput.style.flex = '1';
    var saveBtn = actionButton('save current', accentColor);
    saveBtn.addEventListener('click', function () {
      var name = nameInput.value.trim();
      if (!name) { showToast('Enter a name'); return; }
      var tpl = loadTemplates();
      tpl[name] = { method: state.method, url: state.url, headers: state.headers, body: state.body };
      saveTemplates(tpl);
      nameInput.value = '';
      renderTemplates();
    });
    formRow.appendChild(saveBtn);
    pane.appendChild(formRow);

    var tpl = loadTemplates();
    var names = Object.keys(tpl).sort();
    if (names.length === 0) {
      pane.appendChild(emptyHint('Nothing saved yet. Compose a request, click save, and it shows up here.'));
      return pane;
    }

    names.forEach(function (name) {
      (function (template) {
        var row = document.createElement('div');
        row.style.cssText = 'display:flex;align-items:center;gap:8px;padding:8px 10px;background:' + bgAlt + ';border:1px solid ' + borderColor + ';border-radius:6px;';

        var badge = document.createElement('span');
        badge.textContent = template.method;
        badge.style.cssText = 'font:11px monospace;font-weight:700;color:' + methodColor(template.method) + ';';
        row.appendChild(badge);

        var info = document.createElement('div');
        info.style.cssText = 'flex:1;min-width:0;';
        var tName = document.createElement('div');
        tName.style.cssText = 'font-size:13px;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
        tName.textContent = name;
        info.appendChild(tName);
        var tUrl = document.createElement('div');
        tUrl.style.cssText = 'font-size:11px;color:' + mutedColor + ';white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
        tUrl.textContent = template.url || '(no url)';
        info.appendChild(tUrl);
        row.appendChild(info);

        var loadBtn = actionButton('load', borderColor);
        loadBtn.addEventListener('click', function () {
          state.method = template.method;
          state.url = template.url || '';
          state.headers = template.headers || '';
          state.body = template.body || '';
          activeResponse = null;
          renderComposer();
        });
        row.appendChild(loadBtn);

        var delBtn = actionButton('remove', 'transparent');
        delBtn.style.color = redColor;
        delBtn.addEventListener('click', function () {
          var t2 = loadTemplates();
          delete t2[name];
          saveTemplates(t2);
          renderTemplates();
        });
        row.appendChild(delBtn);

        pane.appendChild(row);
      })(tpl[name]);
    });

    return pane;
  }

  // ---- layout ----
  function layout(activeTabId, left, right) {
    var wrap = document.createElement('div');
    wrap.style.cssText = 'display:flex;flex-direction:column;flex:1;min-height:0;';

    var tabbar = document.createElement('div');
    tabbar.style.cssText = 'display:flex;gap:2px;padding:6px 12px 0;background:' + bgAlt + ';border-bottom:1px solid ' + borderColor + ';';
    var tabs = [
      { id: 'composer', label: 'Composer', make: function () { renderComposer(); } },
      { id: 'saved', label: 'Saved requests', make: function () { renderTemplates(); } },
    ];
    tabs.forEach(function (t) {
      var active = t.id === activeTabId;
      var btn = document.createElement('button');
      btn.type = 'button';
      btn.textContent = t.label;
      btn.style.cssText = 'padding:6px 12px;border:0;cursor:pointer;font:12px -apple-system,Segoe UI,sans-serif;border-radius:6px 6px 0 0;background:' + (active ? accentColor : 'transparent') + ';color:' + (active ? '#1e1e2e' : mutedColor) + ';';
      btn.addEventListener('click', t.make);
      tabbar.appendChild(btn);
    });
    wrap.appendChild(tabbar);

    var body = document.createElement('div');
    body.style.cssText = 'flex:1;display:flex;min-height:0;';
    body.appendChild(left);
    if (right) body.appendChild(right);
    wrap.appendChild(body);
    return wrap;
  }

  function renderComposer() {
    root.innerHTML = '';
    root.appendChild(layout('composer', composerPane(), responsePane()));
  }
  function renderTemplates() {
    root.innerHTML = '';
    root.appendChild(layout('saved', templatesPane()));
  }
  function renderResponse() {
    renderComposer();
  }

  // ---- helpers ----
  function styleInput() {
    return 'background:' + bgAlt + ';border:1px solid ' + borderColor + ';border-radius:6px;padding:8px 10px;color:' + textColor + ';font:13px ui-monospace,menlo,monospace;outline:none;';
  }
  function styleTextarea() {
    return 'resize:vertical;background:' + bgAlt + ';border:1px solid ' + borderColor + ';border-radius:6px;padding:8px 10px;color:' + textColor + ';font:13px/1.5 ui-monospace,menlo,monospace;outline:none;';
  }
  function styleSelect(sel) {
    sel.style.cssText = 'background:' + bgAlt + ';border:1px solid ' + methodColor(state.method) + ';border-radius:6px;padding:8px 6px;color:' + textColor + ';font:13px ui-monospace,menlo,monospace;outline:none;cursor:pointer;';
  }
  function stylePre(accent) {
    return 'margin:0;overflow:auto;background:' + bgAlt + ';border:1px solid ' + borderColor + ';border-left:3px solid ' + accent + ';border-radius:6px;padding:10px;color:' + textColor + ';font:12px/1.5 ui-monospace,menlo,monospace;white-space:pre-wrap;word-break:break-word;';
  }
  function heading(label) {
    var h = document.createElement('div');
    h.style.cssText = 'font-size:10px;text-transform:uppercase;letter-spacing:1px;color:' + mutedColor + ';';
    h.textContent = label;
    return h;
  }
  function smallLabel(label) {
    var l = document.createElement('div');
    l.style.cssText = 'font-size:11px;color:' + mutedColor + ';';
    l.textContent = label;
    return l;
  }
  function emptyHint(msg) {
    var d = document.createElement('div');
    d.style.cssText = 'color:' + mutedColor + ';font-size:13px;padding:16px 0;';
    d.textContent = msg;
    return d;
  }
  function actionButton(label, color) {
    var btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = label;
    btn.style.cssText = 'padding:8px 16px;background:' + color + ';color:' + (color === borderColor || color === 'transparent' ? textColor : '#1e1e2e') + ';border:0;border-radius:6px;cursor:pointer;font:12px -apple-system,Segoe UI,sans-serif;font-weight:600;';
    return btn;
  }
  function methodColor(m) {
    switch (m) {
      case 'GET': return greenColor;
      case 'POST': return accentColor;
      case 'PUT': return warnColor;
      case 'PATCH': return '#f4c2e7';
      case 'DELETE': return redColor;
      default: return textColor;
    }
  }

  var toastTimeout;
  function showToast(msg) {
    var existing = document.getElementById('toast');
    if (existing) existing.remove();
    if (toastTimeout) clearTimeout(toastTimeout);

    var toast = document.createElement('div');
    toast.id = 'toast';
    toast.textContent = msg;
    toast.style.cssText = 'position:fixed;bottom:16px;left:50%;transform:translateX(-50%);background:' + accentColor + ';color:#1e1e2e;padding:6px 16px;border-radius:6px;font-size:12px;font-weight:500;z-index:999;opacity:0;transition:opacity 0.3s;';
    document.body.appendChild(toast);
    requestAnimationFrame(function () { toast.style.opacity = '1'; });
    toastTimeout = setTimeout(function () {
      toast.style.opacity = '0';
      setTimeout(function () { toast.remove(); }, 300);
    }, 1500);
  }

  renderComposer();
})();