<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px; display:flex; gap:8px">
        <el-button type="primary" @click="dlg()">{{ $t('danger.create') }}</el-button>
        <el-button @click="load">{{ $t('common.refresh') }}</el-button>
      </div>
      <el-alert type="warning" :closable="false" style="margin-bottom:12px" :title="$t('danger.banner')" />
      <el-table :data="rules" v-loading="loading" size="small" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="pattern" :label="$t('danger.pattern')" min-width="260" class-name="mono" />
        <el-table-column prop="desc" :label="$t('danger.desc')" min-width="180" />
        <el-table-column :label="$t('danger.enabled')" width="90">
          <template #default="{ row }">
            <el-switch :model-value="row.enabled" @change="v => toggle(row, v)" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="150" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link @click="dlg(row)">{{ $t('common.edit') }}</el-button>
            <el-popconfirm :title="$t('danger.delConfirm')" @confirm="del(row)">
              <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="visible" :title="form.id ? $t('common.edit') : $t('danger.create')" width="520px">
      <el-form label-width="110px">
        <el-form-item :label="$t('danger.pattern')"><el-input v-model="form.pattern" class="mono" /></el-form-item>
        <el-form-item :label="$t('danger.desc')"><el-input v-model="form.desc" /></el-form-item>
        <el-form-item :label="$t('danger.enabled')"><el-switch v-model="form.enabled" /></el-form-item>
        <el-form-item :label="$t('danger.test')">
          <el-input v-model="testCmd" class="mono" :placeholder="$t('danger.testPlaceholder')" @input="testRule" />
          <el-tag v-if="hit !== null" :type="hit ? 'danger' : 'success'" size="small" style="margin-top:6px">
            {{ hit ? $t('danger.hit') : $t('danger.notHit') }}
          </el-tag>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const rules = ref([])
const loading = ref(false)
const visible = ref(false)
const form = ref({})
const testCmd = ref('')
const hit = ref(null)

const load = async () => {
  loading.value = true
  try { rules.value = await api.get('/danger_rules') } finally { loading.value = false }
}
onMounted(load)

const testRule = () => {
  try {
    hit.value = new RegExp(form.value.pattern).test(testCmd.value)
  } catch { hit.value = null }
}

const dlg = row => {
  form.value = row ? { ...row } : { pattern: '', desc: '', enabled: true }
  testCmd.value = ''
  hit.value = null
  visible.value = true
}
const save = async () => {
  if (!form.value.pattern) { ElMessage.warning(t('danger.patternRequired')); return }
  try { new RegExp(form.value.pattern) } catch (e) { ElMessage.error(t('danger.invalidRegex') + ': ' + e.message); return }
  if (form.value.id) await api.put(`/danger_rules/${form.value.id}`, form.value)
  else await api.post('/danger_rules', form.value)
  ElMessage.success(t('hosts.saved'))
  visible.value = false
  load()
}
const toggle = async (row, v) => {
  await api.put(`/danger_rules/${row.id}`, { ...row, enabled: v })
  load()
}
const del = async row => { await api.delete(`/danger_rules/${row.id}`); load() }
</script>
