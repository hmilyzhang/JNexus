<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px; display:flex; gap:8px">
        <el-button type="primary" @click="dlg()">新增规则</el-button>
        <el-button @click="load">刷新</el-button>
      </div>
      <el-alert type="warning" :closable="false" style="margin-bottom:12px"
                title="命中的命令在批量执行/脚本执行/发布启停时会被直接拦截并记录审计日志" />
      <el-table :data="rules" v-loading="loading" size="small" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="pattern" label="正则" min-width="260" class-name="mono" />
        <el-table-column prop="desc" label="说明" min-width="180" />
        <el-table-column label="启用" width="90">
          <template #default="{ row }">
            <el-switch :model-value="row.enabled" @change="v => toggle(row, v)" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link @click="dlg(row)">编辑</el-button>
            <el-popconfirm title="确认删除规则?" @confirm="del(row)">
              <template #reference><el-button size="small" type="danger" link>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="visible" :title="form.id ? '编辑规则' : '新增规则'" width="520px">
      <el-form label-width="90px">
        <el-form-item label="正则表达式"><el-input v-model="form.pattern" class="mono" /></el-form-item>
        <el-form-item label="说明"><el-input v-model="form.desc" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item>
        <el-form-item label="测试">
          <el-input v-model="testCmd" class="mono" placeholder="输入命令测试是否命中" @input="testRule" />
          <el-tag v-if="hit !== null" :type="hit ? 'danger' : 'success'" size="small" style="margin-top:6px">
            {{ hit ? '命中拦截' : '未命中' }}
          </el-tag>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../api'
import { ElMessage } from 'element-plus'

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
  if (!form.value.pattern) { ElMessage.warning('正则必填'); return }
  try { new RegExp(form.value.pattern) } catch (e) { ElMessage.error('非法正则: ' + e.message); return }
  if (form.value.id) await api.put(`/danger_rules/${form.value.id}`, form.value)
  else await api.post('/danger_rules', form.value)
  ElMessage.success('已保存')
  visible.value = false
  load()
}
const toggle = async (row, v) => {
  await api.put(`/danger_rules/${row.id}`, { ...row, enabled: v })
  load()
}
const del = async row => { await api.delete(`/danger_rules/${row.id}`); load() }
</script>
