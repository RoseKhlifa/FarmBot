import type { InjectionKey } from 'vue'
import type { PasswordStrength } from '@/composables/usePasswordStrength'
import { computed, inject, onMounted, onScopeDispose, reactive, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { cardApi, systemApi, userApi } from '@/api'
import { getPasswordStrength } from '@/composables/usePasswordStrength'
import { formatTimeDuration } from '@/stores/admin'
import { useAppStore } from '@/stores/app'
import { useUserStore } from '@/stores/user'

const USERNAME_RE = /^\w+$/

export interface ClaimModalContent {
  success: boolean
  title: string
  message: string
  cardCode: string
  days: number
}

interface RenewalResponse {
  cardType?: string
  card?: {
    expiresAt?: number | string | null
  }
}

interface ClaimResponse {
  cardCode?: string
  days?: number
  durationValue?: number
  durationUnit?: 'hour' | 'day'
  durationMs?: number | null
  isPermanent?: boolean
  error?: string
}

export interface AuthFlow {
  gameVersion: string
  loginLinks: {
    logoUrl: string
    title: string
    loginSubtitle: string
    registerSubtitle: string
    purchaseUrl: string
    qqGroupUrl: string
  }
  showUpdateLog: boolean
  logoLoadFailed: boolean
  isLogin: boolean
  username: string
  password: string
  cardCode: string
  error: string
  success: string
  loading: boolean
  showPasswordStrength: boolean
  lockoutRemaining: number
  rateLimitRemaining: number
  cardClaimEnabled: boolean
  cardClaimLoading: boolean
  showClaimModal: boolean
  claimModalContent: ClaimModalContent
  showResetVerifyModal: boolean
  showResetPasswordModal: boolean
  resetUsername: string
  resetCardCode: string
  resetNewPassword: string
  resetConfirmPassword: string
  resetError: string
  resetLoading: boolean
  resetPasswordTouched: boolean
  showRenewalModal: boolean
  renewalUsername: string
  renewalCardCode: string
  renewalError: string
  renewalSuccess: string
  renewalLoading: boolean
  passwordStrength: PasswordStrength
  resetPasswordStrength: PasswordStrength
  usernameValid: { valid: boolean, message: string }
  handleSubmit: () => Promise<void>
  toggleMode: () => void
  openRenewal: () => void
  closeRenewalModal: () => void
  submitRenewal: () => Promise<void>
  openResetVerifyModal: () => void
  closeResetVerifyModal: () => void
  closeResetPasswordModal: () => void
  verifyResetPassword: () => Promise<void>
  submitResetPassword: () => Promise<void>
  claimFreeCard: () => Promise<void>
  closeClaimModal: () => void
}

export const authFlowKey: InjectionKey<AuthFlow> = Symbol('farmbot-auth-flow')

function responseError(error: unknown, fallback: string) {
  const value = error as {
    response?: { data?: { error?: string, errorType?: string, remainingMs?: number } }
    message?: string
  }
  return value.response?.data || { error: value.message || fallback }
}

export function useAuthFlow(): AuthFlow {
  const userStore = useUserStore()
  const appStore = useAppStore()
  const route = useRoute()
  const router = useRouter()
  let navigationTimer: ReturnType<typeof setTimeout> | undefined

  onScopeDispose(() => {
    if (navigationTimer !== undefined) {
      clearTimeout(navigationTimer)
      navigationTimer = undefined
    }
  })

  const flow = reactive({
    gameVersion: '',
    loginLinks: computed(() => appStore.loginPageConfig),
    showUpdateLog: false,
    logoLoadFailed: false,
    isLogin: true,
    username: '',
    password: '',
    cardCode: '',
    error: '',
    success: '',
    loading: false,
    showPasswordStrength: false,
    lockoutRemaining: 0,
    rateLimitRemaining: 0,
    cardClaimEnabled: false,
    cardClaimLoading: false,
    showClaimModal: false,
    claimModalContent: {
      success: true,
      title: '',
      message: '',
      cardCode: '',
      days: 0,
    },
    showResetVerifyModal: false,
    showResetPasswordModal: false,
    resetUsername: '',
    resetCardCode: '',
    resetNewPassword: '',
    resetConfirmPassword: '',
    resetError: '',
    resetLoading: false,
    resetPasswordTouched: false,
    showRenewalModal: false,
    renewalUsername: '',
    renewalCardCode: '',
    renewalError: '',
    renewalSuccess: '',
    renewalLoading: false,
    passwordStrength: computed(() => getPasswordStrength(flow.password)),
    resetPasswordStrength: computed(() => getPasswordStrength(flow.resetNewPassword)),
    usernameValid: computed(() => {
      const name = flow.username
      if (!name)
        return { valid: false, message: '' }
      if (name.length < 3)
        return { valid: false, message: '用户名至少3位' }
      if (name.length > 32)
        return { valid: false, message: '用户名最多32位' }
      if (!USERNAME_RE.test(name))
        return { valid: false, message: '只能包含字母、数字、下划线' }
      return { valid: true, message: '' }
    }),
    handleSubmit: async () => {},
    toggleMode: () => {},
    openRenewal: () => {},
    closeRenewalModal: () => {},
    submitRenewal: async () => {},
    openResetVerifyModal: () => {},
    closeResetVerifyModal: () => {},
    closeResetPasswordModal: () => {},
    verifyResetPassword: async () => {},
    submitResetPassword: async () => {},
    claimFreeCard: async () => {},
    closeClaimModal: () => {},
  }) as AuthFlow

  function validateForm() {
    if (!flow.username) {
      flow.error = '请输入用户名'
      return false
    }

    if (!flow.usernameValid.valid) {
      flow.error = flow.usernameValid.message
      return false
    }

    if (!flow.password) {
      flow.error = '请输入密码'
      return false
    }

    if (!flow.isLogin) {
      if (flow.password.length < 6) {
        flow.error = '密码长度至少6位'
        return false
      }

      if (!flow.passwordStrength.valid) {
        flow.error = '密码强度不足：需包含大写字母、小写字母、数字、特殊符号中的至少两种'
        return false
      }

      if (!flow.cardCode) {
        flow.error = '请输入卡密'
        return false
      }
    }

    return true
  }

  flow.handleSubmit = async () => {
    if (!validateForm())
      return

    flow.loading = true
    flow.error = ''
    flow.success = ''

    try {
      if (flow.isLogin) {
        const result = await userStore.login(flow.username, flow.password)
        if (result.ok) {
          if (result.data?.mustChangePassword)
            flow.success = '登录成功！请修改默认密码以确保账户安全'
          if (navigationTimer !== undefined)
            clearTimeout(navigationTimer)
          navigationTimer = setTimeout(() => {
            navigationTimer = undefined
            void router.push({ name: 'dashboard' })
          }, 500)
        }
        else if (result.errorType === 'rate_limit') {
          flow.error = result.error || '请求过于频繁，请稍后重试'
          if (result.remainingMs)
            flow.rateLimitRemaining = Math.ceil(result.remainingMs / 1000)
        }
        else if (result.errorType === 'locked') {
          flow.error = result.error || '账户已被锁定'
          if (result.remainingMs)
            flow.lockoutRemaining = Math.ceil(result.remainingMs / 1000 / 60)
        }
        else {
          flow.error = result.error || '登录失败'
        }
      }
      else {
        const result = await userStore.register(flow.username, flow.password, flow.cardCode)
        if (result.ok) {
          flow.success = '注册成功，请登录'
          flow.isLogin = true
          flow.cardCode = ''
          flow.password = ''
        }
        else {
          flow.error = result.error || '注册失败'
        }
      }
    }
    catch (error) {
      const data = responseError(error, '操作异常')
      if (data.errorType === 'rate_limit') {
        flow.error = data.error || '请求过于频繁'
        if (data.remainingMs)
          flow.rateLimitRemaining = Math.ceil(data.remainingMs / 1000)
      }
      else if (data.errorType === 'locked') {
        flow.error = data.error || '账户已被锁定'
        if (data.remainingMs)
          flow.lockoutRemaining = Math.ceil(data.remainingMs / 1000 / 60)
      }
      else {
        flow.error = data.error || '操作异常'
      }
    }
    finally {
      flow.loading = false
    }
  }

  flow.toggleMode = () => {
    flow.isLogin = !flow.isLogin
    flow.error = ''
    flow.success = ''
    flow.showPasswordStrength = false
    flow.lockoutRemaining = 0
    flow.rateLimitRemaining = 0
  }

  flow.openRenewal = () => {
    flow.renewalUsername = flow.username.trim()
    flow.renewalCardCode = ''
    flow.renewalError = ''
    flow.renewalSuccess = ''
    flow.showRenewalModal = true
  }

  flow.closeRenewalModal = () => {
    if (flow.renewalLoading)
      return
    flow.showRenewalModal = false
    flow.renewalError = ''
    flow.renewalSuccess = ''
  }

  flow.submitRenewal = async () => {
    if (!flow.renewalUsername.trim()) {
      flow.renewalError = '请输入用户名'
      return
    }
    if (!flow.renewalCardCode.trim()) {
      flow.renewalError = '请输入卡密'
      return
    }

    flow.renewalLoading = true
    flow.renewalError = ''
    flow.renewalSuccess = ''
    try {
      const { data } = await userApi.publicRenew<RenewalResponse>({
        username: flow.renewalUsername.trim(),
        cardCode: flow.renewalCardCode.trim(),
      })
      if (!data.ok) {
        flow.renewalError = data.error || '续费失败'
        return
      }

      const cardType = data.data?.cardType
      const card = data.data?.card
      flow.renewalSuccess = cardType === 'quota'
        ? '续费成功，账号额度已更新'
        : `续费成功，有效期已更新${card?.expiresAt ? `至 ${new Date(card.expiresAt).toLocaleString('zh-CN')}` : ''}`
      flow.username = flow.renewalUsername.trim()
    }
    catch (error) {
      const data = responseError(error, '续费失败')
      flow.renewalError = data.error || '续费失败'
    }
    finally {
      flow.renewalLoading = false
    }
  }

  flow.openResetVerifyModal = () => {
    flow.resetUsername = flow.username.trim()
    flow.resetCardCode = ''
    flow.resetNewPassword = ''
    flow.resetConfirmPassword = ''
    flow.resetError = ''
    flow.resetPasswordTouched = false
    flow.showResetVerifyModal = true
  }

  flow.closeResetVerifyModal = () => {
    if (flow.resetLoading)
      return
    flow.showResetVerifyModal = false
    flow.resetError = ''
  }

  flow.closeResetPasswordModal = () => {
    if (flow.resetLoading)
      return
    flow.showResetPasswordModal = false
    flow.resetNewPassword = ''
    flow.resetConfirmPassword = ''
    flow.resetError = ''
    flow.resetPasswordTouched = false
  }

  flow.verifyResetPassword = async () => {
    if (!flow.resetUsername.trim()) {
      flow.resetError = '请输入用户名'
      return
    }
    if (!flow.resetCardCode.trim()) {
      flow.resetError = '请输入注册时使用的卡密'
      return
    }

    flow.resetLoading = true
    flow.resetError = ''
    try {
      const { data } = await userApi.verifyPasswordReset({
        username: flow.resetUsername.trim(),
        cardCode: flow.resetCardCode.trim(),
      })
      if (!data.ok) {
        flow.resetError = data.error || '验证失败'
        return
      }
      flow.showResetVerifyModal = false
      flow.showResetPasswordModal = true
    }
    catch (error) {
      const data = responseError(error, '验证失败')
      flow.resetError = data.error || '验证失败'
    }
    finally {
      flow.resetLoading = false
    }
  }

  flow.submitResetPassword = async () => {
    flow.resetPasswordTouched = true
    if (!flow.resetNewPassword) {
      flow.resetError = '请输入新密码'
      return
    }
    if (flow.resetNewPassword.length < 6) {
      flow.resetError = '密码长度至少6位'
      return
    }
    if (!flow.resetPasswordStrength.valid) {
      flow.resetError = '密码强度不足：需包含大写字母、小写字母、数字、特殊符号中的至少两种'
      return
    }
    if (flow.resetNewPassword !== flow.resetConfirmPassword) {
      flow.resetError = '两次输入的密码不一致'
      return
    }

    flow.resetLoading = true
    flow.resetError = ''
    try {
      const { data } = await userApi.confirmPasswordReset({
        username: flow.resetUsername.trim(),
        cardCode: flow.resetCardCode.trim(),
        newPassword: flow.resetNewPassword,
      })
      if (!data.ok) {
        flow.resetError = data.error || '重置失败'
        return
      }
      flow.showResetPasswordModal = false
      flow.username = flow.resetUsername.trim()
      flow.password = ''
      flow.isLogin = true
      flow.success = '密码重置成功，请使用新密码登录'
      flow.resetNewPassword = ''
      flow.resetConfirmPassword = ''
    }
    catch (error) {
      const data = responseError(error, '重置失败')
      flow.resetError = data.error || '重置失败'
    }
    finally {
      flow.resetLoading = false
    }
  }

  flow.claimFreeCard = async () => {
    if (flow.cardClaimLoading)
      return

    flow.cardClaimLoading = true
    flow.error = ''
    try {
      const { data } = await cardApi.claimCard<ClaimResponse>()
      const payload = data as typeof data & ClaimResponse
      if (payload.ok) {
        flow.cardCode = payload.cardCode || ''
        flow.claimModalContent = {
          success: true,
          title: '领取成功',
          message: `成功领取 ${formatTimeDuration(payload)}卡密！`,
          cardCode: payload.cardCode || '',
          days: payload.days || 0,
        }
      }
      else {
        flow.claimModalContent = {
          success: false,
          title: '领取失败',
          message: payload.error || '领取失败，请稍后重试',
          cardCode: '',
          days: 0,
        }
      }
      flow.showClaimModal = true
    }
    catch (error) {
      const data = responseError(error, '领取失败')
      flow.claimModalContent = {
        success: false,
        title: '领取失败',
        message: data.error || '领取失败',
        cardCode: '',
        days: 0,
      }
      flow.showClaimModal = true
    }
    finally {
      flow.cardClaimLoading = false
    }
  }

  flow.closeClaimModal = () => {
    flow.showClaimModal = false
  }

  async function checkCardClaimStatus() {
    try {
      const { data } = await cardApi.getClaimStatus()
      flow.cardClaimEnabled = data.ok && data.enabled === true
    }
    catch (error) {
      console.error('检查卡密领取状态失败:', error)
    }
  }

  async function fetchGameVersion() {
    try {
      const { data } = await systemApi.getGameVersion()
      if (data.ok)
        flow.gameVersion = String(data.clientVersion || '')
    }
    catch (error) {
      console.error('获取游戏版本失败:', error)
    }
  }

  watch(() => flow.password, () => {
    if (!flow.isLogin && flow.password)
      flow.showPasswordStrength = true
  })

  watch(() => String(route.query.username || '').trim(), (value) => {
    if (value && !flow.username.trim())
      flow.username = value
  }, { immediate: true })

  watch(() => flow.loginLinks.logoUrl, () => {
    flow.logoLoadFailed = false
  })

  onMounted(() => {
    void checkCardClaimStatus()
    void fetchGameVersion()
    void appStore.fetchLoginPageConfig()
  })

  return flow
}

export function useAuthFlowContext() {
  const flow = inject(authFlowKey)
  if (!flow)
    throw new Error('useAuthFlowContext must be used inside Login.vue')
  return flow
}
