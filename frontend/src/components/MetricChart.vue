<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div class="mc-wrap">
    <svg :viewBox="`0 0 ${W} ${H}`" class="mc-svg" @mousemove="onMove" @mouseleave="hover = null">
      <!-- Y 轴刻度（按 yMax 换算 + 单位后缀；yMax=100 且单位 % 时即百分比刻度） -->
      <text v-for="p in [0, 25, 50, 75, 100]" :key="'y' + p" x="2" :y="yPct(p) + 4"
            class="mc-ylabel">{{ tickLabel(p) }}</text>
      <line v-for="p in [25, 50, 75, 100]" :key="'gl' + p" :x1="PAD.l" :x2="W - 6"
            :y1="yPct(p)" :y2="yPct(p)" stroke="#f0f2f5" stroke-width="1" />
      <!-- 折线 -->
      <polyline :points="line" fill="none" :stroke="color" stroke-width="2"
                stroke-linejoin="round" stroke-linecap="round" />
      <!-- 悬浮参考线 + 点 -->
      <template v-if="hover">
        <line :x1="hover.x" :x2="hover.x" :y1="4" :y2="H - 4" stroke="#c0c4cc" stroke-dasharray="3 3" />
        <circle :cx="hover.x" :cy="hover.y" r="4" :fill="color" stroke="#fff" stroke-width="1.5" />
      </template>
    </svg>
    <!-- X 轴时间标签 -->
    <div class="mc-xlabels">
      <span>{{ tLabel(firstT) }}</span><span v-if="midT">{{ midT }}</span><span>{{ tLabel(lastT) }}</span>
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
})

const W = 600, H = 110
const PAD = { l: 36, r: 8, t: 6, b: 6 }

const vals = computed(() => props.points.map(p => Number(p.v) || 0))
const line = computed(() => {
  const n = props.points.length
  if (n < 2) return ''
  return props.points.map((p, i) =>
    `${(PAD.l + (i / Math.max(1, n - 1)) * (W - PAD.l - 6)).toFixed(1)},${(H - 10 - ((Number(p.v) || 0) / props.yMax) * (H - 20)).toFixed(1)}`
  ).join(' ')
})
const yPct = p => (H - 10 - (p / props.yMax) * (H - 20)).toFixed(1)
const tickLabel = p => `${Math.round(((p / 100) * props.yMax) * 10) / 10}${props.unit}`

const hover = ref(null)
const tipLeft = computed(() => {
  if (!hover.value) return '50%'
  const x = parseFloat(hover.value.x)
  return `${Math.min(Math.max(x, 60), W - 60)}px`
})

function onMove(ev) {
  const n = props.points.length
  if (n < 2) { hover.value = null; return }
  const svg = ev.currentTarget
  const rect = svg.getBoundingClientRect()
  const xr = ((ev.clientX - rect.left) / rect.width) * W
  const idx = Math.round(((xr - PAD.l) / (W - PAD.l - 6)) * (n - 1))
  if (idx < 0 || idx >= n) { hover.value = null; return }
  const p = props.points[idx]
  const px = (PAD.l + (idx / Math.max(1, n - 1)) * (W - PAD.l - 6)).toFixed(1)
  const py = (H - 10 - ((Number(p.v) || 0) / props.yMax) * (H - 20)).toFixed(1)
  hover.value = { x: px, y: py, tLabel: fmtT(p.t), vLabel: fmtV(p.v) }
}
const fmtT = t => { const d = new Date(t); return isNaN(d) ? '-' : `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}` }
const fmtV = v => `${Math.round((Number(v) || 0) * 10) / 10}${props.unit}`
</script>

<style scoped>
.mc-wrap { position: relative; }
.mc-svg { width: 100%; display: block; background: #fafbfc; border-radius: 4px; }
.mc-ylabel { font-size: 9px; fill: #909399; }
.mc-xlabels { display: flex; justify-content: space-between; font-size: 10px; color: #909399; padding: 2px 4px 0; }
.mc-tip {
  position: absolute; transform: translateX(-50%); top: 0; pointer-events: none;
  background: #303133; color: #fff; border-radius: 4px; padding: 5px 8px;
  font-size: 11px; line-height: 1.5; white-space: nowrap; z-index: 5;
}
.mc-tip-v { font-weight: 600; }
</style>
