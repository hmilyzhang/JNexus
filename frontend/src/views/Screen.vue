<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div class="scr">
    <header class="scr-head">
      <div class="scr-brand">
        <svg viewBox="0 0 32 32" width="26" height="26"><rect width="32" height="32" rx="7" fill="#0d1526"/><path d="M8 11l6 5-6 5" stroke="#22d3ee" stroke-width="2.6" fill="none" stroke-linecap="round" stroke-linejoin="round"/><path d="M16 21h8" stroke="#4f8cff" stroke-width="2.6" stroke-linecap="round"/></svg>
        {{ systemName }}
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
      <!-- 顶部统计 -->
      <section class="scr-stats">
        <div class="scr-stat">
          <div class="scr-num">{{ data.hosts?.online ?? 0 }}<i>/{{ data.hosts?.total ?? 0 }}</i></div>
          <div class="scr-lbl">{{ $t('monitor.screenHostsOnline') }}</div>
          <div class="scr-bar"><i :style="{ width: pct(data.hosts?.online, data.hosts?.total) }"></i></div>
        </div>
        <div class="scr-stat">
          <div class="scr-num ok">{{ data.monitors?.up ?? 0 }}<i>/{{ data.monitors?.total ?? 0 }}</i></div>
          <div class="scr-lbl">{{ $t('monitor.screenMonitorsUp') }}</div>
          <div class="scr-bar"><i class="ok" :style="{ width: pct(data.monitors?.up, data.monitors?.total) }"></i></div>
        </div>
        <div class="scr-stat">
          <div class="scr-num" :class="{ bad: (data.alerts?.today ?? 0) > 0 }">{{ data.alerts?.today ?? 0 }}</div>
          <div class="scr-lbl">{{ $t('monitor.screenAlertsToday') }}</div>
        </div>
        <div class="scr-stat">
          <div class="scr-num">{{ data.tasks_today ?? 0 }}</div>
          <div class="scr-lbl">{{ $t('monitor.screenTasksToday') }}</div>
        </div>
      </section>

      <!-- 主体三栏 -->
      <section class="scr-grid">
        <div class="scr-panel">
          <h3>{{ $t('monitor.screenGroups') }}</h3>
          <div v-for="g in data.hosts?.groups || []" :key="g.name" class="scr-row">
            <span class="scr-name">{{ g.name }}</span>
            <span class="scr-cnt" :class="{ warn: g.online < g.total }">{{ g.online }}/{{ g.total }}</span>
          </div>
          <div v-if="!(data.hosts?.groups || []).length" class="scr-empty">{{ $t('monitor.screenNoData') }}</div>
        </div>
        <div class="scr-panel">
          <h3>{{ $t('monitor.screenProbes') }}</h3>
          <div v-for="m in data.monitors?.rows || []" :key="m.name" class="scr-row">
            <span class="scr-dot" :class="'d-' + m.status"></span>
            <span class="scr-name">{{ m.name }}</span>
            <span class="scr-ms">{{ m.status === 'up' && m.resp_ms ? m.resp_ms + 'ms' : '' }}</span>
            <span class="scr-cnt" :class="{ bad: m.status === 'down', warn: m.status === 'paused' || m.status === 'unknown' }">{{ stText(m.status) }}</span>
          </div>
          <div v-if="!(data.monitors?.rows || []).length" class="scr-empty">{{ $t('monitor.screenNoData') }}</div>
        </div>
        <div class="scr-panel">
          <h3>{{ $t('monitor.screenRecentAlerts') }}</h3>
          <div v-for="a in data.alerts?.recent || []" :key="a.id" class="scr-alert">
            <span class="scr-lv" :class="a.level === 'P1' ? 'lv-p1' : a.level === 'P2' ? 'lv-p2' : 'lv-x'">{{ a.level }}</span>
            <span class="scr-alert-body">
              <b>{{ a.target }}</b>
              <i>{{ a.message }}</i>
            </span>
            <span class="scr-ms">{{ fmtTime(a.fired_at) }}</span>
          </div>
          <div v-if="!(data.alerts?.recent || []).length" class="scr-empty">{{ $t('monitor.screenNoAlerts') }}</div>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { useUserStore } from '../store'

const { t } = i18n.global
const store = useUserStore()
const systemName = ref(localStorage.getItem('system_name') || 'JNexus')

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
.scr {
  min-height: 100vh; background:
    radial-gradient(900px 400px at 15% -5%, rgba(79, 140, 255, .14), transparent 60%),
    radial-gradient(700px 380px at 88% 5%, rgba(34, 211, 238, .1), transparent 60%),
    #0b1020;
  color: #e7edfb; display: flex; flex-direction: column;
  font-family: -apple-system, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
}
.scr-head {
  display: flex; align-items: center; gap: 18px; padding: 14px 26px;
  border-bottom: 1px solid rgba(255, 255, 255, .09);
}
.scr-brand { display: flex; align-items: center; gap: 10px; font-size: 21px; font-weight: 800; }
.scr-sub { color: #7ee7f7; font-size: 14px; font-weight: 600; }
.scr-maint {
  padding: 2px 10px; border-radius: 999px; font-size: 12px; margin-left: 8px;
  color: #ffd591; background: rgba(230, 162, 60, .15); border: 1px solid rgba(230, 162, 60, .4);
}
.scr-clock { margin-left: auto; text-align: right; line-height: 1.2; }
.scr-clock b { font-size: 26px; font-weight: 800; letter-spacing: 1px; font-variant-numeric: tabular-nums; }
.scr-clock span { display: block; font-size: 12px; color: #97a3ba; }
.scr-actions { display: flex; gap: 4px; }
.scr-btn { color: #97a3ba !important; }
.scr-btn:hover { color: #e7edfb !important; }

.scr-body { flex: 1; padding: 18px 26px 26px; display: flex; flex-direction: column; gap: 16px; }
.scr-stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; }
.scr-stat {
  background: rgba(255, 255, 255, .04); border: 1px solid rgba(255, 255, 255, .09);
  border-radius: 12px; padding: 16px 20px;
}
.scr-num { font-size: 40px; font-weight: 800; line-height: 1.1; }
.scr-num i { font-style: normal; font-size: 18px; color: #97a3ba; font-weight: 600; }
.scr-num.ok { color: #4ade80; }
.scr-num.bad { color: #f56c6c; }
.scr-lbl { font-size: 13px; color: #97a3ba; margin-top: 2px; }
.scr-bar { height: 5px; border-radius: 3px; background: rgba(255, 255, 255, .08); margin-top: 10px; overflow: hidden; }
.scr-bar i { display: block; height: 100%; background: linear-gradient(120deg, #4f8cff, #22d3ee); border-radius: 3px; }
.scr-bar i.ok { background: linear-gradient(120deg, #22c55e, #4ade80); }

.scr-grid { flex: 1; display: grid; grid-template-columns: 1fr 1.2fr 1.4fr; gap: 16px; min-height: 0; }
.scr-panel {
  background: rgba(255, 255, 255, .035); border: 1px solid rgba(255, 255, 255, .09);
  border-radius: 12px; padding: 16px 18px; overflow: auto;
}
.scr-panel h3 { font-size: 14px; color: #9fd3ff; margin-bottom: 12px; letter-spacing: .5px; }
.scr-row { display: flex; align-items: center; gap: 10px; padding: 7px 4px; border-bottom: 1px dashed rgba(255, 255, 255, .06); font-size: 14px; }
.scr-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.scr-cnt { font-weight: 700; font-variant-numeric: tabular-nums; }
.scr-cnt.warn { color: #e6a23c; }
.scr-cnt.bad { color: #f56c6c; }
.scr-ms { font-size: 12px; color: #97a3ba; font-variant-numeric: tabular-nums; }
.scr-dot { width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0; }
.d-up { background: #4ade80; }
.d-down { background: #f56c6c; }
.d-paused, .d-unknown { background: #6b7a8d; }

.scr-alert { display: flex; align-items: flex-start; gap: 10px; padding: 8px 4px; border-bottom: 1px dashed rgba(255, 255, 255, .06); }
.scr-lv {
  flex-shrink: 0; padding: 1px 8px; border-radius: 5px; font-size: 11.5px; font-weight: 700;
}
.lv-p1 { color: #ffb4b4; background: rgba(245, 108, 108, .18); border: 1px solid rgba(245, 108, 108, .45); }
.lv-p2 { color: #ffd591; background: rgba(230, 162, 60, .15); border: 1px solid rgba(230, 162, 60, .4); }
.lv-x { color: #a3abb9; background: rgba(255, 255, 255, .06); border: 1px solid rgba(255, 255, 255, .12); }
.scr-alert-body { flex: 1; min-width: 0; line-height: 1.45; }
.scr-alert-body b { display: block; font-size: 13.5px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.scr-alert-body i { font-style: normal; font-size: 12px; color: #97a3ba; display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.scr-empty { color: #6b7a8d; font-size: 13px; padding: 14px 4px; }

@media (max-width: 1000px) {
  .scr-grid { grid-template-columns: 1fr; }
  .scr-stats { grid-template-columns: repeat(2, 1fr); }
}
</style>
