(function () {
  var root = document.getElementById('root');
  if (!root) return;

  var cfg = window.__diaPluginConfig || {};
  var colors = ['#000000', '#1e66f5', '#40a02b', '#df8e1d', '#d20f39', '#8839ef'];
  var cfgColor = cfg.color || '#000000';
  var cfgWidth = cfg.width || 3;
  var cfgTheme = cfg.theme || 'light';

  var theme = {
    light: { toolbarBg: '#f0f0f0', toolbarText: '#333', border: '#ddd', btnBg: '#e0e0e0', btnText: '#333', accent: '#555', canvasBg: '#ffffff', strokeBg: '#ffffff' },
    dark: { toolbarBg: '#181825', toolbarText: '#cdd6f4', border: '#313244', btnBg: '#313244', btnText: '#cdd6f4', accent: '#89b4fa', canvasBg: '#1e1e2e', strokeBg: '#1e1e2e' }
  };
  function t(k) { return (theme[cfgTheme] || theme.light)[k]; }

  var toolbar = document.createElement('div');
  toolbar.style.cssText = [
    'display:flex', 'align-items:center', 'gap:8px', 'padding:8px 12px',
    'background:' + t('toolbarBg'), 'color:' + t('toolbarText'),
    'border-bottom:1px solid ' + t('border'),
    'font:13px -apple-system,Segoe UI,sans-serif',
  ].join(';');
  root.appendChild(toolbar);

  var colorRow = document.createElement('div');
  colorRow.style.cssText = 'display:flex;gap:6px;align-items:center';
  for (var ci = 0; ci < colors.length; ci++) {
    (function (c) {
      var sw = document.createElement('button');
      sw.type = 'button';
      sw.title = c;
      sw.style.cssText = [
        'width:22px', 'height:22px', 'border-radius:50%', 'background:' + c,
        'border:2px solid ' + (c === cfgColor ? t('toolbarText') : 'transparent'),
        'cursor:pointer', 'padding:0',
      ].join(';');
      sw.addEventListener('click', function () {
        cfgColor = c;
        for (var si = 0; si < colorRow.children.length; si++) {
          colorRow.children[si].style.border = '2px solid ' +
            (colorRow.children[si].title === c ? t('toolbarText') : 'transparent');
        }
      });
      colorRow.appendChild(sw);
    })(colors[ci]);
  }
  toolbar.appendChild(colorRow);

  var widthLabel = document.createElement('span');
  widthLabel.textContent = 'thickness: ' + cfgWidth;
  widthLabel.style.fontSize = '11px';
  toolbar.appendChild(widthLabel);
  var slider = document.createElement('input');
  slider.type = 'range';
  slider.min = '1';
  slider.max = '30';
  slider.value = String(cfgWidth);
  slider.style.cssText = 'accent-color:' + t('accent') + ';width:80px';
  slider.addEventListener('input', function () {
    cfgWidth = Number(slider.value);
    widthLabel.textContent = 'thickness: ' + cfgWidth;
  });
  toolbar.appendChild(slider);

  toolbar.appendChild(document.createTextNode(' '));

  var eraserBtn = document.createElement('button');
  eraserBtn.type = 'button';
  eraserBtn.textContent = 'eraser';
  eraserBtn.title = 'Toggle eraser mode';
  eraserBtn.style.cssText = [
    'padding:4px 8px', 'background:' + t('btnBg'), 'color:' + t('btnText'),
    'border:1px solid transparent', 'border-radius:6px', 'cursor:pointer',
    'font:11px -apple-system,Segoe UI,sans-serif',
  ].join(';');
  var erasing = false;
  eraserBtn.addEventListener('click', function () {
    erasing = !erasing;
    eraserBtn.style.border = erasing ? '1px solid ' + t('accent') : '1px solid transparent';
    eraserBtn.style.background = erasing ? t('accent') : t('btnBg');
    eraserBtn.style.color = erasing ? '#fff' : t('btnText');
  });
  toolbar.appendChild(eraserBtn);

  var spacer = document.createElement('div');
  spacer.style.flex = '1';
  toolbar.appendChild(spacer);

  var undoBtn = document.createElement('button');
  undoBtn.type = 'button';
  undoBtn.textContent = 'undo';
  undoBtn.style.cssText = [
    'padding:4px 8px', 'background:' + t('btnBg'), 'color:' + t('btnText'),
    'border:1px solid transparent', 'border-radius:6px', 'cursor:pointer',
    'font:11px -apple-system,Segoe UI,sans-serif',
  ].join(';');
  toolbar.appendChild(undoBtn);

  var redoBtn = document.createElement('button');
  redoBtn.type = 'button';
  redoBtn.textContent = 'redo';
  redoBtn.style.cssText = [
    'padding:4px 8px', 'background:' + t('btnBg'), 'color:' + t('btnText'),
    'border:1px solid transparent', 'border-radius:6px', 'cursor:pointer',
    'font:11px -apple-system,Segoe UI,sans-serif',
  ].join(';');
  toolbar.appendChild(redoBtn);

  toolbar.appendChild(document.createTextNode(' '));

  var exportBtn = document.createElement('button');
  exportBtn.type = 'button';
  exportBtn.textContent = 'download';
  exportBtn.style.cssText = [
    'padding:4px 8px', 'background:' + t('btnBg'), 'color:' + t('btnText'),
    'border:0', 'border-radius:6px', 'cursor:pointer',
    'font:11px -apple-system,Segoe UI,sans-serif',
  ].join(';');
  toolbar.appendChild(exportBtn);

  toolbar.appendChild(document.createTextNode(' '));

  var clearBtn = document.createElement('button');
  clearBtn.type = 'button';
  clearBtn.textContent = 'clear';
  clearBtn.style.cssText = [
    'padding:4px 8px', 'background:' + t('btnBg'), 'color:' + t('btnText'),
    'border:0', 'border-radius:6px', 'cursor:pointer',
    'font:11px -apple-system,Segoe UI,sans-serif',
  ].join(';');
  toolbar.appendChild(clearBtn);

  var status = document.createElement('span');
  status.style.cssText = 'color:' + t('toolbarText') + ';font-size:11px;opacity:0.6';
  status.textContent = '0 strokes';
  toolbar.appendChild(status);

  var canvas = document.createElement('canvas');
  canvas.style.cssText = 'flex:1;display:block;background:' + t('canvasBg') + ';cursor:crosshair;touch-action:none';
  root.appendChild(canvas);
  var ctx = canvas.getContext('2d');

  var strokes = [];
  var undoStack = [];
  var redoStack = [];
  var current = null;

  function resize() {
    var dpr = window.devicePixelRatio || 1;
    var w = canvas.clientWidth;
    var h = canvas.clientHeight;
    canvas.width = Math.floor(w * dpr);
    canvas.height = Math.floor(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    redraw();
  }
  var ro = new ResizeObserver(resize);
  ro.observe(canvas);

  function redraw() {
    var w = canvas.clientWidth;
    var h = canvas.clientHeight;
    ctx.fillStyle = t('canvasBg');
    ctx.fillRect(0, 0, w, h);
    for (var si = 0; si < strokes.length; si++) drawStroke(strokes[si]);
  }

  function drawStroke(s) {
    if (s.points.length === 0) return;
    ctx.strokeStyle = s.color;
    ctx.lineWidth = s.width;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.beginPath();
    var first = s.points[0];
    ctx.moveTo(first.x, first.y);
    for (var pi = 1; pi < s.points.length; pi++) {
      ctx.lineTo(s.points[pi].x, s.points[pi].y);
    }
    if (s.points.length === 1) ctx.lineTo(first.x + 0.01, first.y + 0.01);
    ctx.stroke();
  }

  function pt(ev) {
    var r = canvas.getBoundingClientRect();
    return { x: ev.clientX - r.left, y: ev.clientY - r.top };
  }

  function saveUndo() {
    undoStack.push(JSON.parse(JSON.stringify(strokes)));
    redoStack = [];
    updateStatus();
  }

  function updateStatus() {
    status.textContent = strokes.length + ' stroke' + (strokes.length === 1 ? '' : 's');
    undoBtn.style.opacity = undoStack.length > 0 ? '1' : '0.3';
    redoBtn.style.opacity = redoStack.length > 0 ? '1' : '0.3';
  }

  canvas.addEventListener('pointerdown', function (ev) {
    canvas.setPointerCapture(ev.pointerId);
    var drawColor = erasing ? t('strokeBg') : cfgColor;
    current = { color: drawColor, width: erasing ? Math.max(cfgWidth, 10) : cfgWidth, points: [pt(ev)] };
    strokes.push(current);
    updateStatus();
  });
  canvas.addEventListener('pointermove', function (ev) {
    if (!current) return;
    var p = pt(ev);
    current.points.push(p);
    var seg = current.points;
    ctx.strokeStyle = current.color;
    ctx.lineWidth = current.width;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.beginPath();
    ctx.moveTo(seg[seg.length - 2].x, seg[seg.length - 2].y);
    ctx.lineTo(p.x, p.y);
    ctx.stroke();
  });
  canvas.addEventListener('pointerup', function () {
    if (current) { saveUndo(); current = null; }
    try { canvas.releasePointerCapture(1); } catch (e) {}
  });
  canvas.addEventListener('pointercancel', function () { if (current) { saveUndo(); current = null; } });
  canvas.addEventListener('pointerleave', function () { if (current) { saveUndo(); current = null; } });

  undoBtn.addEventListener('click', function () {
    if (undoStack.length === 0) return;
    redoStack.push(JSON.parse(JSON.stringify(strokes)));
    strokes = undoStack.pop();
    redraw();
    updateStatus();
  });

  redoBtn.addEventListener('click', function () {
    if (redoStack.length === 0) return;
    undoStack.push(JSON.parse(JSON.stringify(strokes)));
    strokes = redoStack.pop();
    redraw();
    updateStatus();
  });

  clearBtn.addEventListener('click', function () {
    if (strokes.length === 0) return;
    saveUndo();
    strokes = [];
    redraw();
    updateStatus();
  });

  exportBtn.addEventListener('click', function () {
    var link = document.createElement('a');
    link.download = 'whiteboard.png';
    link.href = canvas.toDataURL('image/png');
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  });

  window.addEventListener('keydown', function (e) {
    if (e.ctrlKey && e.key === 'z' && !e.shiftKey) { e.preventDefault(); undoBtn.click(); }
    if (e.ctrlKey && e.key === 'z' && e.shiftKey) { e.preventDefault(); redoBtn.click(); }
    if (e.ctrlKey && e.key === 'y') { e.preventDefault(); redoBtn.click(); }
  });

  resize();
  updateStatus();
})();
