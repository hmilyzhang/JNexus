<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<!-- Unified asset workspace: hosts, databases and web apps as tabs -->
<template>
  <div class="assets-page">
    <div class="assets-head">
      <el-tabs v-model="activeTab" class="assets-tabs" @tab-change="onTab">
        <el-tab-pane :label="$t('assets.tabHosts')" name="hosts" />
        <el-tab-pane v-if="tabAllowed('databases')" :label="$t('assets.tabDatabases')" name="databases" />
        <el-tab-pane v-if="tabAllowed('webapps')" :label="$t('assets.tabApps')" name="webapps" />
      </el-tabs>
      <el-dropdown v-if="started" trigger="click" @command="onAddCommand">
        <el-button type="primary">{{ $t('assets.add') }}<el-icon style="margin-left:4px"><ArrowDown /></el-icon></el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="hosts">{{ $t('assets.tabHosts') }}</el-dropdown-item>
            <el-dropdown-item v-if="tabAllowed('databases')" command="databases">{{ $t('assets.tabDatabases') }}</el-dropdown-item>
            <el-dropdown-item v-if="tabAllowed('webapps')" command="webapps">{{ $t('assets.tabApps') }}</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>

    <div class="assets-body">
      <HostsView v-if="activeTab === 'hosts'" ref="hostsRef" />
      <DatabasesView v-else-if="activeTab === 'databases'" ref="dbRef" />
      <WebAppsView v-else-if="activeTab === 'webapps'" ref="appsRef" />
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown } from '@element-plus/icons-vue'
import api from '../api'
import i18n from '../i18n'
import { useUserStore } from '../store'
import HostsView from './Hosts.vue'
import DatabasesView from './Databases.vue'
import WebAppsView from './WebApps.vue'

const { t } = i18n.global
const route = useRoute()
const router = useRouter()
const store = useUserStore()

const activeTab = ref(route.params.tab || 'hosts')
const hostsRef = ref(null)
const dbRef = ref(null)
const appsRef = ref(null)
const started = computed(() => !!route.params.tab)

const roleMenus = ref(null)
api.get('/system/roles').then(rs => { roleMenus.value = rs }).catch(() => {})

// tab visibility for non-admin roles follows the role's menu grants
const tabAllowed = key => {
  if (store.isAdmin) return true
  if (key === 'hosts') return true // primary tab: visible when the menu itself is
  const conf = (roleMenus.value || {})[store.role]
  return !conf || !Array.isArray(conf.menus) || conf.menus.includes(key)
}

const onTab = name => {
  if (route.params.tab !== name) router.replace({ path: `/assets/${name}` })
}
const onAddCommand = type => {
  activeTab.value = type
  if (route.params.tab !== type) router.replace({ path: `/assets/${type}` })
  nextTick(() => setTimeout(() => {
    const ref = { hosts: hostsRef, databases: dbRef, webapps: appsRef }[type]
    ref?.value?.openCreate?.()
  }, 250))
}

watch(() => route.params.tab, v => { if (v && v !== activeTab.value) activeTab.value = v })
</script>

<style scoped>
.assets-page { display: flex; flex-direction: column; height: calc(100vh - 92px); }
.assets-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex: 0 0 auto; }
.assets-tabs { flex: 1; }
.assets-tabs :deep(.el-tabs__header) { margin-bottom: 0; }
.assets-body { flex: 1 1 0; min-height: 0; margin-top: 8px; overflow: auto; }
</style>
