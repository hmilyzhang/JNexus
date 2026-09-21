<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <template #header>
        <div style="display:flex; align-items:center; justify-content:space-between">
          <span style="font-weight:600">{{ $t('webapp.title') }}</span>
          <el-button v-if="store.isAdmin" type="primary" size="small" @click="dlg()">{{ $t('webapp.create') }}</el-button>
        </div>
      </template>
      <el-table :data="assets" v-loading="loading" size="small" border>
        <el-table-column prop="name" :label="$t('webapp.name')" min-width="140" />
        <el-table-column prop="url" :label="$t('webapp.url')" min-width="220" show-overflow-tooltip />
        <el-table-column prop="username" :label="$t('hosts.credUser')" min-width="110" />
        <el-table-column prop="description" :label="$t('scripts.desc')" min-width="140" show-overflow-tooltip />
        <el-table-column :label="$t('common.operation')" width="200" fixed="right">
          <template #default="{ row }">
            <el-button type="success" size="small" link @click="openCard(row)">{{ $t('webapp.open') }}</el-button>
            <el-button v-if="store.isAdmin" size="small" link @click="dlg(row)">{{ $t('common.edit') }}</el-button>
            <el-popconfirm :title="$t('common.delete') + '?'" @confirm="del(row)">
              <template #reference>
                <el-button v-if="store.isAdmin" size="small" type="danger" link>{{ $t('common.delete') }}</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <div style="color:#909399; font-size:12px; margin-top:8px">{{ $t('webapp.tip') }}</div>
    </el-card>

    <!-- PAM-style confirm card: pre-filled account, audited open -->
    <el-dialog v-model="openVisible" :title="$t('webapp.openTitle')" width="460px">
      <el-form label-width="90px">
        <el-form-item :label="$t('webapp.url')"><span class="mono" style="word-break:break-all">{{ current.url }}</span></el-form-item>
        <el-form-item :label="$t('hosts.credUser')"><span class="mono">{{ current.username || '—' }}</span></el-form-item>
        <el-form-item :label="$t('hosts.password')">
          <span class="mono">{{ pwdVisible ? (pwd || '••••••••') : '••••••••' }}</span>
          <el-button v-if="store.isAdmin && current.has_password" size="small" link style="margin-left:8px" @click="revealPwd">
            {{ pwdVisible ? $t('common.hide') : $t('rot.view') }}
          </el-button>
        </el-form-item>
      </el-form>
      <el-alert type="warning" :closable="false" :title="$t('webapp.auditTip')" />
      <template #footer>
        <el-button @click="openVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button @click="openOriginal">{{ $t('webapp.openOriginal') }}</el-button>
        <el-button v-if="current.has_password" type="primary" :loading="streamConnecting" @click="startStream">{{ $t('webapp.remoteSession') }}</el-button>
      </template>
    </el-dialog>

    <!-- Headless-browser remote session -->
    <el-dialog v-model="streamVisible" :title="$t('webapp.sessionTitle') + ' — ' + (current.name || '')" width="90%" top="2vh"
               :close-on-click-modal="false" @closed="stopStream">
      <div v-if="streamStage !== 'live'" style="text-align:center; padding:30px 0; color:#909399">
        {{ streamStage === 'opening' ? $t('webapp.loginInProgress') : $t('webapp.sessionStart') }}
      </div>
      <canvas ref="streamCanvas" tabindex="0" class="web-stream"
              style="width:100%; display:block; border:1px solid #333; outline:none"
              @mousedown="onStreamMouse($event, 'down')" @mouseup="onStreamMouse($event, 'up')"
              @click="onStreamClick" @wheel.prevent="onStreamWheel" @keydown.prevent="onStreamKey"
              @contextmenu.prevent></canvas>
      <div style="color:#909399; font-size:12px; margin-top:6px">{{ $t('webapp.streamTip') }}</div>
    </el-dialog>

    <!-- Edit (admin) -->
    <el-dialog v-model="editVisible" :title="form.id ? $t('common.edit') : $t('webapp.create')" width="480px">
      <el-form label-width="90px">
        <el-form-item :label="$t('webapp.name')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="$t('webapp.url')"><el-input v-model="form.url" class="mono" placeholder="https://" /></el-form-item>
        <el-form-item :label="$t('hosts.credUser')"><el-input v-model="form.username" class="mono" /></el-form-item>
        <el-form-item :label="$t('hosts.password')">
          <el-input v-model="form.password" type="password" show-password class="mono"
                    :placeholder="form.id ? $t('webapp.pwdKeep') : ''" />
        </el-form-item>
        <el-form-item :label="$t('scripts.desc')"><el-input v-model="form.description" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { nextTick, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import i18n from '../i18n'
import { useUserStore } from '../store'

const { t } = i18n.global
const store = useUserStore()
const assets = ref([])
const loading = ref(false)
const openVisible = ref(false)
const current = ref({})
const pwd = ref('')
const pwdVisible = ref(false)
const editVisible = ref(false)
const form = ref({})

const load = async () => {
  loading.value = true
  try { assets.value = await api.get('/webassets') } finally { loading.value = false }
}
onMounted(load)

const openCard = row => {
  current.value = row
  pwd.value = ''
  pwdVisible.value = false
  openVisible.value = true
}
// reveal the vaulted password on the confirm card (admin, audited)
const revealPwd = async () => {
  if (pwdVisible.value) { pwdVisible.value = false; return }
  const r = await api.post(`/webassets/${current.value.id}/reveal`)
  pwd.value = r.password
  pwdVisible.value = true
}
// audited "open": records who opened which asset, then opens the target in a new tab.
// The blank window is opened synchronously inside the click gesture (before the await),
// otherwise browsers treat the post-await window.open as a blocked popup.
const confirmOpen = async () => {
  const w = window.open('about:blank', '_blank')
  try {
    const r = await api.post(`/webassets/${current.value.id}/open`)
    if (w) w.location = r.url
    openVisible.value = false
  } catch (e) {
    w?.close()
    throw e
  }
}
const openOriginal = () => confirmOpen()

// ---- headless-browser remote session (PAM phase B) ----
const streamVisible = ref(false)
const streamConnecting = ref(false)
const streamStage = ref('')
const streamCanvas = ref(null)
let streamWS = null

const startStream = async () => {
  streamConnecting.value = true
  streamStage.value = ''
  streamVisible.value = true
  await nextTick()
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  streamWS = new WebSocket(`${proto}://${location.host}/api/webassets/${current.value.id}/stream?token=${localStorage.getItem('token')}`)
  streamWS.onmessage = ev => {
    const m = JSON.parse(ev.data)
    if (m.type === 'frame') {
      const img = new Image()
      img.onload = () => {
        const c = streamCanvas.value
        if (!c) return
        c.width = img.width
        c.height = img.height
        c.getContext('2d').drawImage(img, 0, 0)
      }
      img.src = 'data:image/jpeg;base64,' + m.data
    } else if (m.type === 'status') streamStage.value = m.stage
    else if (m.type === 'error') ElMessage.error(m.message)
    else if (m.type === 'end') streamWS?.close()
  }
  streamWS.onopen = () => {
    streamConnecting.value = false
    streamCanvas.value?.focus()
  }
}
const stopStream = () => {
  try { streamWS?.close() } catch { /* ignore */ }
  streamWS = null
}
const streamPos = e => {
  const c = streamCanvas.value
  const sx = c.width / c.clientWidth
  const sy = c.height / c.clientHeight
  const r = c.getBoundingClientRect()
  return { x: Math.round((e.clientX - r.left) * sx), y: Math.round((e.clientY - r.top) * sy) }
}
const onStreamMouse = (e, phase) => {
  const p = streamPos(e)
  streamWS?.readyState === WebSocket.OPEN && streamWS.send(JSON.stringify({ t: phase === 'down' ? 'press' : 'release', x: p.x, y: p.y }))
}
const onStreamClick = e => {
  const p = streamPos(e)
  streamWS?.readyState === WebSocket.OPEN && streamWS.send(JSON.stringify({ t: 'click', x: p.x, y: p.y }))
  streamCanvas.value?.focus()
}
const onStreamWheel = e => {
  const p = streamPos(e)
  streamWS?.readyState === WebSocket.OPEN && streamWS.send(JSON.stringify({ t: 'wheel', x: p.x, y: p.y, dy: e.deltaY }))
}
const onStreamKey = e => {
  if (streamWS?.readyState !== WebSocket.OPEN) return
  if (e.key.length === 1) streamWS.send(JSON.stringify({ t: 'text', text: e.key }))
  else streamWS.send(JSON.stringify({ t: 'key', key: e.key }))
}
const dlg = row => {
  form.value = row
    ? { ...row, password: '' }
    : { name: '', url: '', username: '', password: '', description: '' }
  editVisible.value = true
}
const save = async () => {
  if (!form.value.name || !form.value.url) { ElMessage.warning(t('webapp.needNameUrl')); return }
  if (form.value.id) await api.put(`/webassets/${form.value.id}`, form.value)
  else await api.post('/webassets', form.value)
  ElMessage.success(t('common.success'))
  editVisible.value = false
  load()
}
const del = async row => {
  await api.delete(`/webassets/${row.id}`)
  ElMessage.success(t('common.success'))
  load()
}
</script>
