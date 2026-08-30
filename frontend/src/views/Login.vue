<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <h2 style="text-align:center">⚙️ {{ systemName }}</h2>
      <div style="text-align:center; color:#909399; font-size:12px; margin-bottom:18px">By JJ Zhang · Version 1.0</div>
      <el-form @keyup.enter="doLogin">
        <el-form-item>
          <el-input v-model="form.username" :placeholder="$t('login.username')" size="large">
            <template #prefix><el-icon><User /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.password" type="password" show-password :placeholder="$t('login.password')" size="large">
            <template #prefix><el-icon><Lock /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-button type="primary" size="large" style="width:100%" :loading="loading" @click="doLogin">{{ $t('login.submit') }}</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import i18n from '../i18n'
import { useUserStore } from '../store'

const { t } = i18n.global
const router = useRouter()
const store = useUserStore()
const form = ref({ username: '', password: '' })
const loading = ref(false)
const systemName = ref(localStorage.getItem('system_name') || 'AutoOps 运维平台')

api.get('/system/info').then(info => {
  systemName.value = info.system_name || systemName.value
  localStorage.setItem('system_name', info.system_name || 'AutoOps 运维平台')
}).catch(() => {})

const doLogin = async () => {
  if (!form.value.username || !form.value.password) return
  loading.value = true
  try {
    const data = await api.post('/login', form.value)
    store.setLogin(data.token, data.user)
    router.push('/')
  } finally { loading.value = false }
}
</script>

<style scoped>
.login-wrap {
  height: 100vh; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #1d2935 0%, #2c3e50 60%, #3a6073 100%);
}
.login-card { width: 380px; padding: 10px 10px 0; }
</style>
