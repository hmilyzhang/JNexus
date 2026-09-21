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
        <el-button type="primary" @click="confirmOpen">{{ $t('webapp.confirmOpen') }}</el-button>
      </template>
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
import { onMounted, ref } from 'vue'
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
