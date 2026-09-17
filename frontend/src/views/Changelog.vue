<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<!-- Changelog: release notes with a short summary per update. Read for everyone, admin-managed. -->
<template>
  <el-card v-loading="loading">
    <template #header>
      <div style="display:flex; align-items:center; gap:10px">
        <span style="font-weight:600">{{ $t('menu.changelog') }}</span>
        <el-tag size="small" type="success" effect="plain">{{ $t('changelog.now') }} v{{ curVersion }}</el-tag>
        <span style="flex:1"></span>
        <el-button v-if="isAdmin" size="small" type="primary" @click="openDlg()">{{ $t('changelog.add') }}</el-button>
      </div>
    </template>

    <el-empty v-if="!rows.length" :description="$t('changelog.empty')" :image-size="90" />
    <el-timeline v-else style="padding-left:6px">
      <el-timeline-item v-for="row in rows" :key="row.id"
                        :timestamp="fmtDate(row.released_at)" placement="top" type="primary">
        <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap">
          <el-tag v-if="row.version" size="small">v{{ row.version }}</el-tag>
          <span style="font-weight:600">{{ row.title }}</span>
          <span style="flex:1"></span>
          <template v-if="isAdmin">
            <el-button size="small" link type="primary" @click="openDlg(row)">{{ $t('common.edit') }}</el-button>
            <el-button size="small" link type="danger" @click="del(row.id)">{{ $t('common.delete') }}</el-button>
          </template>
        </div>
        <div v-if="row.details" style="color:var(--el-text-color-regular); font-size:13px; white-space:pre-line; margin-top:4px; line-height:1.7">
          {{ row.details }}
        </div>
      </el-timeline-item>
    </el-timeline>

    <!-- create / edit dialog (admin) -->
    <el-dialog v-model="dlgVisible" :title="form.id ? $t('changelog.edit') : $t('changelog.add')" width="520px">
      <el-form label-width="90px">
        <el-form-item :label="$t('changelog.version')">
          <el-input v-model="form.version" placeholder="1.281" style="width:160px" />
        </el-form-item>
        <el-form-item :label="$t('changelog.title')" required>
          <el-input v-model="form.title" :placeholder="$t('changelog.titlePh')" />
        </el-form-item>
        <el-form-item :label="$t('changelog.details')">
          <el-input v-model="form.details" type="textarea" :rows="4" :placeholder="$t('changelog.detailsPh')" />
        </el-form-item>
        <el-form-item :label="$t('changelog.date')">
          <el-date-picker v-model="form.released_at" type="date" value-format="YYYY-MM-DD" style="width:180px" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import i18n from '../i18n'
import { useUserStore } from '../store'

const { t } = i18n.global
const store = useUserStore()
const isAdmin = store.isAdmin

const rows = ref([])
const curVersion = ref('-')
const loading = ref(false)
const dlgVisible = ref(false)
const saving = ref(false)
const form = ref({})

const load = async () => {
  loading.value = true
  try {
    const r = await api.get('/changelog')
    rows.value = r.items || []
    curVersion.value = r.version || '-'
  } finally { loading.value = false }
}

const openDlg = row => {
  form.value = row
    ? { id: row.id, version: row.version, title: row.title, details: row.details,
        released_at: (row.released_at || '').slice(0, 10) }
    : { id: '', version: '', title: '', details: '', released_at: new Date().toISOString().slice(0, 10) }
  dlgVisible.value = true
}

const save = async () => {
  if (!form.value.title.trim()) { ElMessage.warning(t('changelog.titlePh')); return }
  saving.value = true
  try {
    if (form.value.id) await api.put(`/changelog/${form.value.id}`, { ...form.value })
    else await api.post('/changelog', { ...form.value })
    ElMessage.success(t('common.success'))
    dlgVisible.value = false
    load()
  } catch { /* interceptor shows the error */ } finally { saving.value = false }
}

const del = async id => {
  try {
    await ElMessageBox.confirm(t('changelog.delConfirm'), t('common.tip'), { type: 'warning' })
  } catch { return }
  try {
    await api.delete(`/changelog/${id}`)
    load()
  } catch { /* interceptor shows the error */ }
}

const fmtDate = v => (v ? String(v).slice(0, 10) : '')

onMounted(load)
</script>
