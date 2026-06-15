(function () {
  var cfg = window.__diaPluginConfig || {};
  var fontSize = cfg.font_size || 14;
  var wrap = cfg.wrap !== false;

  var STORAGE_KEY = 'dia-quick-notes';

  var root = document.getElementById('root');
  if (!root) return;

  var bgColor = '#1e1e2e';
  var textColor = '#cdd6f4';
  var mutedColor = '#585b70';
  var accentColor = '#89b4fa';
  var borderColor = '#313244';

  function loadNotes() {
    try { return localStorage.getItem(STORAGE_KEY) || ''; } catch (e) { return ''; }
  }

  function saveNotes(content) {
    try { localStorage.setItem(STORAGE_KEY, content); } catch (e) {}
  }

  function exportNotes(content) {
    var blob = new Blob([content], { type: 'text/plain' });
    var url = URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = 'notes.txt';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  function render() {
    root.innerHTML = '';
    root.style.cssText = 'display:flex;flex-direction:column;height:100%;background:' + bgColor + ';color:' + textColor + ';font-family:-apple-system,Segoe UI,sans-serif;';

    var toolbar = document.createElement('div');
    toolbar.style.cssText = 'display:flex;align-items:center;gap:8px;padding:6px 12px;background:#181825;border-bottom:1px solid ' + borderColor + ';';

    var title = document.createElement('span');
    title.style.cssText = 'font-size:12px;font-weight:600;color:' + mutedColor + ';';
    title.textContent = 'Notes';
    toolbar.appendChild(title);

    toolbar.appendChild(document.createElement('div'));

    var spacer = document.createElement('div');
    spacer.style.flex = '1';
    toolbar.appendChild(spacer);

    var saveBtn = toolButton('save');
    saveBtn.addEventListener('click', function () {
      saveNotes(textarea.value);
      showToast('Saved');
    });
    toolbar.appendChild(saveBtn);

    var exportBtn = toolButton('export');
    exportBtn.addEventListener('click', function () {
      saveNotes(textarea.value);
      exportNotes(textarea.value);
    });
    toolbar.appendChild(exportBtn);

    var clearBtn = toolButton('clear');
    clearBtn.style.color = '#f38ba8';
    clearBtn.addEventListener('click', function () {
      if (confirm('Clear all notes?')) {
        textarea.value = '';
        saveNotes('');
        showToast('Cleared');
      }
    });
    toolbar.appendChild(clearBtn);

    root.appendChild(toolbar);

    var textarea = document.createElement('textarea');
    textarea.value = loadNotes();
    textarea.style.cssText = [
      'flex:1', 'width:100%', 'box-sizing:border-box', 'resize:none',
      'background:' + bgColor, 'color:' + textColor,
      'border:0', 'outline:none', 'padding:16px',
      'font-size:' + fontSize + 'px',
      'font-family:"SF Mono","Cascadia Code","Fira Code","Consolas",monospace',
      'line-height:1.6',
      'white-space:' + (wrap ? 'pre-wrap' : 'pre'),
      'overflow-wrap:' + (wrap ? 'break-word' : 'normal'),
    ].join(';');
    textarea.addEventListener('input', function () {
      saveNotes(textarea.value);
    });
    root.appendChild(textarea);

    textarea.focus();
  }

  function toolButton(label) {
    var btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = label;
    btn.style.cssText = 'padding:4px 10px;background:#313244;color:' + textColor + ';border:0;border-radius:4px;cursor:pointer;font:11px -apple-system,Segoe UI,sans-serif;transition:background 0.15s;';
    btn.addEventListener('mouseenter', function () { btn.style.background = '#45475a'; });
    btn.addEventListener('mouseleave', function () { btn.style.background = '#313244'; });
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
})();
