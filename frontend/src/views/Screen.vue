<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div class="scr">
    <!-- 顶栏 -->
    <header class="scr-head">
      <div class="scr-brand">
        <svg viewBox="0 0 32 32" width="24" height="24"><rect width="32" height="32" rx="7" fill="#0d1526"/><path d="M8 11l6 5-6 5" stroke="#22d3ee" stroke-width="2.6" fill="none" stroke-linecap="round" stroke-linejoin="round"/><path d="M16 21h8" stroke="#4f8cff" stroke-width="2.6" stroke-linecap="round"/></svg>
        JNexus
        <span class="scr-sub">{{ $t('monitor.bigScreen') }}</span>
        <span v-if="data.maintenance" class="scr-maint">{{ $t('monitor.maintActive') }}</span>
      </div>
      <div class="scr-clock">
        <b>{{ clock }}</b>
        <span>{{ dateStr }}</span>
      </div>
      <div class="scr-actions">
        <el-button text @click="toggleFull" class="scr-btn">{{ full ? $t('monitor.screenExitFull') : $t('monitor.screenFull') }}</el-button>
        <el-button text @click="$router.push('/monitor')" class="scr-btn">{{ $t('monitor.screenExit') }}</el-button>
      </div>
    </header>

    <main class="scr-body">
      <!-- Overview：统计瓦片 -->
      <div class="g-section">{{ $t('screen.sectionOverview') }}</div>
      <section class="g-grid g-4">
        <div class="g-tile">
          <div class="g-tile-title">{{ $t('monitor.screenHostsOnline') }}</div>
          <div class="g-tile-num"><b>{{ data.hosts?.online ?? 0 }}</b><i>/ {{ data.hosts?.total ?? 0 }}</i></div>
          <div class="g-bar"><i :style="{ width: pct(data.hosts?.online, data.hosts?.total), background: '#5794f2' }"></i></div>
        </div>
        <div class="g-tile">
          <div class="g-tile-title">{{ $t('monitor.screenMonitorsUp') }}</div>
          <div class="g-tile-num"><b class="c-green">{{ data.monitors?.up ?? 0 }}</b><i>/ {{ data.monitors?.total ?? 0 }}</i></div>
          <div class="g-bar"><i :style="{ width: pct(data.monitors?.up, data.monitors?.total), background: '#73bf69' }"></i></div>
        </div>
        <div class="g-tile">
          <div class="g-tile-title">{{ $t('monitor.screenAlertsToday') }}</div>
          <div class="g-tile-num"><b :class="(data.alerts?.today ?? 0) > 0 ? 'c-red' : 'c-green'">{{ data.alerts?.today ?? 0 }}</b></div>
          <svg class="g-spark" viewBox="0 0 100 26" preserveAspectRatio="none">
            <polyline :points="alertSpark" fill="none" stroke="#f2495c" stroke-width="1.6" />
          </svg>
        </div>
        <div class="g-tile">
          <div class="g-tile-title">{{ $t('monitor.screenTasksToday') }}</div>
          <div class="g-tile-num"><b>{{ data.tasks_today ?? 0 }}</b></div>
        </div>
      </section>

      <!-- Trends：时间线面板 -->
      <div class="g-section">{{ $t('screen.sectionTrends') }}</div>
      <section class="g-grid g-3">
        <div class="g-panel">
          <div class="g-panel-title">{{ $t('screen.panelHostTrend') }}</div>
          <svg v-if="trendChart" viewBox="0 0 600 170" class="g-chart">
            <line v-for="(g, i) in trendChart.grid" :key="'g' + i" x1="34" :y1="g.y" x2="594" :y2="g.y" class="g-grid" />
            <text v-for="(g, i) in trendChart.grid" :key="'gy' + i" x="30" :y="g.y + 3" text-anchor="end" class="g-ytick">{{ g.label }}</text>
            <polyline v-for="(sr, i) in trendChart.series" :key="'s' + i" :points="sr.points" fill="none"
                      :stroke="sr.color" stroke-width="1.7" stroke-linejoin="round" />
            <text v-for="(xl, i) in trendChart.xLabels" :key="'x' + i" :x="xl.x" y="164" :text-anchor="xl.anchor" class="g-xtick">{{ xl.text }}</text>
          </svg>
          <div class="g-legend">
            <span><i style="background:#5794f2"></i>CPU</span>
            <span><i style="background:#73bf69"></i>Memory</span>
          </div>
          <div v-if="!trendChart" class="g-empty">{{ $t('monitor.screenNoData') }}</div>
        </div>
        <div class="g-panel">
          <div class="g-panel-title">{{ $t('screen.panelRespTrend') }}</div>
          <svg v-if="respChart" viewBox="0 0 600 170" class="g-chart">
            <line v-for="(g, i) in respChart.grid" :key="'g' + i" x1="34" :y1="g.y" x2="594" :y2="g.y" class="g-grid" />
            <text v-for="(g, i) in respChart.grid" :key="'gy' + i" x="30" :y="g.y + 3" text-anchor="end" class="g-ytick">{{ g.label }}</text>
            <polyline :points="respChart.points" fill="none" stroke="#ff9830" stroke-width="1.7" stroke-linejoin="round" />
            <text v-for="(xl, i) in respChart.xLabels" :key="'x' + i" :x="xl.x" y="164" :text-anchor="xl.anchor" class="g-xtick">{{ xl.text }}</text>
          </svg>
          <div v-if="!respChart" class="g-empty">{{ $t('monitor.screenNoData') }}</div>
        </div>
        <div class="g-panel">
          <div class="g-panel-title">{{ $t('screen.panelAlertsDaily') }}</div>
          <svg v-if="alertChart" viewBox="0 0 600 170" class="g-chart">
            <line v-for="(g, i) in alertChart.grid" :key="'g' + i" x1="34" :y1="g.y" x2="594" :y2="g.y" class="g-grid" />
            <text v-for="(g, i) in alertChart.grid" :key="'gy' + i" x="30" :y="g.y + 3" text-anchor="end" class="g-ytick">{{ g.label }}</text>
            <rect v-for="(b, i) in alertChart.bars" :key="'b' + i" :x="b.x" :y="b.y" :width="b.w" :height="b.h"
                  :fill="b.n > 0 ? '#f2495c' : '#2c3235'" opacity="0.92" />
            <text v-for="(xl, i) in alertChart.xLabels" :key="'x' + i" :x="xl.x" y="164" :text-anchor="xl.anchor" class="g-xtick">{{ xl.text }}</text>
          </svg>
          <div v-if="!alertChart" class="g-empty">{{ $t('monitor.screenNoData') }}</div>
        </div>
      </section>

      <!-- Status：列表面板 -->
      <div class="g-section">{{ $t('screen.sectionStatus') }}</div>
      <section class="g-grid g-3">
        <div class="g-panel">
          <div class="g-panel-title">{{ $t('monitor.screenGroups') }}</div>
          <div v-for="g in data.hosts?.groups || []" :key="g.name" class="g-row">
            <span class="g-name">{{ g.name }}</span>
            <span class="g-cnt" :class="{ warn: g.online < g.total }">{{ g.online }}/{{ g.total }}</span>
          </div>
          <div v-if="!(data.hosts?.groups || []).length" class="g-empty">{{ $t('monitor.screenNoData') }}</div>
        </div>
        <div class="g-panel">
          <div class="g-panel-title">{{ $t('monitor.screenProbes') }}</div>
          <div v-for="m in data.monitors?.rows || []" :key="m.name" class="g-row">
            <span class="g-dot" :class="'d-' + m.status"></span>
            <span class="g-name">{{ m.name }}</span>
            <span class="g-ms">{{ m.status === 'up' && m.resp_ms ? m.resp_ms + 'ms' : '' }}</span>
            <span class="g-cnt" :class="{ bad: m.status === 'down', warn: m.status === 'paused' || m.status === 'unknown' }">{{ stText(m.status) }}</span>
          </div>
          <div v-if="!(data.monitors?.rows || []).length" class="g-empty">{{ $t('monitor.screenNoData') }}</div>
        </div>
        <div class="g-panel">
          <div class="g-panel-title">{{ $t('monitor.screenRecentAlerts') }}</div>
          <div v-for="a in data.alerts?.recent || []" :key="a.id" class="g-alert">
            <span class="g-lv" :class="a.level === 'P1' ? 'lv-p1' : a.level === 'P2' ? 'lv-p2' : 'lv-x'">{{ a.level }}</span>
            <span class="g-alert-body">
              <b>{{ a.target }}</b>
              <i>{{ a.message }}</i>
            </span>
            <span class="g-ms">{{ fmtTime(a.fired_at) }}</span>
          </div>
          <div v-if="!(data.alerts?.recent || []).length" class="g-empty">{{ $t('monitor.screenNoAlerts') }}</div>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'


const { t } = i18n.global
const data = ref({})
const clock = ref('')
const dateStr = ref('')
const full = ref(false)
let clockTimer = null, dataTimer = null

const pct = (a, b) => (b ? Math.round(((a || 0) / b) * 100) + '%' : '0%')
const stText = st => ({ up: t('monitor.up'), down: t('monitor.down'), paused: t('monitor.paused'), unknown: t('monitor.notYet') }[st] || st)
const fmtTime = t => {
  const d = new Date(t)
  return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}
const hm = t => { const d = new Date(t); return `${String(d.getHours()).padStart(2, '0')}:00` }

// ---- 通用线图几何（Grafana 风格网格 + 多序列） ----
function lineGeom(rows, seriesDefs, yMax, xFmt) {
  const W = 600, H = 160, L = 34, R = 8, T = 8, B = 16
  const n = rows.length
  if (n < 2) return null
  const x = i => L + (i / (n - 1)) * (W - L - R)
  const y = v => T + (1 - (Number(v) || 0) / yMax) * (H - T - B)
  const grid = [0, 0.25, 0.5, 0.75, 1].map(f => {
    const v = yMax * (1 - f)
    return { y: (T + f * (H - T - B)).toFixed(1), label: v >= 1 ? Math.round(v) : (Math.round(v * 10) / 10) }
  })
  const series = seriesDefs.map(sd => ({
    color: sd.color,
    points: rows.map((r, i) => `${x(i).toFixed(1)},${y(sd.get(r)).toFixed(1)}`).join(' '),
  }))
  const mid = Math.floor((n - 1) / 2)
  const xLabels = [
    { x: L, text: xFmt(rows[0].t), anchor: 'start' },
    { x: x(mid), text: xFmt(rows[mid].t), anchor: 'middle' },
    { x: W - R, text: xFmt(rows[n - 1].t), anchor: 'end' },
  ]
  return { grid, series, xLabels }
}

const trendChart = computed(() => lineGeom(
  data.value.trend || [],
  [
    { color: '#5794f2', get: r => r.cpu },
    { color: '#73bf69', get: r => r.mem },
  ],
  100,
  hm,
))
const respChart = computed(() => {
  const rows = data.value.resp_trend || []
  if (rows.length < 2) return null
  const maxMs = Math.max(...rows.map(r => r.ms), 1) * 1.15
  return lineGeom(rows, [{ color: '#ff9830', get: r => r.ms }], maxMs, hm)
})
const alertSpark = computed(() => {
  const days = data.value.alerts_daily || []
  if (days.length < 2) return '0,26 100,26'
  const max = Math.max(...days.map(d => d.n), 1)
  return days.map((d, i) => `${(i / (days.length - 1)) * 100},${26 - (d.n / max) * 24}`).join(' ')
})
const alertChart = computed(() => {
  const days = data.value.alerts_daily || []
  if (!days.length) return null
  const W = 600, H = 160, L = 34, R = 8, B = 16, T = 8
  const max = Math.max(...days.map(d => d.n), 1)
  const slot = (W - L - R) / days.length
  const bw = Math.max(6, slot * 0.6)
  const bars = days.map((d, i) => {
    const h = (d.n / max) * (H - T - B)
    return { x: (L + i * slot + (slot - bw) / 2).toFixed(1), y: (H - B - h).toFixed(1), w: bw.toFixed(1), h: Math.max(h, 1).toFixed(1), n: d.n }
  })
  const xLabels = days
    .map((d, i) => ({ i, x: L + i * slot + slot / 2 }))
    .filter(o => o.i % 2 === 0)
    .map(o => ({ x: o.x, text: (days[o.i].d || '').slice(5), anchor: 'middle' }))
  const grid = [0, 0.5, 1].map(f => {
    const v = max * (1 - f)
    return { y: (T + f * (H - T - B)).toFixed(1), label: Math.round(v) }
  })
  return { bars, grid, xLabels }
})

const load = async () => {
  try {
    data.value = await api.get('/monitoring/screen')
  } catch { /* 断线时保留上一帧数据 */ }
}

const tickClock = () => {
  const d = new Date()
  clock.value = [d.getHours(), d.getMinutes(), d.getSeconds()].map(x => String(x).padStart(2, '0')).join(':')
  dateStr.value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const toggleFull = () => {
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen().then(() => { full.value = true }).catch(() => {})
  } else {
    document.exitFullscreen().then(() => { full.value = false }).catch(() => {})
  }
}

onMounted(() => {
  tickClock()
  load()
  clockTimer = setInterval(tickClock, 1000)
  dataTimer = setInterval(load, 15000)
  document.addEventListener('fullscreenchange', onFs)
})
const onFs = () => { full.value = !!document.fullscreenElement }
onBeforeUnmount(() => {
  clearInterval(clockTimer)
  clearInterval(dataTimer)
  document.removeEventListener('fullscreenchange', onFs)
})
</script>

<style scoped>
/* Grafana 风格：深灰面板 + 细边框 + 小标题色条 */
.scr {
  min-height: 100vh; background: #111217; color: #ccccdc;
  font-family: -apple-system, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
  display: flex; flex-direction: column;
}
.scr-head {
  display: flex; align-items: center; gap: 16px; padding: 10px 22px;
  background: #181b1f; border-bottom: 1px solid #26292e;
}
.scr-brand { display: flex; align-items: center; gap: 10px; font-size: 19px; font-weight: 800; color: #fff; }
.scr-sub { color: #7ee7f7; font-size: 13px; font-weight: 600; }
.scr-maint {
  padding: 2px 10px; border-radius: 999px; font-size: 12px; margin-left: 8px;
  color: #ff9830; background: rgba(255, 152, 48, .12); border: 1px solid rgba(255, 152, 48, .45);
}
.scr-clock { margin-left: auto; text-align: right; line-height: 1.2; }
.scr-clock b { font-size: 24px; font-weight: 800; color: #fff; font-variant-numeric: tabular-nums; letter-spacing: 1px; }
.scr-clock span { display: block; font-size: 11.5px; color: #8e8e8e; }
.scr-actions { display: flex; gap: 2px; }
.scr-btn { color: #8e8e8e !important; }
.scr-btn:hover { color: #fff !important; }

.scr-body { flex: 1; padding: 14px 22px 24px; }
.g-section {
  font-size: 13px; font-weight: 700; color: #8e8e8e; letter-spacing: .8px;
  margin: 14px 2px 8px; text-transform: uppercase;
}
.g-grid { display: grid; gap: 12px; }
.g-4 { grid-template-columns: repeat(4, 1fr); }
.g-3 { grid-template-columns: repeat(3, 1fr); }

/* 面板 */
.g-panel, .g-tile {
  background: #181b1f; border: 1px solid #26292e; border-radius: 2px; padding: 10px 12px;
}
.g-panel-title {
  display: inline-block; font-size: 12.5px; font-weight: 700; color: #ccccdc;
  padding-bottom: 5px; margin-bottom: 8px; border-bottom: 2px solid #5794f2;
}
.g-chart { width: 100%; display: block; }
.g-grid { stroke: #2c3235; stroke-width: 1; }
.g-ytick { font-size: 9.5px; fill: #8e8e8e; text-anchor: end; }
.g-xtick { font-size: 9.5px; fill: #8e8e8e; }
.g-legend { display: flex; gap: 16px; margin-top: 6px; }
.g-legend span { font-size: 11.5px; color: #b7b7b7; display: flex; align-items: center; gap: 5px; }
.g-legend i { width: 10px; height: 4px; display: inline-block; }
.g-empty { color: #5f6368; font-size: 13px; padding: 18px 4px; }

/* 统计瓦片 */
.g-tile { padding: 12px 14px 14px; }
.g-tile-title { font-size: 12px; color: #8e8e8e; margin-bottom: 4px; }
.g-tile-num b { font-size: 40px; font-weight: 800; line-height: 1.05; color: #fff; }
.g-tile-num i { font-style: normal; font-size: 16px; color: #8e8e8e; font-weight: 600; margin-left: 4px; }
.c-green { color: #73bf69; }
.c-red { color: #f2495c; }
.g-bar { height: 5px; border-radius: 3px; background: #2c3235; margin-top: 10px; overflow: hidden; }
.g-bar i { display: block; height: 100%; border-radius: 3px; }
.g-spark { width: 100%; height: 26px; display: block; margin-top: 10px; }

/* 列表行 */
.g-row { display: flex; align-items: center; gap: 10px; padding: 6px 2px; border-bottom: 1px solid #26292e; font-size: 13.5px; }
.g-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #ccccdc; }
.g-cnt { font-weight: 700; font-variant-numeric: tabular-nums; color: #ccccdc; }
.g-cnt.warn { color: #ff9830; }
.g-cnt.bad { color: #f2495c; }
.g-ms { font-size: 11.5px; color: #8e8e8e; font-variant-numeric: tabular-nums; }
.g-dot { width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0; }
.d-up { background: #73bf69; }
.d-down { background: #f2495c; }
.d-paused, .d-unknown { background: #5f6368; }

.g-alert { display: flex; align-items: flex-start; gap: 9px; padding: 7px 2px; border-bottom: 1px solid #26292e; }
.g-lv { flex-shrink: 0; padding: 1px 7px; border-radius: 2px; font-size: 11px; font-weight: 700; }
.lv-p1 { color: #f2495c; background: rgba(242, 73, 92, .14); border: 1px solid rgba(242, 73, 92, .5); }
.lv-p2 { color: #ff9830; background: rgba(255, 152, 48, .12); border: 1px solid rgba(255, 152, 48, .45); }
.lv-x { color: #8e8e8e; background: rgba(255, 255, 255, .05); border: 1px solid #26292e; }
.g-alert-body { flex: 1; min-width: 0; line-height: 1.45; }
.g-alert-body b { display: block; font-size: 13px; color: #ccccdc; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.g-alert-body i { font-style: normal; font-size: 11.5px; color: #8e8e8e; display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

@media (max-width: 1000px) {
  .g-3, .g-4 { grid-template-columns: 1fr; }
}
</style>
