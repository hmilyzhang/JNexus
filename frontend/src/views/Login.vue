<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <h2 style="text-align:center">⚙️ AutoOps 运维平台</h2>
      <el-form @keyup.enter="doLogin">
        <el-form-item>
          <el-input v-model="form.username" placeholder="用户名" size="large">
            <template #prefix><el-icon><User /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.password" type="password" show-password placeholder="密码" size="large">
            <template #prefix><el-icon><Lock /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-button type="primary" size="large" style="width:100%" :loading="loading" @click="doLogin">登 录</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { useUserStore } from '../store'

const router = useRouter()
const store = useUserStore()
const form = ref({ username: '', password: '' })
const loading = ref(false)

const doLogin = async () => {
  if (!form.value.username || !form.value.password) return
  loading.value = true
  try {
    const data = await api.post('/login', form.value)
    store.setLogin(data.token, data.user)
    router.push('/')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap {
  height: 100vh; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #1d2935 0%, #2c3e50 60%, #3a6073 100%);
}
.login-card { width: 380px; padding: 10px 10px 0; }
</style>
