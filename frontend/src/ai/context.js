// JNexus Ops Platform — By JJ Zhang, Version 1.0
// AI context registration module: each page calls setAIContext() after its data loads
// to inject a page summary; the AI panel reads it automatically when sending.
import { reactive } from 'vue'

export const aiContext = reactive({
  page: '',    // Current page identifier (e.g. 'monitor')
  summary: '', // Compact text summary of the page data
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
