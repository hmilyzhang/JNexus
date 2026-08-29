import { defineStore } from 'pinia'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    user: JSON.parse(localStorage.getItem('user') || 'null')
  }),
  getters: {
    role: s => s.user?.role || '',
    isAdmin: s => s.user?.role === 'admin',
    roleLabel: s => ({
      admin: '管理员', ops: '运维', publisher: '发布员', viewer: '只读'
    })[s.user?.role] || s.user?.role || ''
  },
  actions: {
    setLogin(token, user) {
      this.token = token
      this.user = user
      localStorage.setItem('token', token)
      localStorage.setItem('user', JSON.stringify(user))
    },
    logout() {
      this.token = ''
      this.user = null
      localStorage.removeItem('token')
      localStorage.removeItem('user')
    }
  }
})
