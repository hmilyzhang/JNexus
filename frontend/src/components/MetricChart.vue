<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div class="mc-wrap">
    <svg :viewBox="`0 0 ${W} ${H}`" class="mc-svg" @mousemove="onMove" @mouseleave="hover = null">
      <!-- Y 轴刻度（按 yMax 换算 + 单位后缀；yMax=100 且单位 % 时即百分比刻度） -->
      <text v-for="p in [0, 25, 50, 75, 100]" :key="'y' + p" x="2" :y="yOf(p) + 4"
            class="mc-ylabel">{{ tickLabel(p) }}</text>
      <line v-for="p in [25, 50, 75, 100]" :key="'gl' + p" :x1="PAD.l" :x2="W - 6"
            :y1="yOf(p)" :y2="yOf(p)" stroke="#f0f2f5" stroke-width="1" />
      <!-- 折线 -->
      <polyline v-if="points.length > 1" :points="line" fill="none" :stroke="color" stroke-width="2"
                stroke-linejoin="round" stroke-linecap="round" />
      <!-- 单点样本 -->
      <circle v-if="dotXY" :cx="dotXY.x" :cy="dotXY.y" r="4" :fill="color" stroke="#fff" stroke-width="1.5" />
      <!-- 空数据：坐标轴网格保留，居中灰字提示（Zabbix 风格） -->
      <text v-if="!points.length && emptyText" :x="W / 2" :y="H / 2" text-anchor="middle" class="mc-empty">{{ emptyText }}</text>
      <!-- 悬浮参考线 + 点 -->
      <template v-if="hover">
        <line :x1="hover.x" :x2="hover.x" :y1="4" :y2="H - 4" stroke="#c0c4cc" stroke-dasharray="3 3" />
        <circle :cx="hover.x" :cy="hover.y" r="4" :fill="color" stroke="#fff" stroke-width="1.5" />
      </template>
    </svg>
    <!-- X 轴时间标签：传入 range 时按范围固定 5 档刻度，否则取数据点首/中/尾 -->
    <div class="mc-xlabels">
      <span v-for="(lb, i) in xTicks" :key="'x' + i">{{ lb }}</span>
    </div>
    <!-- 悬浮提示 -->
    <div v-if="hover" class="mc-tip" :style="{ left: tipLeft }">
      <div>{{ hover.tLabel }}</div>
      <div class="mc-tip-v" :style="{ color }">{{ hover.vLabel }}</div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'

const props = defineProps({
  points: { type: Array, default: () => [] }, // [{t: 'ISO'|epoch, v: number}]
  color: { type: String, default: '#409eff' },
  unit: { type: String, default: '%' },
  yMax: { type: Number, default: 100 }, // Y 轴满刻度
  range: { type: Array, default: null }, // [start, end]：X 轴按时间范围画刻度、数据按时间定位（空数据也显示完整坐标轴）
  emptyText: { type: String, default: '' }, // 无数据时居中提示文字
})

const W = 600, H = 110
const PAD = { l: 36, r: 8, t: 6, b: 6 }

const toMs = t => { const d = new Date(t); return isNaN(d) ? null : d.getTime() }
const rangeMs = computed(() => {
  if (!props.range || props.range.length !== 2) return null
  const s = toMs(props.range[0]), e = toMs(props.range[1])
  return s != null && e != null && e > s ? [s, e] : null
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

// 悬浮定位：取距光标最近的点（时间定位/等距定位均适用）
function onMove(ev) {
  const n = props.points.length
  if (!n) { hover.value = null; return }
  const rect = ev.currentTarget.getBoundingClientRect()
  const xr = ((ev.clientX - rect.left) / rect.width) * W
  let idx = 0, best = Infinity
  props.points.forEach((p, i) => {
    const d = Math.abs(xOf(p.t, i) - xr)
    if (d < best) { best = d; idx = i }
  })
  const p = props.points[idx]
  hover.value = { x: xOf(p.t, idx).toFixed(1), y: yOf(p.v), tLabel: fmtT(p.t), vLabel: fmtV(p.v) }
}
const fmtT = t => { const d = new Date(t); return isNaN(d) ? '-' : `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}` }
const fmtV = v => `${Math.round((Number(v) || 0) * 10) / 10}${props.unit}`
</script>

<style scoped>
.mc-wrap { position: relative; }
.mc-svg { width: 100%; display: block; background: #fafbfc; border-radius: 4px; }
.mc-ylabel { font-size: 9px; fill: #909399; }
.mc-empty { font-size: 12px; fill: #c0c4cc; }
.mc-xlabels { display: flex; justify-content: space-between; font-size: 10px; color: #909399; padding: 2px 4px 0; }
.mc-tip {
  position: absolute; transform: translateX(-50%); top: 0; pointer-events: none;
  background: #303133; color: #fff; border-radius: 4px; padding: 5px 8px;
  font-size: 11px; line-height: 1.5; white-space: nowrap; z-index: 5;
}
.mc-tip-v { font-weight: 600; }
</style>
