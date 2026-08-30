// AutoOps 运维平台 — By JJ Zhang, Version 1.0
import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from './router'

const api = axios.create({ baseURL: '/api', timeout: 60000 })

api.interceptors.request.use(cfg => {
  const token = localStorage.getItem('token')
  if (token) cfg.headers.Authorization = `Bearer ${token}`
  return cfg
})

api.interceptors.response.use(
  resp => resp.data,
  err => {
    const status = err.response?.status
    const msg = err.response?.data?.error || err.message
    if (status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      if (router.currentRoute.value.name !== 'login') router.push('/login')
    }
    ElMessage.error(msg)
    return Promise.reject(err)
  }
)

export default api
