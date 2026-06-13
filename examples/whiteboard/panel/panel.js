// whiteboard/panel/panel.js
// Runs inside the plugin's own dia process. Config is embedded
// directly in the HTML as window.__diaPluginConfig so no Wails
// bridge call is needed.
(function () {
  var root = document.getElementById('root');
  if (!root) return;

  // ── read config from HTML (set by Go via generatedPanelHTML) ──
  var cfg = window.__diaPluginConfig || {};

  var colors = ['#000000', '#1e66f5', '#40a02b', '#df8e1d', '#d20f39', '#8839ef'];
  var cfgColor = cfg.color || '#000000';
  var cfgWidth = cfg.width || 3;
  var cfgTheme = cfg.theme || 'light';

  var toolbar = document.createElement('div');
  toolbar.style.cssText = [
    'display:flex',
    'align-items:center',
    'gap:8px',
    'padding:8px 12px',
    'background:#181825',
    'color:#cdd6f4',
    'border-bottom:1px solid #313244',
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
        'width:22px',
        'height:22px',
        'border-radius:50%',
        'background:' + c,
        'border:2px solid ' + (c === cfgColor ? '#cdd6f4' : 'transparent'),
        'cursor:pointer',
        'padding:0',
      ].join(';');
      sw.addEventListener('click', function () {
        cfgColor = c;
        for (var si = 0; si < colorRow.children.length; si++) {
          colorRow.children[si].style.border = '2px solid ' +
            (colorRow.children[si].title === c ? '#cdd6f4' : 'transparent');
        }
      });
      colorRow.appendChild(sw);
    })(colors[ci]);
  }
  toolbar.appendChild(colorRow);

  var widthLabel = document.createElement('span');
  widthLabel.textContent = 'thickness: ' + cfgWidth;
  toolbar.appendChild(widthLabel);
  var slider = document.createElement('input');
  slider.type = 'range';
  slider.min = '1';
  slider.max = '30';
  slider.value = String(cfgWidth);
  slider.style.cssText = 'accent-color:#89b4fa';
  slider.addEventListener('input', function () {
    cfgWidth = Number(slider.value);
    widthLabel.textContent = 'thickness: ' + cfgWidth;
  });
  toolbar.appendChild(slider);

  var spacer = document.createElement('div');
  spacer.style.flex = '1';
  toolbar.appendChild(spacer);

  var clearBtn = document.createElement('button');
  clearBtn.type = 'button';
  clearBtn.textContent = 'clear';
  clearBtn.style.cssText = [
    'padding:4px 12px',
    'background:#313244',
    'color:#cdd6f4',
    'border:0',
    'border-radius:6px',
    'cursor:pointer',
    'font:13px -apple-system,Segoe UI,sans-serif',
  ].join(';');
  toolbar.appendChild(clearBtn);

  var status = document.createElement('span');
  status.style.cssText = 'color:#a6adc8;font-size:11px';
  status.textContent = '0 strokes';
  toolbar.appendChild(status);

  function canvasBg() { return cfgTheme === 'dark' ? '#1e1e2e' : '#ffffff'; }

  var canvas = document.createElement('canvas');
  canvas.style.cssText = 'flex:1;display:block;background:' + canvasBg() + ';cursor:crosshair;touch-action:none';
  root.appendChild(canvas);
  var ctx = canvas.getContext('2d');

  var strokes = [];
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
    ctx.fillStyle = canvasBg();
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
      var p = s.points[pi];
      ctx.lineTo(p.x, p.y);
    }
    if (s.points.length === 1) ctx.lineTo(first.x + 0.01, first.y + 0.01);
    ctx.stroke();
  }

  function pt(ev) {
    var r = canvas.getBoundingClientRect();
    return { x: ev.clientX - r.left, y: ev.clientY - r.top };
  }

  canvas.addEventListener('pointerdown', function (ev) {
    canvas.setPointerCapture(ev.pointerId);
    current = { color: cfgColor, width: cfgWidth, points: [pt(ev)] };
    strokes.push(current);
    status.textContent = strokes.length + ' stroke' + (strokes.length === 1 ? '' : 's');
  });
  canvas.addEventListener('pointermove', function (ev) {
    if (!current) return;
    var p = pt(ev);
    current.points.push(p);
    drawSegment(current.points[current.points.length - 2], p, current.color, current.width);
  });
  function drawSegment(a, b, color, width) {
    ctx.strokeStyle = color;
    ctx.lineWidth = width;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.beginPath();
    ctx.moveTo(a.x, a.y);
    ctx.lineTo(b.x, b.y);
    ctx.stroke();
  }
  canvas.addEventListener('pointerup', function (ev) {
    current = null;
    try { canvas.releasePointerCapture(ev.pointerId); } catch (e) {}
  });
  canvas.addEventListener('pointercancel', function () { current = null; });
  canvas.addEventListener('pointerleave', function () { current = null; });

  clearBtn.addEventListener('click', function () {
    strokes.length = 0;
    status.textContent = '0 strokes';
    redraw();
  });

  resize();
})();
