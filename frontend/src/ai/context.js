// JNexus 运维平台 — By JJ Zhang, Version 1.0
// AI 上下文注册模块：各页面在数据加载后调用 setAIContext() 注入页面摘要，
// AI 面板发送时自动读取，让回答具有页面感知能力。
import { reactive } from 'vue'

export const aiContext = reactive({
  page: '',    // 当前页面标识（如 'monitor'）
  summary: '', // 页面数据的紧凑文字摘要
})

export function setAIContext(page, summary) {
  aiContext.page = page
  aiContext.summary = summary
}

export function clearAIContext() {
  aiContext.page = ''
  aiContext.summary = ''
}

export function getAIContextText() {
  if (!aiContext.summary) return ''
  return `[${aiContext.page}]\n${aiContext.summary}`
}
