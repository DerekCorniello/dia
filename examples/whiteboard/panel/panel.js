// whiteboard/panel/panel.js
// Runs inside the plugin's own dia process. The host injects
// window.dia (see internal/wailsapp/pluginwindow.go -> generatedDiaJS).
// This file uses no Svelte: it is plain browser JS so plugin authors
// do not need to learn any framework.
(function () {
  const root = document.getElementById('root');
  if (!root) return;

  const toolbar = document.createElement('div');
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

  const colors = ['#000000', '#1e66f5', '#40a02b', '#df8e1d', '#d20f39', '#8839ef'];
  let color = colors[0];
  let width = 3;
  const colorRow = document.createElement('div');
  colorRow.style.cssText = 'display:flex;gap:6px;align-items:center';
  for (const c of colors) {
    const sw = document.createElement('button');
    sw.type = 'button';
    sw.title = c;
    sw.style.cssText = [
      'width:22px',
      'height:22px',
      'border-radius:50%',
      `background:${c}`,
      `border:2px solid ${c === color ? '#cdd6f4' : 'transparent'}`,
      'cursor:pointer',
      'padding:0',
    ].join(';');
    sw.addEventListener('click', () => {
      color = c;
      for (const sib of colorRow.children) {
        sib.style.border = `2px solid ${sib.title === c ? '#cdd6f4' : 'transparent'}`;
      }
    });
    colorRow.appendChild(sw);
  }
  toolbar.appendChild(colorRow);

  const widthLabel = document.createElement('span');
  widthLabel.textContent = 'thickness: 3';
  toolbar.appendChild(widthLabel);
  const slider = document.createElement('input');
  slider.type = 'range';
  slider.min = '1';
  slider.max = '30';
  slider.value = '3';
  slider.style.cssText = 'accent-color:#89b4fa';
  slider.addEventListener('input', () => {
    width = Number(slider.value);
    widthLabel.textContent = `thickness: ${width}`;
  });
  toolbar.appendChild(slider);

  const spacer = document.createElement('div');
  spacer.style.flex = '1';
  toolbar.appendChild(spacer);

  const clearBtn = document.createElement('button');
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

  const status = document.createElement('span');
  status.style.cssText = 'color:#a6adc8;font-size:11px';
  status.textContent = '0 strokes';
  toolbar.appendChild(status);

  const canvas = document.createElement('canvas');
  canvas.style.cssText = 'flex:1;display:block;background:#ffffff;cursor:crosshair;touch-action:none';
  root.appendChild(canvas);
  const ctx = canvas.getContext('2d');

  const strokes = [];
  let current = null;

  function resize() {
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth;
    const h = canvas.clientHeight;
    canvas.width = Math.floor(w * dpr);
    canvas.height = Math.floor(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    redraw();
  }
  const ro = new ResizeObserver(resize);
  ro.observe(canvas);

  function redraw() {
    const w = canvas.clientWidth;
    const h = canvas.clientHeight;
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, w, h);
    for (const s of strokes) drawStroke(s);
  }

  function drawStroke(s) {
    if (s.points.length === 0) return;
    ctx.strokeStyle = s.color;
    ctx.lineWidth = s.width;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.beginPath();
    const first = s.points[0];
    ctx.moveTo(first.x, first.y);
    for (let i = 1; i < s.points.length; i++) {
      const p = s.points[i];
      ctx.lineTo(p.x, p.y);
    }
    if (s.points.length === 1) ctx.lineTo(first.x + 0.01, first.y + 0.01);
    ctx.stroke();
  }

  function pt(ev) {
    const r = canvas.getBoundingClientRect();
    return { x: ev.clientX - r.left, y: ev.clientY - r.top };
  }

  canvas.addEventListener('pointerdown', (ev) => {
    canvas.setPointerCapture(ev.pointerId);
    current = { color, width, points: [pt(ev)] };
    strokes.push(current);
    status.textContent = `${strokes.length} stroke${strokes.length === 1 ? '' : 's'}`;
  });
  canvas.addEventListener('pointermove', (ev) => {
    if (!current) return;
    const p = pt(ev);
    const prev = current.points[current.points.length - 1];
    current.points.push(p);
    drawSegment(prev, p, current.color, current.width);
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
  canvas.addEventListener('pointerup', (ev) => {
    current = null;
    try { canvas.releasePointerCapture(ev.pointerId); } catch (e) {}
  });
  canvas.addEventListener('pointercancel', () => { current = null; });
  canvas.addEventListener('pointerleave', () => { current = null; });

  clearBtn.addEventListener('click', () => {
    strokes.length = 0;
    status.textContent = '0 strokes';
    redraw();
  });

  resize();
  if (window.dia && typeof window.dia.capabilities === 'function') {
    window.dia.capabilities().then(() => { /* dia host reachable */ }).catch(() => {});
  }
})();
