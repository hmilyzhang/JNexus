<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div style="height:100vh">
    <K8sShell :cluster-id="pod.clusterId" :namespace="pod.namespace" :pod="pod.name" :container="pod.container" />
  </div>
</template>

<script setup>
import { reactive } from 'vue'
import { useRoute } from 'vue-router'
import K8sShell from '../components/K8sShell.vue'

// Standalone Pod Shell page (kept as a legacy entry point); terminal logic reuses the K8sShell component
const route = useRoute()
const pod = reactive({
  clusterId: route.query.clusterId || '', namespace: route.query.namespace || '',
  name: route.query.pod || '', container: route.query.container || '',
})
</script>
