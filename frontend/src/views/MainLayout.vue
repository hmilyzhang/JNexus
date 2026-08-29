<template>
  <el-container style="height: 100vh">
    <el-aside width="200px" style="background:#1d2935">
      <div class="logo">⚙️ AutoOps</div>
      <el-menu :default-active="$route.path" router background-color="#1d2935" text-color="#a7b1c2"
               active-text-color="#ffffff" style="border-right:none">
        <el-menu-item v-for="m in menus" :key="m.path" :index="m.path">
          <el-icon><component :is="m.icon" /></el-icon>
          <span>{{ m.title }}</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="title">{{ $route.meta.title || 'AutoOps' }}</div>
        <el-dropdown @command="onCmd">
          <span class="user-info">
            <el-icon><User /></el-icon>
            {{ store.user?.username }}（{{ store.roleLabel }}）
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="password">修改密码</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main style="padding:16px; background:#f5f7fa">
        <router-view />
      </el-main>
    </el-container>
  </el-container>

  <el-dialog v-model="pwdVisible" title="修改密码" width="400px">
    <el-form label-width="90px">
      <el-form-item label="旧密码"><el-input v-model="pwdForm.old_password" type="password" show-password /></el-form-item>
      <el-form-item label="新密码"><el-input v-model="pwdForm.new_password" type="password" show-password /></el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="pwdVisible = false">取消</el-button>
      <el-button type="primary" @click="doChangePwd">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../store'

const store = useUserStore()
const router = useRouter()

const menus = computed(() => {
  const items = [
    { path: '/hosts', title: '主机管理', icon: 'Monitor' },
    { path: '/exec', title: '批量执行', icon: 'Promotion' },
    { path: '/tasks', title: '执行记录', icon: 'List' },
    { path: '/files', title: '文件分发', icon: 'FolderOpened' },
    { path: '/scripts', title: '脚本中心', icon: 'Document' },
    { path: '/apps', title: '应用管理', icon: 'Box' },
    { path: '/releases', title: '发布中心', icon: 'UploadFilled' },
    { path: '/users', title: '用户权限', icon: 'User', admin: true },
    { path: '/danger', title: '危险命令规则', icon: 'Warning', admin: true },
    { path: '/audit', title: '审计日志', icon: 'Notebook' }
  ]
  return items.filter(m => !m.admin || store.isAdmin)
})

const pwdVisible = ref(false)
const pwdForm = ref({ old_password: '', new_password: '' })

const onCmd = cmd => {
  if (cmd === 'logout') {
    store.logout()
    router.push('/login')
  } else if (cmd === 'password') {
    pwdForm.value = { old_password: '', new_password: '' }
    pwdVisible.value = true
  }
}

const doChangePwd = async () => {
  await api.post('/change_password', pwdForm.value)
  ElMessage.success('密码已修改')
  pwdVisible.value = false
}
</script>

<style scoped>
.logo {
  color: #fff; font-size: 18px; font-weight: bold;
  padding: 18px 20px; letter-spacing: 1px;
}
.header {
  background: #fff; display: flex; align-items: center; justify-content: space-between;
  box-shadow: 0 1px 4px rgba(0,21,41,.08);
}
.title { font-size: 16px; font-weight: 600; }
.user-info { cursor: pointer; display: flex; align-items: center; gap: 6px; color: #333; font-size: 14px; }
</style>
