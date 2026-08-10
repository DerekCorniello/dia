(function () {
  var cfg = window.__diaPluginConfig || {};
  var weekStartsMonday = cfg.week_starts_monday !== false;

  var STORAGE_KEY = 'dia-time-tracker-sessions';

  var root = document.getElementById('root');
  if (!root) return;

  var bgColor = '#1e1e2e';
  var bgAlt = '#181825';
  var textColor = '#cdd6f4';
  var mutedColor = '#585b70';
  var accentColor = '#89b4fa';
  var greenColor = '#a6e3a1';
  var redColor = '#f38ba8';
  var borderColor = '#313244';

  // Sessions are { name, notes?, start, duration } in milliseconds,
  // plus an optional running session under the same shape.
  function loadSessions() {
    try { return JSON.parse(localStorage.getItem(STORAGE_KEY)) || { list: [], running: null }; }
    catch (e) { return { list: [], running: null }; }
  }
  function saveSessions(s) {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(s)); } catch (e) {}
  }

  function fmtDuration(ms) {
    var s = Math.floor(ms / 1000);
    var h = Math.floor(s / 3600);
    var m = Math.floor((s % 3600) / 60);
    var sec = s % 60;
    if (h > 0) return h + 'h ' + m + 'm';
    if (m > 0) return m + 'm ' + sec + 's';
    return sec + 's';
  }

  function fmtClock(ts) {
    var d = new Date(ts);
    return String(d.getHours()).padStart(2, '0') + ':' + String(d.getMinutes()).padStart(2, '0');
  }

  function dayKey(ts) {
    var d = new Date(ts);
    return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
  }

  function weekStartKey(ts) {
    var d = new Date(ts);
    var day = d.getDay();
    if (weekStartsMonday) { day = (day + 6) % 7; }
    d.setDate(d.getDate() - day);
    return dayKey(d.getTime());
  }

  function startSession(name) {
    var s = loadSessions();
    if (s.running) return false;
    s.running = { name: name, start: Date.now(), duration: 0 };
    saveSessions(s);
    return true;
  }

  function stopSession() {
    var s = loadSessions();
    if (!s.running) return null;
    s.running.duration = Date.now() - s.running.start;
    s.sessions.push(s.running);
    s.running = null;
    saveSessions(s);
    return s.sessions[s.sessions.length - 1];
  }

  function deleteSession(index) {
    var s = loadSessions();
    s.sessions.splice(index, 1);
    saveSessions(s);
  }

  function exportCsv() {
    var s = loadSessions();
    var lines = ['name,start,duration_ms'];
    s.sessions.forEach(function (e) {
      lines.push('"' + String(e.name).replace(/"/g, '""') + '",' + new Date(e.start).toISOString() + ',' + e.duration);
    });
    var blob = new Blob([lines.join('\n')], { type: 'text/csv' });
    var url = URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = 'dia-time-tracker.csv';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  function render() {
    var s = loadSessions();
    var now = Date.now();

    root.innerHTML = '';
    root.style.cssText = 'display:flex;flex-direction:column;height:100%;background:' + bgColor + ';color:' + textColor + ';font-family:-apple-system,Segoe UI,sans-serif;';

    var header = document.createElement('div');
    header.style.cssText = 'display:flex;align-items:center;gap:8px;padding:10px 14px;background:' + bgAlt + ';border-bottom:1px solid ' + borderColor + ';';
    var title = document.createElement('span');
    title.style.cssText = 'font-size:13px;font-weight:600;color:' + accentColor + ';text-transform:uppercase;letter-spacing:1px;';
    title.textContent = 'Time Tracker';
    header.appendChild(title);

    header.appendChild(document.createElement('div'));
    // flexible spacer
    header.children[header.children.length - 1].style.flex = '1';

    var exportBtn = toolButton('export csv');
    exportBtn.addEventListener('click', exportCsv);
    header.appendChild(exportBtn);
    root.appendChild(header);

    var body = document.createElement('div');
    body.style.cssText = 'flex:1;overflow-y:auto;padding:14px;display:flex;flex-direction:column;gap:14px;';
    root.appendChild(body);

    // Running session card
    if (s.running) {
      var running = document.createElement('div');
      running.style.cssText = 'border:1px solid ' + greenColor + ';border-radius:8px;padding:12px;background:' + bgAlt + ';';
      var runLabel = document.createElement('div');
      runLabel.style.cssText = 'font-size:11px;text-transform:uppercase;letter-spacing:1px;color:' + greenColor + ';margin-bottom:6px;';
      runLabel.textContent = 'Running';
      running.appendChild(runLabel);

      var runName = document.createElement('div');
      runName.style.cssText = 'font-size:16px;font-weight:600;';
      runName.textContent = s.running.name;
      running.appendChild(runName);

      var runElapsed = document.createElement('div');
      runElapsed.style.cssText = 'font-size:24px;font-weight:700;font-variant-numeric:tabular-nums;color:' + accentColor + ';margin:6px 0;';
      var elapsed = now - s.running.start;
      runElapsed.textContent = fmtDuration(elapsed);
      running.appendChild(runElapsed);

      var stopBtn = toolButton('stop');
      stopBtn.style.cssText += ';background:' + redColor + ';color:#1e1e2e;';
      stopBtn.addEventListener('click', function () {
        stopSession();
        render();
      });
      running.appendChild(stopBtn);
      body.appendChild(running);
    }

    // Start controls
    var form = document.createElement('div');
    form.style.cssText = 'display:flex;gap:8px;align-items:center;';
    var input = document.createElement('input');
    input.type = 'text';
    input.placeholder = 'workspace or task name';
    input.style.cssText = 'flex:1;background:' + bgAlt + ';border:1px solid ' + borderColor + ';border-radius:6px;padding:8px 10px;color:' + textColor + ';font:13px -apple-system,Segoe UI,sans-serif;outline:none;';
    form.appendChild(input);

    var startBtn = toolButton('start');
    startBtn.addEventListener('click', function () {
      var name = input.value.trim();
      if (!name) return;
      if (!startSession(name)) { showToast('A session is already running'); return; }
      render();
    });
    input.addEventListener('keydown', function (e) { if (e.key === 'Enter') startBtn.click(); });
    form.appendChild(startBtn);

    if (s.running) { input.disabled = true; startBtn.disabled = true; startBtn.style.opacity = '0.4'; }

    body.appendChild(form);

    // Totals
    var totals = document.createElement('div');
    totals.style.cssText = 'display:flex;gap:8px;';
    var todayMs = 0;
    var weekMs = 0;
    var today = dayKey(now);
    var week = weekStartKey(now);
    s.sessions.forEach(function (e) {
      if (dayKey(e.start) === today) todayMs += e.duration;
      if (weekStartKey(e.start) === week) weekMs += e.duration;
    });
    function totalBadge(label, ms) {
      var b = document.createElement('div');
      b.style.cssText = 'flex:1;border:1px solid ' + borderColor + ';border-radius:8px;padding:10px;text-align:center;background:' + bgAlt + ';';
      var l = document.createElement('div');
      l.style.cssText = 'font-size:10px;text-transform:uppercase;letter-spacing:1px;color:' + mutedColor + ';';
      l.textContent = label;
      b.appendChild(l);
      var v = document.createElement('div');
      v.style.cssText = 'font-size:18px;font-weight:700;margin-top:4px;color:' + accentColor + ';';
      v.textContent = fmtDuration(ms);
      b.appendChild(v);
      return b;
    }
    totals.appendChild(totalBadge('today', todayMs));
    totals.appendChild(totalBadge('this week', weekMs));
    totals.appendChild(totalBadge('total sessions', s.sessions.length));
    body.appendChild(totals);

    // History
    var historyLabel = document.createElement('div');
    historyLabel.style.cssText = 'font-size:10px;text-transform:uppercase;letter-spacing:1px;color:' + mutedColor + ';margin-top:4px;';
    historyLabel.textContent = 'Past sessions (' + s.sessions.length + ')';
    body.appendChild(historyLabel);

    if (s.sessions.length === 0) {
      var empty = document.createElement('div');
      empty.style.cssText = 'color:' + mutedColor + ';font-size:13px;padding:8px 0;';
      empty.textContent = 'No finished sessions yet. Start one above.';
      body.appendChild(empty);
    }

    var list = document.createElement('div');
    list.style.cssText = 'display:flex;flex-direction:column;gap:6px;';
    for (var i = s.sessions.length - 1; i >= 0; i--) {
      (function (session, idx) {
        var row = document.createElement('div');
        row.style.cssText = 'display:flex;align-items:center;gap:10px;padding:8px 10px;background:' + bgAlt + ';border:1px solid ' + borderColor + ';border-radius:6px;';
        var nameEl = document.createElement('div');
        nameEl.style.cssText = 'flex:1;min-width:0;font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;';
        nameEl.textContent = session.name;
        row.appendChild(nameEl);
        var when = document.createElement('div');
        when.style.cssText = 'font-size:11px;color:' + mutedColor + ';font-variant-numeric:tabular-nums;';
        when.textContent = fmtClock(session.start);
        row.appendChild(when);
        var dur = document.createElement('div');
        dur.style.cssText = 'font-size:13px;font-weight:600;color:' + accentColor + ';font-variant-numeric:tabular-nums;min-width:64px;text-align:right;';
        dur.textContent = fmtDuration(session.duration);
        row.appendChild(dur);
        var del = document.createElement('button');
        del.type = 'button';
        del.textContent = 'x';
        del.title = 'delete';
        del.style.cssText = 'border:0;background:transparent;color:' + mutedColor + ';cursor:pointer;font-size:12px;padding:2px 6px;';
        del.addEventListener('click', function () {
          if (confirm('Delete this session?')) { deleteSession(idx); render(); }
        });
        row.appendChild(del);
        list.appendChild(row);
      })(s.sessions[i], i);
    }
    body.appendChild(list);
  }

  function toolButton(label) {
    var btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = label;
    btn.style.cssText = 'padding:8px 14px;background:' + accentColor + ';color:#1e1e2e;border:0;border-radius:6px;cursor:pointer;font:12px -apple-system,Segoe UI,sans-serif;font-weight:600;';
    return btn;
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

  render();
  setInterval(function () {
    var s = loadSessions();
    if (s.running) render();
  }, 1000);
})();