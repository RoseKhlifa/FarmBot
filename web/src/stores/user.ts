import { useStorage } from '@vueuse/core'
import { defineStore } from 'pinia'
import { computed } from 'vue'
import { userApi } from '@/api'

export interface UserCard {
  code: string
  description: string
  days: number
  durationValue?: number
  durationUnit?: 'hour' | 'day'
  durationMs?: number | null
  isPermanent?: boolean
  expiresAt: number | null
  enabled: boolean
}

export interface User {
  username: string
  role: 'admin' | 'super_admin' | 'user'
  card: UserCard | null
  accountLimit: number
  avatar?: string
  mustChangePassword?: boolean
}

export interface LoginResult {
  ok: boolean
  error?: string
  errorType?: 'rate_limit' | 'locked' | 'invalid_credentials'
  remainingMs?: number
  data?: {
    token: string
    role: User['role']
    card: UserCard | null
    accountLimit: number
    user: { username: string }
    mustChangePassword?: boolean
  }
}

export type { CardDurationLike } from '@/utils/card-format'

export const useUserStore = defineStore('user', () => {
  const token = useStorage('admin_token', '')
  const userInfo = useStorage<User | null>('user_info', null)
  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => userInfo.value?.role === 'admin' || userInfo.value?.role === 'super_admin')
  const isSuperAdmin = computed(() => userInfo.value?.role === 'super_admin')
  const username = computed(() => userInfo.value?.username || '')
  const userCard = computed(() => userInfo.value?.card)
  const accountLimit = computed(() => userInfo.value?.accountLimit ?? 2)
  const avatar = computed(() => userInfo.value?.avatar || '')

  // 检查用户是否过期
  const isExpired = computed(() => {
    if (!userInfo.value?.card?.expiresAt)
      return false
    return Date.now() > userInfo.value.card.expiresAt
  })

  // 获取过期时间显示
  const expireTimeText = computed(() => {
    if (!userInfo.value?.card)
      return '无卡密'
    if (userInfo.value.card.isPermanent === true || userInfo.value.card.days === -1)
      return '永久有效'
    if (!userInfo.value.card.expiresAt)
      return '未激活'
    const date = new Date(userInfo.value.card.expiresAt)
    return date.toLocaleString('zh-CN')
  })

  async function login(username: string, password: string): Promise<LoginResult> {
    try {
      const res = await userApi.login({ username, password })
      if (res.data.ok) {
        token.value = res.data.data.token
        userInfo.value = {
          username: res.data.data.user.username,
          role: res.data.data.role,
          card: res.data.data.card,
          accountLimit: res.data.data.accountLimit ?? 2,
          mustChangePassword: res.data.data.mustChangePassword,
        }
      }
      return res.data
    }
    catch (error: any) {
      const data = error.response?.data
      if (data) {
        return {
          ok: false,
          error: data.error,
          errorType: data.errorType,
          remainingMs: data.remainingMs,
        }
      }
      return { ok: false, error: error.message || '网络错误' }
    }
  }

  async function register(username: string, password: string, cardCode: string) {
    const res = await userApi.register({ username, password, cardCode })
    return res.data
  }

  async function logout() {
    try {
      await userApi.logout()
    }
    finally {
      token.value = ''
      userInfo.value = null
    }
  }

  async function fetchUserInfo() {
    try {
      const res = await userApi.getCurrentUser()
      if (res.data.ok) {
        userInfo.value = res.data.data
      }
      return res.data
    }
    catch {
      return { ok: false }
    }
  }

  async function renew(cardCode: string) {
    const res = await userApi.renewUser(cardCode)
    if (res.data.ok) {
      // 更新本地用户信息
      if (userInfo.value) {
        userInfo.value.card = res.data.data.card
        userInfo.value.accountLimit = res.data.data.accountLimit
      }
    }
    return res.data
  }

  async function changePassword(oldPassword: string, newPassword: string) {
    const res = await userApi.changePassword({ oldPassword, newPassword })
    return res.data
  }

  async function verifyResetPassword(username: string, cardCode: string) {
    const res = await userApi.verifyPasswordReset({ username, cardCode })
    return res.data
  }

  async function resetPassword(username: string, cardCode: string, newPassword: string) {
    const res = await userApi.confirmPasswordReset({ username, cardCode, newPassword })
    return res.data
  }

  return {
    token,
    userInfo,
    isLoggedIn,
    isAdmin,
    isSuperAdmin,
    username,
    userCard,
    accountLimit,
    avatar,
    isExpired,
    expireTimeText,
    login,
    register,
    logout,
    fetchUserInfo,
    renew,
    changePassword,
    verifyResetPassword,
    resetPassword,
  }
})
