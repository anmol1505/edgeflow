package observability

import (
	"fmt"
	"net/http"
)

func Dashboard() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, dashboardHTML)
	})
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>EdgeFlow Dashboard</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { background: #0f1117; color: #e2e8f0; font-family: 'Segoe UI', sans-serif; padding: 24px; }
  h1 { font-size: 28px; font-weight: 700; color: #60a5fa; margin-bottom: 4px; }
  .subtitle { color: #64748b; font-size: 14px; margin-bottom: 32px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; margin-bottom: 32px; }
  .card { background: #1e2130; border-radius: 12px; padding: 20px; border: 1px solid #2d3748; }
  .card-label { font-size: 12px; color: #64748b; text-transform: uppercase; letter-spacing: 1px; margin-bottom: 8px; }
  .card-value { font-size: 32px; font-weight: 700; color: #60a5fa; }
  .card-value.green { color: #34d399; }
  .card-value.red { color: #f87171; }
  .card-value.yellow { color: #fbbf24; }
  .section { background: #1e2130; border-radius: 12px; padding: 24px; border: 1px solid #2d3748; margin-bottom: 16px; }
  .section h2 { font-size: 16px; color: #94a3b8; margin-bottom: 16px; text-transform: uppercase; letter-spacing: 1px; }
  .bar-row { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; }
  .bar-label { width: 80px; font-size: 13px; color: #94a3b8; }
  .bar-bg { flex: 1; background: #2d3748; border-radius: 4px; height: 8px; }
  .bar-fill { height: 8px; border-radius: 4px; transition: width 0.5s ease; }
  .bar-fill.hit { background: #34d399; }
  .bar-fill.miss { background: #f87171; }
  .bar-fill.stale { background: #fbbf24; }
  .bar-count { width: 40px; text-align: right; font-size: 13px; color: #64748b; }
  .origin-row { display: flex; justify-content: space-between; align-items: center; padding: 10px 0; border-bottom: 1px solid #2d3748; }
  .origin-row:last-child { border-bottom: none; }
  .badge { padding: 3px 10px; border-radius: 999px; font-size: 12px; font-weight: 600; }
  .badge.healthy { background: #064e3b; color: #34d399; }
  .badge.unhealthy { background: #450a0a; color: #f87171; }
  .badge.closed { background: #064e3b; color: #34d399; }
  .badge.open { background: #450a0a; color: #f87171; }
  .badge.half-open { background: #422006; color: #fbbf24; }
  .latency-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
  .latency-card { background: #0f1117; border-radius: 8px; padding: 14px; text-align: center; }
  .latency-label { font-size: 11px; color: #64748b; margin-bottom: 6px; }
  .latency-value { font-size: 22px; font-weight: 700; color: #a78bfa; }
  .updated { font-size: 11px; color: #374151; text-align: right; margin-top: 16px; }
  .logo { display: inline-block; background: #1d4ed8; color: white; padding: 2px 10px; border-radius: 6px; font-size: 12px; font-weight: 700; margin-left: 10px; vertical-align: middle; }
</style>
</head>
<body>
<h1>EdgeFlow <span class="logo">LIVE</span></h1>
<p class="subtitle">Edge Proxy · CDN · Load Balancer · Observability</p>

<div class="grid">
  <div class="card">
    <div class="card-label">Total Requests</div>
    <div class="card-value" id="total-requests">—</div>
  </div>
  <div class="card">
    <div class="card-label">Cache Hit Rate</div>
    <div class="card-value green" id="hit-rate">—</div>
  </div>
  <div class="card">
    <div class="card-label">Active Connections</div>
    <div class="card-value" id="active-conn">—</div>
  </div>
  <div class="card">
    <div class="card-label">Rate Limited</div>
    <div class="card-value red" id="rate-limited">—</div>
  </div>
</div>

<div class="section">
  <h2>Cache Performance</h2>
  <div class="bar-row">
    <div class="bar-label">HIT</div>
    <div class="bar-bg"><div class="bar-fill hit" id="bar-hit" style="width:0%"></div></div>
    <div class="bar-count" id="count-hit">0</div>
  </div>
  <div class="bar-row">
    <div class="bar-label">MISS</div>
    <div class="bar-bg"><div class="bar-fill miss" id="bar-miss" style="width:0%"></div></div>
    <div class="bar-count" id="count-miss">0</div>
  </div>
  <div class="bar-row">
    <div class="bar-label">STALE</div>
    <div class="bar-bg"><div class="bar-fill stale" id="bar-stale" style="width:0%"></div></div>
    <div class="bar-count" id="count-stale">0</div>
  </div>
</div>

<div class="section">
  <h2>Latency (p50 / p95 / p99)</h2>
  <div class="latency-grid">
    <div class="latency-card">
      <div class="latency-label">p50 median</div>
      <div class="latency-value" id="p50">—</div>
    </div>
    <div class="latency-card">
      <div class="latency-label">p95</div>
      <div class="latency-value" id="p95">—</div>
    </div>
    <div class="latency-card">
      <div class="latency-label">p99</div>
      <div class="latency-value" id="p99">—</div>
    </div>
  </div>
</div>

<div class="section">
  <h2>Origin Health</h2>
  <div id="origins-list"><div style="color:#64748b">Loading...</div></div>
</div>

<div class="section">
  <h2>Circuit Breaker</h2>
  <div id="cb-state"><div style="color:#64748b">Loading...</div></div>
</div>

<div class="updated" id="updated">Refreshing every 2s...</div>

<script>
async function fetchMetrics() {
  const res = await fetch('/metrics');
  return res.text();
}

async function fetchHealth() {
  const res = await fetch('/health');
  return res.json();
}

function parseMetric(text, name) {
  const lines = text.split('\n');
  let total = 0;
  for (const line of lines) {
    if (line.startsWith(name) && !line.startsWith('#')) {
      const val = parseFloat(line.split(' ').pop());
      if (!isNaN(val)) total += val;
    }
  }
  return total;
}

function parseMetricLabel(text, name, label) {
  const lines = text.split('\n');
  for (const line of lines) {
    if (line.startsWith(name) && line.includes(label) && !line.startsWith('#')) {
      return parseFloat(line.split(' ').pop()) || 0;
    }
  }
  return 0;
}

function parsePercentile(text, metricName, pct) {
  const lines = text.split('\n');
  const buckets = [];
  let count = 0;
  for (const line of lines) {
    if (line.startsWith(metricName + '_bucket') && !line.startsWith('#')) {
      const m = line.match(/le="([^"]+)"/);
      const v = parseFloat(line.split(' ').pop());
      if (m) buckets.push({ le: m[1] === '+Inf' ? Infinity : parseFloat(m[1]), count: v });
    }
    if (line.startsWith(metricName + '_count') && !line.startsWith('#')) {
      count = parseFloat(line.split(' ').pop());
    }
  }
  if (!count || !buckets.length) return null;
  const target = pct * count;
  for (const b of buckets) { if (b.count >= target) return b.le; }
  return null;
}

function fmt(v) {
  if (v === null) return '—';
  if (v < 0.001) return '<1ms';
  return (v * 1000).toFixed(1) + 'ms';
}

async function update() {
  try {
    const [metrics, health] = await Promise.all([fetchMetrics(), fetchHealth()]);
    const total = parseMetric(metrics, 'edgeflow_requests_total');
    const hits = parseMetricLabel(metrics, 'edgeflow_cache_hits_total', 'status="HIT"');
    const misses = parseMetricLabel(metrics, 'edgeflow_cache_hits_total', 'status="MISS"');
    const stales = parseMetricLabel(metrics, 'edgeflow_cache_hits_total', 'status="STALE"');
    const rateLimited = parseMetric(metrics, 'edgeflow_rate_limited_total');
    const cacheTotal = hits + misses + stales || 1;
    const hitRate = ((hits / cacheTotal) * 100).toFixed(1);

    document.getElementById('total-requests').textContent = total;
    document.getElementById('hit-rate').textContent = hitRate + '%';
    document.getElementById('active-conn').textContent = parseMetric(metrics, 'edgeflow_active_connections');
    document.getElementById('rate-limited').textContent = rateLimited;

    document.getElementById('bar-hit').style.width = (hits / cacheTotal * 100) + '%';
    document.getElementById('bar-miss').style.width = (misses / cacheTotal * 100) + '%';
    document.getElementById('bar-stale').style.width = (stales / cacheTotal * 100) + '%';
    document.getElementById('count-hit').textContent = hits;
    document.getElementById('count-miss').textContent = misses;
    document.getElementById('count-stale').textContent = stales;

    document.getElementById('p50').textContent = fmt(parsePercentile(metrics, 'edgeflow_request_duration_seconds', 0.5));
    document.getElementById('p95').textContent = fmt(parsePercentile(metrics, 'edgeflow_request_duration_seconds', 0.95));
    document.getElementById('p99').textContent = fmt(parsePercentile(metrics, 'edgeflow_request_duration_seconds', 0.99));

    const origins = health.healthy_origins || [];
    document.getElementById('origins-list').innerHTML = origins.length
      ? origins.map(function(o) { return '<div class="origin-row"><span>' + o + '</span><span class="badge healthy">healthy</span></div>'; }).join('')
      : '<div class="origin-row"><span>No healthy origins</span><span class="badge unhealthy">unhealthy</span></div>';

    const cb = health.circuit_breaker || 'unknown';
    document.getElementById('cb-state').innerHTML =
      '<div class="origin-row"><span>Circuit Breaker</span><span class="badge ' + cb + '">' + cb + '</span></div>';

    document.getElementById('updated').textContent = 'Last updated: ' + new Date().toLocaleTimeString();
  } catch(e) { console.error(e); }
}

update();
setInterval(update, 2000);
</script>
</body>
</html>`
