<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div class="ws-page">
    <div class="ws-infobar">
      <span class="ws-title">{{ $t('webapp.sessionTitle') }} — {{ asset.name || '…' }}</span>
      <span class="ws-account">{{ $t('hosts.credUser') }}: {{ asset.username || '—' }}</span>
      <span class="ws-audit">{{ $t('webapp.auditTip') }}</span>
      <el-button v-if="!started" size="small" @click="router.push('/webapps')">{{ $t('webapp.backList') }}</el-button>
      <el-button v-if="!started" type="primary" size="small" :loading="starting" @click="confirm">{{ $t('webapp.confirmOpen') }}</el-button>
      <el-button v-else size="small" @click="router.push('/webapps')">{{ $t('webapp.backList') }}</el-button>
    </div>
    <div class="ws-body">
      <div v-if="!started" class="ws-preflight">
        <el-form label-width="90px" style="max-width:480px">
          <el-form-item :label="$t('webapp.url')"><span class="mono">{{ asset.url }}</span></el-form-item>
          <el-form-item :label="$t('hosts.credUser')"><span class="mono">{{ asset.username || '—' }}</span></el-form-item>
          <el-form-item :label="$t('hosts.password')"><span style="color:#909399">{{ $t('webapp.pwdInjected') }}</span></el-form-item>
        </el-form>
        <el-alert type="warning" :closable="false" :title="$t('webapp.auditTip')" style="max-width:480px" />
      </div>
      <div v-show="started" class="ws-stage">
        <div v-if="stage !== 'live'" class="ws-status">{{ stage === 'opening' ? $t('webapp.loginInProgress') : $t('webapp.sessionStart') }}</div>
        <canvas ref="canvasEl" tabindex="0" class="ws-canvas"
                @mousedown="onMouse($event, 'down')" @mouseup="onMouse($event, 'up')"
                @click="onClick" @wheel.prevent="onWheel" @keydown.prevent="onKey" @contextmenu.prevent></canvas>
      </div>
    </div>
  </div>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '../api'
import i18n from '../i18n'

const { t } = i18n.global
const route = useRoute()
const router = useRouter()
const asset = ref({})
const started = ref(false)
const starting = ref(false)
const stage = ref('')
const canvasEl = ref(null)
let ws = null

const pos = e => {
  const c = canvasEl.value
  const sx = c.width / c.clientWidth
  const sy = c.height / c.clientHeight
  const r = c.getBoundingClientRect()
  return { x: Math.round((e.clientX - r.left) * sx), y: Math.round((e.clientY - r.top) * sy) }
}
const send = m => { if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify(m)) }
const onMouse = (e, phase) => { const p = pos(e); send({ t: phase === 'down' ? 'press' : 'release', x: p.x, y: p.y }) }
const onClick = e => { const p = pos(e); send({ t: 'click', x: p.x, y: p.y }); canvasEl.value?.focus() }
const onWheel = e => { const p = pos(e); send({ t: 'wheel', x: p.x, y: p.y, dy: e.deltaY }) }
const onKey = e => {
  if (e.key.length === 1) send({ t: 'text', text: e.key })
  else send({ t: 'key', key: e.key })
}

const confirm = async () => {
  starting.value = true
  try {
    await api.post(`/webassets/${route.params.id}/open`) // audited
    started.value = true
    stage.value = ''
    await nextTick()
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    ws = new WebSocket(`${proto}://${location.host}/api/webassets/${route.params.id}/stream?token=${localStorage.getItem('token')}`)
    ws.onmessage = ev => {
      const m = JSON.parse(ev.data)
      if (m.type === 'frame') {
        const img = new Image()
        img.onload = () => {
          const c = canvasEl.value
          if (!c) return
          c.width = img.width
          c.height = img.height
          c.getContext('2d').drawImage(img, 0, 0)
        }
        img.src = 'data:image/jpeg;base64,' + m.data
      } else if (m.type === 'status') stage.value = m.stage
      else if (m.type === 'error') ElMessage.error(m.message)
      else if (m.type === 'end') ws?.close()
    }
    ws.onopen = () => { stage.value = ''; canvasEl.value?.focus() }
  } finally {
    starting.value = false
  }
}

onMounted(async () => {
  asset.value = await api.get(`/webassets/${route.params.id}`).catch(() => ({}))
})
onBeforeUnmount(() => { try { ws?.close() } catch { /* ignore */ } })
</script>

<style scoped>
.ws-page { height: calc(100vh - 92px); display: flex; flex-direction: column; }
.ws-infobar { display: flex; align-items: center; gap: 14px; padding: 8px 12px; background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter); border-radius: 6px; flex: 0 0 auto; flex-wrap: wrap; }
.ws-title { font-weight: 600; }
.ws-account { color: var(--el-text-color-regular); }
.ws-audit { color: #909399; font-size: 12px; flex: 1; }
.ws-body { flex: 1 1 0; min-height: 0; margin-top: 10px; display: flex; flex-direction: column; }
.ws-preflight { padding: 24px; }
.ws-stage { position: relative; flex: 1; display: flex; flex-direction: column; min-height: 0; }
.ws-status { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; color: #909399; z-index: 2; }
.ws-canvas { flex: 1 1 auto; min-height: 0; width: auto; max-width: 100%; margin: 0 auto; background: #1e1e1e; border-radius: 6px; outline: none; }
</style>
