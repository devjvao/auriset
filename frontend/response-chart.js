const frequencyMin = 20;
const frequencyMax = 20000;
const gainFormat = new Intl.NumberFormat('pt-BR', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
const kilohertzFormat = new Intl.NumberFormat('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
export const formatGain = gain => `${gainFormat.format(gain).replace('-', '−')} dB`;

function formatFrequency(frequency) {
  if (frequency < 1000) return `${Math.round(frequency)} Hz`;
  return `${kilohertzFormat.format(frequency / 1000)} kHz`;
}

function chartBounds(presets) {
  let min = Infinity;
  let max = -Infinity;
  for (const { curve, target } of presets) {
    for (const points of [curve, target]) {
      for (const { frequency, db } of points) {
        if (frequency > 16000) continue;
        min = Math.min(min, db);
        max = Math.max(max, db);
      }
    }
  }
  const low = Math.max(Math.floor(min / 5) * 5, -20);
  const high = Math.ceil(max / 5) * 5;
  const pad = Math.max(0, Math.ceil((40 - (high - low)) / 10) * 5);
  return { low: low - pad, high: high + pad };
}

function curvePath(curve, toX, toY) {
  const path = new Path2D();
  curve.forEach(({ frequency, db }, index) => {
    const x = toX(frequency);
    const y = toY(db);
    if (index === 0) path.moveTo(x, y);
    else path.lineTo(x, y);
  });
  return path;
}

export function createResponseChart(canvas, reading) {
  const chartStyles = getComputedStyle(canvas);
  const chartColor = name => chartStyles.getPropertyValue(name).trim();
  const chartColors = {
    response: chartColor('--gold'),
    reference: chartColor('--plot-target'),
    label: chartColor('--muted'),
    grid: chartColor('--plot-grid'),
    zero: chartColor('--plot-zero'),
    guide: chartColor('--plot-guide'),
    fillStart: chartColor('--plot-fill-start'),
    fillEnd: chartColor('--plot-fill-end'),
  };

  let selected;
  let bounds = { low: -25, high: 20 };
  let hoverFrequency;

  function chartLayout() {
    const width = canvas.clientWidth;
    const height = canvas.clientHeight;
    return { width, height, left: 37, right: width - 12, top: 9, bottom: height - 22 };
  }

  function drawChart() {
    const { width, height, left, right, top, bottom } = chartLayout();
    if (right <= left || bottom <= top) return;
    const scale = window.devicePixelRatio || 1;
    canvas.width = Math.round(width * scale);
    canvas.height = Math.round(height * scale);
    const g = canvas.getContext('2d');
    if (!g) return;
    g.scale(scale, scale);
    const toX = frequency => left + (right - left) * Math.log(frequency / frequencyMin) / Math.log(frequencyMax / frequencyMin);
    const toY = gain => top + (bottom - top) * (bounds.high - gain) / (bounds.high - bounds.low);
    let ticks = [20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 20000];
    if (width < 350) ticks = [20, 100, 1000, 5000, 20000];
    else if (width < 540) ticks = [20, 100, 500, 1000, 5000, 20000];

    g.font = '11px Consolas, monospace';
    g.fillStyle = chartColors.label;
    g.textAlign = 'right';
    g.textBaseline = 'middle';
    for (let gain = bounds.low; gain <= bounds.high; gain += 5) {
      const y = toY(gain);
      g.strokeStyle = gain === 0 ? chartColors.zero : chartColors.grid;
      g.beginPath();
      g.moveTo(left, y);
      g.lineTo(right, y);
      g.stroke();
      g.fillText(gain === 0 ? '0 dB' : `${gain > 0 ? '+' : ''}${gain}`, left - 8, y);
    }
    g.textBaseline = 'top';
    for (const frequency of ticks) {
      const x = toX(frequency);
      g.strokeStyle = chartColors.grid;
      g.beginPath();
      g.moveTo(x, top);
      g.lineTo(x, bottom);
      g.stroke();
      g.textAlign = frequency === frequencyMin ? 'left' : frequency === frequencyMax ? 'right' : 'center';
      const label = frequency === frequencyMax ? '20 kHz' : frequency < 1000 ? String(frequency) : `${frequency / 1000}k`;
      g.fillText(label, x, bottom + 7);
    }

    reading.textContent = '';
    if (!selected) return;
    const { curve, target } = selected;
    const trace = curvePath(curve, toX, toY);
    g.save();
    g.beginPath();
    g.rect(left, top, right - left, bottom - top);
    g.clip();
    g.lineJoin = 'round';
    g.lineCap = 'round';
    g.lineWidth = 3;
    g.strokeStyle = chartColors.reference;
    g.stroke(curvePath(target, toX, toY));
    const area = new Path2D(trace);
    area.lineTo(right, bottom);
    area.lineTo(left, bottom);
    area.closePath();
    const fill = g.createLinearGradient(0, top, 0, bottom);
    fill.addColorStop(0, chartColors.fillStart);
    fill.addColorStop(1, chartColors.fillEnd);
    g.fillStyle = fill;
    g.fill(area);
    g.lineWidth = 2;
    g.strokeStyle = chartColors.response;
    g.stroke(trace);

    if (hoverFrequency !== undefined) {
      let closestIndex = 0;
      let smallestDistance = Infinity;
      for (const [index, { frequency }] of curve.entries()) {
        const distance = Math.abs(Math.log(frequency / hoverFrequency));
        if (distance >= smallestDistance) continue;
        closestIndex = index;
        smallestDistance = distance;
      }
      const response = curve[closestIndex];
      const reference = target[closestIndex];
      const x = toX(response.frequency);
      g.lineWidth = 1;
      g.strokeStyle = chartColors.guide;
      g.beginPath();
      g.moveTo(x, top);
      g.lineTo(x, bottom);
      g.stroke();
      for (const [point, color] of [[reference, chartColors.reference], [response, chartColors.response]]) {
        g.fillStyle = color;
        g.beginPath();
        g.arc(x, toY(point.db), 4, 0, 2 * Math.PI);
        g.fill();
      }
      reading.textContent = `${formatFrequency(response.frequency)} · Auri ${formatGain(response.db)} · Alvo ${formatGain(reference.db)}`;
    }
    g.restore();
  }

  canvas.addEventListener('pointermove', event => {
    const { left, right } = chartLayout();
    const fraction = (event.offsetX - left) / (right - left);
    hoverFrequency = fraction >= 0 && fraction <= 1 ? frequencyMin * (frequencyMax / frequencyMin) ** fraction : undefined;
    drawChart();
  });
  canvas.addEventListener('pointerleave', () => {
    hoverFrequency = undefined;
    drawChart();
  });
  new ResizeObserver(drawChart).observe(canvas);

  return {
    setPresets(presets) {
      bounds = chartBounds(presets);
    },
    setPreset(preset) {
      selected = preset;
      canvas.setAttribute('aria-label', `Resposta em frequência estimada do Auri com o preset ${preset.name} e o alvo ${preset.targetName}, em decibéis relativos`);
      drawChart();
    },
  };
}
