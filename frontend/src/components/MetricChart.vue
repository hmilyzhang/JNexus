<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div class="mc-wrap">
    <svg :viewBox="`0 0 ${W} ${H}`" class="mc-svg" @mousemove="onMove" @mouseleave="hover = null">
      <!-- Y axis ticks (converted from yMax plus unit suffix; with yMax=100 and unit %, these are percentage ticks) -->
      <text v-for="p in [0, 25, 50, 75, 100]" :key="'y' + p" x="2" :y="yOf(p) + 4"
            class="mc-ylabel">{{ tickLabel(p) }}</text>
      <line v-for="p in [25, 50, 75, 100]" :key="'gl' + p" :x1="PAD.l" :x2="W - 6"
            :y1="yOf(p)" :y2="yOf(p)" class="mc-grid" stroke-width="1" />
      <!-- Line -->
      <polyline v-if="points.length > 1" :points="line" fill="none" :stroke="color" stroke-width="2"
                stroke-linejoin="round" stroke-linecap="round" />
      <!-- Forecast extrapolation line (red dashed, starting from the last historical point) -->
      <polyline v-if="forecastLine" :points="forecastLine" fill="none" :stroke="forecastColor" stroke-width="1.8"
                stroke-dasharray="6 4" stroke-linejoin="round" stroke-linecap="round" />
      <!-- Single sample point -->
      <circle v-if="dotXY" :cx="dotXY.x" :cy="dotXY.y" r="4" :fill="color" stroke="#fff" stroke-width="1.5" />
      <!-- Empty data: keep the axis grid and show a centered gray hint (Zabbix style) -->
      <text v-if="!points.length && emptyText" :x="W / 2" :y="H / 2" text-anchor="middle" class="mc-empty">{{ emptyText }}</text>
      <!-- Hover reference line + point -->
      <template v-if="hover">
        <line :x1="hover.x" :x2="hover.x" :y1="4" :y2="H - 4" stroke="#c0c4cc" stroke-dasharray="3 3" />
        <circle :cx="hover.x" :cy="hover.y" r="4" :fill="hover.f ? forecastColor : color" stroke="#fff" stroke-width="1.5" />
      </template>
    </svg>
    <!-- X axis time labels: with a range, fix 5 evenly spaced ticks over the range (extended to the forecast end when forecasting); otherwise take the first/middle/last data points -->
    <div class="mc-xlabels">
      <span v-for="(lb, i) in xTicks" :key="'x' + i">{{ lb }}</span>
    </div>
    <!-- Tooltip -->
    <div v-if="hover" class="mc-tip" :style="{ left: tipLeft }">
      <div>{{ hover.tLabel }}</div>
      <div class="mc-tip-v" :style="{ color: hover.f ? forecastColor : color }">{{ hover.vLabel }}</div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'

const props = defineProps({
  points: { type: Array, default: () => [] }, // [{t: 'ISO'|epoch, v: number}]
  color: { type: String, default: '#409eff' },
  unit: { type: String, default: '%' },
  yMax: { type: Number, default: 100 }, // Full Y axis scale
  range: { type: Array, default: null }, // [start, end]: X axis ticks follow the time range, data is positioned by time (full axes shown even with no data)
  emptyText: { type: String, default: '' }, // Centered hint text when there is no data
  forecast: { type: Array, default: () => [] }, // [{t, v}] extrapolated forecast points based on history (red dashed; X axis auto-extends to the forecast end)
  forecastColor: { type: String, default: '#f56c6c' },
  forecastTag: { type: String, default: '' }, // Annotation suffix for hovered forecast values (e.g. "(forecast)")
})

const W = 600, H = 110
const PAD = { l: 36, r: 8, t: 6, b: 6 }

const toMs = t => { const d = new Date(t); return isNaN(d) ? null : d.getTime() }
const rangeMs = computed(() => {
  if (!props.range || props.range.length !== 2) return null
  let s = toMs(props.range[0]), e = toMs(props.range[1])
  if (s == null || e == null || e <= s) return null
  // When a forecast series exists, extend the X axis to the last forecast point
  const fl = props.forecast[props.forecast.length - 1]
  if (fl) {
    const fe = toMs(fl.t)
    if (fe != null && fe > e) e = fe
  }
  return [s, e]
})

const xOf = (t, i) => {
  const r = rangeMs.value
  if (r) {
    const ms = toMs(t)
    const frac = ms == null ? 0 : Math.min(1, Math.max(0, (ms - r[0]) / (r[1] - r[0])))
    return PAD.l + frac * (W - PAD.l - 6)
  }
  return PAD.l + (i / Math.max(1, props.points.length - 1)) * (W - PAD.l - 6)
}
const yOf = v => (H - 10 - ((Number(v) || 0) / props.yMax) * (H - 20)).toFixed(1)
const tickLabel = p => `${Math.round(((p / 100) * props.yMax) * 10) / 10}${props.unit}`

const line = computed(() => props.points.map((p, i) => `${xOf(p.t, i).toFixed(1)},${yOf(p.v)}`).join(' '))
const dotXY = computed(() => (props.points.length === 1
  ? { x: xOf(props.points[0].t, 0).toFixed(1), y: yOf(props.points[0].v) } : null))

// Forecast line: anchored to the last historical point, followed by the extrapolated points (requires time positioning mode)
const forecastLine = computed(() => {
  if (!rangeMs.value || !props.forecast.length || !props.points.length) return ''
  const last = props.points[props.points.length - 1]
  const head = `${xOf(last.t, props.points.length - 1).toFixed(1)},${yOf(last.v)}`
  const tail = props.forecast.map(p => `${xOf(p.t, 0).toFixed(1)},${yOf(p.v)}`).join(' ')
  return `${head} ${tail}`
})

const xTicks = computed(() => {
  const r = rangeMs.value
  if (r) return [0, 0.25, 0.5, 0.75, 1].map(f => fmtT(r[0] + f * (r[1] - r[0])))
  const n = props.points.length
  if (!n) return []
  if (n === 1) return [fmtT(props.points[0].t)]
  const ticks = [fmtT(props.points[0].t)]
  if (n > 2) ticks.push(fmtT(props.points[Math.floor((n - 1) / 2)].t))
  ticks.push(fmtT(props.points[n - 1].t))
  return ticks
})

const hover = ref(null)
const tipLeft = computed(() => {
  if (!hover.value) return '50%'
  const x = parseFloat(hover.value.x)
  return `${Math.min(Math.max(x, 60), W - 60)}px`
})

// Hover positioning: merge history and forecast and pick the point nearest the cursor (works for both time and uniform positioning)
function onMove(ev) {
  const n = props.points.length
  if (!n) { hover.value = null; return }
  const rect = ev.currentTarget.getBoundingClientRect()
  const xr = ((ev.clientX - rect.left) / rect.width) * W
  let pick = null, best = Infinity
  props.points.forEach((p, i) => {
    const x = xOf(p.t, i)
    const d = Math.abs(x - xr)
    if (d < best) { best = d; pick = { x, t: p.t, v: p.v, f: false } }
  })
  if (rangeMs.value) {
    props.forecast.forEach(p => {
      const x = xOf(p.t, 0)
      const d = Math.abs(x - xr)
      if (d < best) { best = d; pick = { x, t: p.t, v: p.v, f: true } }
    })
  }
  hover.value = {
    x: pick.x.toFixed(1), y: yOf(pick.v),
    tLabel: fmtT(pick.t), vLabel: fmtV(pick.v) + (pick.f ? props.forecastTag : ''), f: pick.f,
  }
}
const fmtT = t => { const d = new Date(t); return isNaN(d) ? '-' : `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}` }
const fmtV = v => `${Math.round((Number(v) || 0) * 10) / 10}${props.unit}`
</script>

<style scoped>
.mc-wrap { position: relative; }
.mc-svg { width: 100%; display: block; background: var(--mc-bg); border-radius: 4px; }
.mc-grid { stroke: var(--mc-grid); }
.mc-ylabel { font-size: 9px; fill: var(--mc-label); }
.mc-empty { font-size: 12px; fill: #c0c4cc; }
.mc-xlabels { display: flex; justify-content: space-between; font-size: 10px; color: var(--mc-label); padding: 2px 4px 0; }
.mc-tip {
  position: absolute; transform: translateX(-50%); top: 0; pointer-events: none;
  background: #303133; color: #fff; border-radius: 4px; padding: 5px 8px;
  font-size: 11px; line-height: 1.5; white-space: nowrap; z-index: 5;
}
.mc-tip-v { font-weight: 600; }
</style>
