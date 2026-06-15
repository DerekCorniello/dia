(function () {
  var cfg = window.__diaPluginConfig || {};
  var focusMin = cfg.focus_minutes || 25;
  var breakMin = cfg.break_minutes || 5;
  var longBreakMin = cfg.long_break_minutes || 15;
  var sessionsBeforeLong = cfg.sessions_before_long || 4;

  var root = document.getElementById('root');
  if (!root) return;

  var state = 'idle'; // idle | focus | break | long_break
  var timeLeft = focusMin * 60;
  var timerId = null;
  var sessionCount = 0;
  var totalFocus = 0;

  var bgColor = '#1e1e2e';
  var accentColor = '#89b4fa';
  var textColor = '#cdd6f4';
  var mutedColor = '#585b70';

  function fmt(s) {
    var m = Math.floor(s / 60);
    var sec = Math.floor(s % 60);
    return String(m).padStart(2, '0') + ':' + String(sec).padStart(2, '0');
  }

  function beep() {
    try {
      var a = new AudioContext();
      var osc = a.createOscillator();
      var gain = a.createGain();
      osc.connect(gain);
      gain.connect(a.destination);
      osc.frequency.value = 800;
      gain.gain.value = 0.3;
      osc.start();
      setTimeout(function () { osc.stop(); a.close(); }, 200);
    } catch (e) {}
  }

  function notify(title, body) {
    if ('Notification' in window && Notification.permission === 'granted') {
      new Notification(title, { body: body });
    }
    beep();
  }

  function tick() {
    if (timeLeft <= 0) {
      clearInterval(timerId);
      timerId = null;
      if (state === 'focus') {
        sessionCount++;
        totalFocus += (state === 'focus' ? focusMin : 0);
        if (sessionCount % sessionsBeforeLong === 0) {
          state = 'long_break';
          timeLeft = longBreakMin * 60;
          notify('Long break time!', 'Take a ' + longBreakMin + ' minute break.');
        } else {
          state = 'break';
          timeLeft = breakMin * 60;
          notify('Break time!', 'Take a ' + breakMin + ' minute break.');
        }
      } else {
        state = 'focus';
        timeLeft = focusMin * 60;
        notify('Focus time!', 'Work for ' + focusMin + ' minutes.');
      }
      render();
      return;
    }
    timeLeft--;
    render();
  }

  function startTimer() {
    if (state === 'idle') {
      state = 'focus';
      timeLeft = focusMin * 60;
    }
    if (timerId) return;
    timerId = setInterval(tick, 1000);
    render();
  }

  function pauseTimer() {
    if (timerId) {
      clearInterval(timerId);
      timerId = null;
    }
    render();
  }

  function resetTimer() {
    if (timerId) { clearInterval(timerId); timerId = null; }
    state = 'idle';
    timeLeft = focusMin * 60;
    sessionCount = 0;
    totalFocus = 0;
    render();
  }

  function render() {
    root.innerHTML = '';
    root.style.cssText = 'display:flex;flex-direction:column;align-items:center;justify-content:center;height:100%;background:' + bgColor + ';color:' + textColor + ';font-family:-apple-system,Segoe UI,sans-serif;gap:16px;';

    var phaseLabel = document.createElement('div');
    phaseLabel.style.cssText = 'font-size:13px;font-weight:600;text-transform:uppercase;letter-spacing:2px;color:' + accentColor + ';';
    if (state === 'focus') phaseLabel.textContent = 'Focus';
    else if (state === 'break') phaseLabel.textContent = 'Break';
    else if (state === 'long_break') phaseLabel.textContent = 'Long Break';
    else phaseLabel.textContent = 'Ready';
    root.appendChild(phaseLabel);

    var timerDisplay = document.createElement('div');
    timerDisplay.style.cssText = 'font-size:64px;font-weight:700;font-variant-numeric:tabular-nums;line-height:1;';
    timerDisplay.textContent = fmt(timeLeft);
    root.appendChild(timerDisplay);

    var sessionInfo = document.createElement('div');
    sessionInfo.style.cssText = 'font-size:12px;color:' + mutedColor + ';';
    sessionInfo.textContent = 'Session ' + (sessionCount + 1) + ' of ' + sessionsBeforeLong + ' to long break';
    root.appendChild(sessionInfo);

    var controls = document.createElement('div');
    controls.style.cssText = 'display:flex;gap:10px;';

    if (timerId) {
      var pauseBtn = createButton('pause', function () { pauseTimer(); });
      controls.appendChild(pauseBtn);
    } else {
      var startBtn = createButton('start', function () { startTimer(); });
      controls.appendChild(startBtn);
    }

    var resetBtn = createButton('reset', function () { resetTimer(); });
    controls.appendChild(resetBtn);

    root.appendChild(controls);

    var stats = document.createElement('div');
    stats.style.cssText = 'font-size:11px;color:' + mutedColor + ';margin-top:8px;';
    stats.textContent = 'Total focus today: ' + fmt(totalFocus * 60);
    root.appendChild(stats);
  }

  function createButton(label, onClick) {
    var btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = label;
    btn.style.cssText = 'padding:8px 24px;background:#313244;color:' + textColor + ';border:0;border-radius:8px;cursor:pointer;font:14px -apple-system,Segoe UI,sans-serif;text-transform:uppercase;letter-spacing:1px;transition:background 0.15s;';
    btn.addEventListener('mouseenter', function () { btn.style.background = '#45475a'; });
    btn.addEventListener('mouseleave', function () { btn.style.background = '#313244'; });
    btn.addEventListener('click', onClick);
    return btn;
  }

  if ('Notification' in window && Notification.permission === 'default') {
    Notification.requestPermission();
  }

  render();
})();
