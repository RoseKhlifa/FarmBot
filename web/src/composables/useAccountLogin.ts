import type { Ref } from 'vue'
import { useIntervalFn, useTimeoutFn } from '@vueuse/core'
import { computed, onScopeDispose, reactive, ref, watch } from 'vue'
import * as accountApi from '@/api/account'
import * as captureApi from '@/api/capture'
import * as systemApi from '@/api/system'
import * as yybApi from '@/api/yyb'

const CODE_QUERY_RE = /[?&]code=([^&]+)/i
const CAPTURE_SUCCESS_STORAGE_KEY = 'capture_login_succeeded'

export type AccountLoginTab = 'capture' | 'manual' | 'yyb' | 'yybqr' | 'yyb3rd'
export type LoginPlatform = 'qq' | 'wx'
export type YybQrStatus = 'idle' | 'loading' | 'pending' | 'scanned' | 'authorizing' | 'success' | 'expired' | 'error'

export interface ThirdPartyAccountConfig {
  apiBase?: string
  apiToken?: string
  openid?: string
  autoReconnect?: boolean
  reconnectDelayMin?: number
  reconnectMaxAttempts?: number
}

export interface AccountEditData {
  id: string | number
  name?: string
  code?: string
  platform?: LoginPlatform
  provider?: string
  thirdparty?: ThirdPartyAccountConfig
}

export interface CaptureFlowState {
  id: string
  platform: LoginPlatform
  codeCaptured: boolean
  accountGid: string
  friendCount: number
  captureStatus: string
  proxy: {
    running: boolean
    status: string
    error: string
  }
  publicInfo: {
    host: string
    mitmPort: number
    remainingSec: number
    certificateUrl: string
  }
}

export interface YybAccount {
  openid: string
  nickname?: string
  alias?: string
  status?: string
}

interface WxConfig {
  [key: string]: unknown
  apiBase?: string
  apiKey?: string
  appId?: string
  enabled?: boolean
  autoReconnect?: boolean
  reconnectDelayMin?: number
  reconnectMaxAttempts?: number
  confirmed?: boolean
}

interface CaptureConfig {
  enabled?: boolean
}

interface CaptureErrorBody {
  code?: string
  error?: string
}

interface RequestError {
  code?: string
  message?: string
  response?: {
    data?: CaptureErrorBody
  }
}

interface YybCodeResult {
  code?: string
}

function extractYybCode(value: unknown): string {
  if (typeof value === 'string')
    return value.trim()
  if (value && typeof value === 'object') {
    const code = (value as { code?: unknown }).code
    return typeof code === 'string' ? code.trim() : ''
  }
  return ''
}

interface YybQrCreateResult {
  session_id?: string
  image_base64?: string
}

interface YybQrPollResult {
  status?: string
}

interface YybQrConfirmResult {
  account?: YybAccount
}

export interface UseAccountLoginOptions {
  show: Ref<boolean>
  editData: Ref<AccountEditData | undefined>
  onClose: () => void
  onSaved: () => void
}

function errorMessage(error: unknown, fallback: string): string {
  const requestError = error as RequestError
  return requestError.response?.data?.error || requestError.message || fallback
}

export function useAccountLogin(options: UseAccountLoginOptions) {
  const activeTab = ref<AccountLoginTab>('manual')
  const loading = ref(false)
  const error = ref('')
  const disposed = ref(false)

  const manualForm = reactive({
    name: '',
    code: '',
    platform: 'qq' as LoginPlatform,
  })

  const captureEnabled = ref(false)
  const captureLoading = ref(false)
  const captureChecking = ref(false)
  const captureCompleting = ref(false)
  const captureError = ref('')
  const captureCopiedField = ref<'host' | 'port' | ''>('')
  const captureAccountName = ref('')
  const capturePlatform = ref<LoginPlatform>('qq')
  const captureFlow = ref<CaptureFlowState | null>(null)
  const showCaptureHelp = ref(false)
  const captureHelpMode = ref<'first' | 'daily'>('first')
  const captureHelpDevice = ref<'ios' | 'android'>('ios')
  let captureActionController: AbortController | null = null
  let capturePollController: AbortController | null = null
  let captureGeneration = 0

  const captureHelpSteps = computed(() => captureHelpMode.value === 'first'
    ? [
        '点击开始抓取，获取本次代理地址和端口',
        '打开 CA 证书，并在手机系统中安装和信任',
        '连续添加时，先切换到目标 QQ 并彻底关闭上一个农场',
        '将手机 Wi-Fi 代理设置为页面显示的地址和端口',
        '彻底关闭后重新打开对应的 QQ 或微信农场',
        'Code 获取后账号会立即添加；QQ 好友 GID 将在后台继续同步',
        'QQ 农场保持打开，完整好友列表同步后会立即释放代理，最迟约 15 秒',
      ]
    : [
        '点击开始抓取，确认本次代理地址和端口',
        '连续添加时，先切换到目标 QQ 并彻底关闭上一个农场',
        '将手机 Wi-Fi 代理更新为本次显示的地址和端口',
        '重新打开对应农场，并保持页面打开',
        '账号添加后，QQ 农场继续保持打开，最迟约 15 秒完成后台同步',
        '后台同步结束后，将手机 Wi-Fi 代理改回关闭',
      ])

  const captureDeviceSteps = computed(() => captureHelpDevice.value === 'ios'
    ? [
        '在 Safari 中点击“打开证书”并允许下载描述文件',
        '进入“设置 → 通用 → VPN 与设备管理”安装描述文件',
        '进入“设置 → 通用 → 关于本机 → 证书信任设置”启用完全信任',
      ]
    : [
        '点击“打开证书”下载 CA 文件',
        '进入系统安全设置中的“安装证书”或“凭据存储”',
        '选择 CA 证书并确认安装；不同品牌的菜单名称可能不同',
      ])

  const captureCurrentStep = computed(() => {
    if (!captureFlow.value)
      return '开始新的抓取任务'
    if (!captureFlow.value.codeCaptured)
      return `设置 Wi-Fi 代理并打开${captureFlow.value.platform === 'qq' ? ' QQ' : '微信'}农场`
    return '已获取 Code，正在立即完成账号操作'
  })

  const captureNextStep = computed(() => {
    if (!captureFlow.value)
      return '开始后按本次显示的代理信息设置手机 Wi-Fi'
    if (!captureFlow.value.codeCaptured)
      return '重新打开小程序，并保持农场页面打开'
    if (captureFlow.value.platform === 'qq')
      return `即将自动${options.editData.value ? '更新' : '添加'}账号，好友 GID 将在后台同步`
    return `即将自动${options.editData.value ? '更新' : '添加'}账号`
  })

  const yybApiBase = ref('')
  const yybApiKey = ref('')
  const yybConfigLoaded = ref(false)
  const yybConfigSaving = ref(false)
  const yybAccounts = ref<YybAccount[]>([])
  const yybAccountsLoading = ref(false)
  const yybSelectedOpenid = ref('')
  const yybAccountName = ref('')
  const yybLoginLoading = ref(false)
  const yybError = ref('')
  const yybAutoReconnect = ref(false)
  const yybReconnectDelayMin = ref(5)
  const yybReconnectMaxAttempts = ref(3)
  const yybShowConfigEditor = ref(false)
  const yybConfigured = computed(() => !!yybApiBase.value.trim() && !!yybApiKey.value.trim())

  const yybQrImage = ref('')
  const yybQrSessionId = ref('')
  const yybQrStatus = ref<YybQrStatus>('idle')
  const yybQrLoading = computed(() => yybQrStatus.value === 'loading')
  const yybQrError = ref('')
  let yybQrPollTimer: ReturnType<typeof setTimeout> | null = null
  let yybQrRequestController: AbortController | null = null
  let yybQrPollGeneration = 0

  const yyb3rdApiBase = ref('')
  const yyb3rdApiToken = ref('')
  const yyb3rdOpenid = ref('')
  const yyb3rdAccountName = ref('')
  const yyb3rdLoading = ref(false)
  const yyb3rdError = ref('')
  const yyb3rdTokenMasked = ref('')
  const yyb3rdAutoReconnect = ref(true)
  const yyb3rdReconnectDelayMin = ref(5)
  const yyb3rdReconnectMaxAttempts = ref(3)

  async function addAccount(payload: Record<string, unknown>): Promise<boolean> {
    loading.value = true
    error.value = ''
    try {
      const response = await accountApi.createAccount(payload)
      if (!response.data.ok) {
        error.value = `保存失败: ${response.data.error || '未知错误'}`
        return false
      }
      options.onSaved()
      close()
      return true
    }
    catch (requestError) {
      error.value = `保存失败: ${errorMessage(requestError, '请求失败')}`
      return false
    }
    finally {
      loading.value = false
    }
  }

  async function completeCaptureAccount() {
    if (!captureFlow.value || captureCompleting.value)
      return
    const generation = captureGeneration
    const controller = new AbortController()
    captureActionController?.abort()
    captureActionController = controller
    captureCompleting.value = true
    captureError.value = ''
    try {
      const response = await captureApi.completeCaptureSession(captureFlow.value.id, {
        name: captureAccountName.value.trim(),
      }, { signal: controller.signal, timeout: 35000 })
      if (generation !== captureGeneration || disposed.value)
        return
      if (!response.data.ok)
        throw new Error(response.data.error || (options.editData.value ? '更新账号失败' : '添加账号失败'))
      localStorage.setItem(CAPTURE_SUCCESS_STORAGE_KEY, '1')
      stopCaptureCheck()
      captureFlow.value = null
      options.onSaved()
      close()
    }
    catch (requestError) {
      if ((requestError as RequestError).code === 'ERR_CANCELED' || generation !== captureGeneration)
        return
      const body = (requestError as RequestError).response?.data
      if (body?.code === 'DUPLICATE_CAPTURE_ACCOUNT') {
        stopCaptureCheck()
        captureFlow.value = null
      }
      captureError.value = errorMessage(requestError, options.editData.value ? '更新账号失败' : '添加账号失败')
    }
    finally {
      if (captureActionController === controller)
        captureActionController = null
      captureCompleting.value = false
    }
  }

  const captureInterval = useIntervalFn(async () => {
    if (disposed.value || activeTab.value !== 'capture' || !captureFlow.value || captureCompleting.value || captureChecking.value)
      return
    captureChecking.value = true
    const generation = captureGeneration
    const controller = new AbortController()
    capturePollController = controller
    try {
      const response = await captureApi.getCaptureSession<CaptureFlowState>(captureFlow.value.id, {
        signal: controller.signal,
        timeout: 20000,
      })
      if (disposed.value || generation !== captureGeneration || !response.data.ok || !response.data.data)
        return
      captureFlow.value = response.data.data
      captureError.value = response.data.data.proxy?.error || ''
      if (response.data.data.codeCaptured)
        await completeCaptureAccount()
    }
    catch (requestError) {
      if ((requestError as RequestError).code !== 'ERR_CANCELED')
        captureError.value = errorMessage(requestError, '查询抓取状态失败')
    }
    finally {
      captureChecking.value = false
      if (capturePollController === controller)
        capturePollController = null
    }
  }, 1500, { immediate: false })

  function stopCaptureCheck() {
    captureInterval.pause()
    capturePollController?.abort()
    capturePollController = null
  }

  async function loadCaptureConfig() {
    try {
      const response = await captureApi.getCaptureConfig<CaptureConfig>()
      captureEnabled.value = response.data.ok && response.data.data?.enabled === true
    }
    catch {
      captureEnabled.value = false
    }
  }

  async function cancelCaptureSession() {
    captureGeneration += 1
    stopCaptureCheck()
    captureActionController?.abort()
    captureActionController = null
    const flowId = captureFlow.value?.id
    captureFlow.value = null
    if (!flowId)
      return
    try {
      await captureApi.deleteCaptureSession(flowId)
    }
    catch {
      // The server also expires abandoned capture sessions.
    }
  }

  async function startCaptureSession() {
    captureLoading.value = true
    captureError.value = ''
    await cancelCaptureSession()
    const generation = captureGeneration
    const controller = new AbortController()
    captureActionController = controller
    try {
      const response = await captureApi.createCaptureSession<CaptureFlowState>({
        platform: capturePlatform.value,
        accountId: options.editData.value?.id || '',
      }, { signal: controller.signal, timeout: 35000 })
      if (generation !== captureGeneration || disposed.value)
        return
      if (!response.data.ok || !response.data.data)
        throw new Error(response.data.error || '启动抓取失败')
      captureFlow.value = response.data.data
      captureInterval.resume()
    }
    catch (requestError) {
      if ((requestError as RequestError).code !== 'ERR_CANCELED' && generation === captureGeneration)
        captureError.value = errorMessage(requestError, '启动抓取失败')
    }
    finally {
      if (captureActionController === controller)
        captureActionController = null
      captureLoading.value = false
    }
  }

  function openCaptureHelp() {
    captureHelpMode.value = localStorage.getItem(CAPTURE_SUCCESS_STORAGE_KEY) === '1' ? 'daily' : 'first'
    showCaptureHelp.value = true
  }

  const copiedReset = useTimeoutFn(() => {
    captureCopiedField.value = ''
  }, 1800, { immediate: false })

  async function copyCaptureValue(field: 'host' | 'port') {
    const host = captureFlow.value?.publicInfo.host || ''
    const port = captureFlow.value?.publicInfo.mitmPort || 0
    if (!host || !port)
      return
    const value = field === 'host' ? host : String(port)
    try {
      let copied = false
      if (navigator.clipboard?.writeText) {
        try {
          await navigator.clipboard.writeText(value)
          copied = true
        }
        catch {
          // Fall through to the compatibility path.
        }
      }
      if (!copied) {
        const textarea = document.createElement('textarea')
        textarea.value = value
        textarea.style.position = 'fixed'
        textarea.style.opacity = '0'
        document.body.appendChild(textarea)
        textarea.select()
        copied = document.execCommand('copy')
        textarea.remove()
      }
      if (!copied)
        throw new Error('copy failed')
      captureCopiedField.value = field
      copiedReset.stop()
      copiedReset.start()
    }
    catch {
      captureError.value = '复制失败，请手动填写代理地址和端口'
    }
  }

  async function submitManual() {
    error.value = ''
    if (!manualForm.code) {
      error.value = '请输入 Code'
      return
    }

    let code = manualForm.code.trim()
    const match = code.match(CODE_QUERY_RE)
    if (match?.[1]) {
      code = decodeURIComponent(match[1])
      manualForm.code = code
    }

    const editData = options.editData.value
    if (!editData) {
      await addAccount({
        name: manualForm.name,
        code,
        platform: manualForm.platform,
        loginType: 'manual',
      })
      return
    }

    const onlyNameChanged = manualForm.name !== editData.name
      && manualForm.code === (editData.code || '')
      && manualForm.platform === (editData.platform || 'qq')
    await addAccount(onlyNameChanged
      ? { id: editData.id, name: manualForm.name }
      : {
          id: editData.id,
          name: manualForm.name,
          code,
          platform: manualForm.platform,
          loginType: 'manual',
        })
  }

  async function loadYybConfig() {
    if (yybConfigLoaded.value)
      return
    try {
      const response = await systemApi.getWXConfig<WxConfig>()
      const config = response.data.data
      if (config) {
        yybApiBase.value = config.apiBase || ''
        yybApiKey.value = config.apiKey || ''
        yybAutoReconnect.value = config.autoReconnect === true
        yybReconnectDelayMin.value = config.reconnectDelayMin || 5
        yybReconnectMaxAttempts.value = config.reconnectMaxAttempts || 3
      }
    }
    catch (requestError) {
      console.error('加载应用宝配置失败', requestError)
    }
    finally {
      yybConfigLoaded.value = true
    }
  }

  async function saveYybConfig() {
    const apiBase = yybApiBase.value.trim()
    const apiKey = yybApiKey.value.trim()
    if (!!apiBase !== !!apiKey) {
      yybError.value = '独立 YYB 服务需要同时填写接口地址和 API Token'
      return
    }
    yybConfigSaving.value = true
    yybError.value = ''
    try {
      const response = await systemApi.getWXConfig<WxConfig>()
      const existingConfig = response.data.data || {}
      await systemApi.saveWXConfig({
        ...existingConfig,
        apiBase,
        apiKey,
        ...(existingConfig.appId ? {} : { appId: '' }),
        enabled: !!apiBase && !!apiKey,
        autoReconnect: yybAutoReconnect.value,
        reconnectDelayMin: Number(yybReconnectDelayMin.value) || 5,
        reconnectMaxAttempts: Number(yybReconnectMaxAttempts.value) || 3,
        confirmed: true,
      })
      yybConfigLoaded.value = true
      await fetchYybAccounts()
    }
    catch (requestError) {
      yybError.value = errorMessage(requestError, '保存配置失败')
    }
    finally {
      yybConfigSaving.value = false
    }
  }

  async function copyYybToken() {
    const token = yybApiKey.value.trim()
    if (!token)
      return
    try {
      await navigator.clipboard?.writeText(token)
    }
    catch {
      yybError.value = '复制 Token 失败'
    }
  }

  async function fetchYybAccounts() {
    yybAccountsLoading.value = true
    yybError.value = ''
    try {
      const response = await yybApi.getAccounts<YybAccount[]>({
        apiBase: yybApiBase.value.trim(),
        apiKey: yybApiKey.value.trim(),
      })
      if (!response.data.ok) {
        yybError.value = response.data.error || '获取账号列表失败'
        yybAccounts.value = []
        return
      }
      yybAccounts.value = response.data.data || []
      if (yybAccounts.value.length === 0)
        yybError.value = '应用宝接口没有可用账号'
    }
    catch (requestError) {
      yybError.value = errorMessage(requestError, '获取账号列表失败')
      yybAccounts.value = []
    }
    finally {
      yybAccountsLoading.value = false
    }
  }

  async function submitYybLogin() {
    if (!yybSelectedOpenid.value) {
      yybError.value = '请选择一个账号'
      return
    }
    yybLoginLoading.value = true
    yybError.value = ''
    try {
      const response = await yybApi.getCode<YybCodeResult>({
        apiBase: yybApiBase.value.trim(),
        apiKey: yybApiKey.value.trim(),
        openid: yybSelectedOpenid.value,
      })
      const code = extractYybCode(response.data.data)
      if (!response.data.ok || !code) {
        yybError.value = response.data.error || '获取登录 code 失败'
        return
      }
      const selected = yybAccounts.value.find(account => account.openid === yybSelectedOpenid.value)
      const name = yybAccountName.value.trim() || selected?.nickname || selected?.alias || `应用宝账号${Date.now()}`
      await addAccount({
        name,
        code,
        platform: 'wx',
        loginType: 'yyb',
        yybOpenid: yybSelectedOpenid.value,
      })
    }
    catch (requestError) {
      yybError.value = errorMessage(requestError, '应用宝登录失败')
    }
    finally {
      yybLoginLoading.value = false
    }
  }

  function stopYybQrPoll() {
    yybQrPollGeneration += 1
    if (yybQrPollTimer) {
      clearTimeout(yybQrPollTimer)
      yybQrPollTimer = null
    }
    yybQrRequestController?.abort()
    yybQrRequestController = null
  }

  function scheduleYybQrPoll(generation: number, delay: number) {
    if (generation !== yybQrPollGeneration || disposed.value)
      return
    if (yybQrPollTimer)
      clearTimeout(yybQrPollTimer)
    yybQrPollTimer = setTimeout(() => {
      yybQrPollTimer = null
      void pollYybQrStatus(generation)
    }, delay)
  }

  async function confirmYybQr(generation: number) {
    if (generation !== yybQrPollGeneration)
      return
    yybQrRequestController = new AbortController()
    try {
      const response = await yybApi.confirmQR<YybQrConfirmResult>({
        apiBase: yybApiBase.value.trim(),
        apiKey: yybApiKey.value.trim(),
        sessionId: yybQrSessionId.value,
      }, { signal: yybQrRequestController.signal })
      if (generation !== yybQrPollGeneration)
        return
      if (!response.data.ok) {
        yybQrError.value = response.data.error || '确认授权失败'
        yybQrStatus.value = 'error'
        return
      }
      const account = response.data.data?.account
      if (!account?.openid) {
        yybQrError.value = '扫码成功但未返回账号身份'
        yybQrStatus.value = 'error'
        return
      }
      const codeResponse = await yybApi.getCode<YybCodeResult>({ openid: account.openid })
      const code = extractYybCode(codeResponse.data.data)
      if (!codeResponse.data.ok || !code) {
        yybQrError.value = codeResponse.data.error || '扫码成功但获取登录 Code 失败'
        yybQrStatus.value = 'error'
        return
      }
      const created = await addAccount({
        name: account.nickname || account.alias || `应用宝账号${account.openid.slice(-4)}`,
        code,
        platform: 'wx',
        loginType: 'yyb',
        yybOpenid: account.openid,
      })
      if (!created) {
        yybQrError.value = error.value || '扫码成功但添加 FarmBot 账号失败'
        yybQrStatus.value = 'error'
        return
      }
      yybQrStatus.value = 'success'
      await fetchYybAccounts()
    }
    catch (requestError) {
      if ((requestError as RequestError).code !== 'ERR_CANCELED') {
        yybQrError.value = errorMessage(requestError, '确认授权失败')
        yybQrStatus.value = 'error'
      }
    }
    finally {
      yybQrRequestController = null
    }
  }

  async function pollYybQrStatus(generation: number) {
    if (!yybQrSessionId.value || generation !== yybQrPollGeneration || disposed.value)
      return
    if (['success', 'expired', 'error'].includes(yybQrStatus.value))
      return
    yybQrRequestController = new AbortController()
    try {
      const response = await yybApi.pollQR<YybQrPollResult>({
        apiBase: yybApiBase.value.trim(),
        apiKey: yybApiKey.value.trim(),
        sessionId: yybQrSessionId.value,
      }, { signal: yybQrRequestController.signal, timeout: 60000 })
      if (generation !== yybQrPollGeneration)
        return
      if (!response.data.ok) {
        yybQrError.value = response.data.error || '轮询失败'
        yybQrStatus.value = 'error'
        return
      }

      const status = response.data.data?.status || 'unknown'
      if (status === 'pending') {
        scheduleYybQrPoll(generation, 1000)
      }
      else if (status === 'scanned') {
        yybQrStatus.value = 'scanned'
        scheduleYybQrPoll(generation, 0)
      }
      else if (status === 'authorized') {
        yybQrStatus.value = 'authorizing'
        await confirmYybQr(generation)
      }
      else if (status === 'confirmed') {
        yybQrStatus.value = 'success'
        await fetchYybAccounts()
      }
      else if (status === 'expired' || status === 'cancelled') {
        yybQrStatus.value = 'expired'
        yybQrError.value = status === 'expired' ? '二维码已过期，请重新扫码' : '已取消'
      }
      else {
        scheduleYybQrPoll(generation, 2000)
      }
    }
    catch (requestError) {
      if ((requestError as RequestError).code !== 'ERR_CANCELED' && generation === yybQrPollGeneration)
        scheduleYybQrPoll(generation, 2000)
    }
    finally {
      yybQrRequestController = null
    }
  }

  async function startYybQrLogin() {
    stopYybQrPoll()
    yybQrError.value = ''
    yybQrImage.value = ''
    yybQrSessionId.value = ''
    yybQrStatus.value = 'loading'
    const generation = yybQrPollGeneration
    yybQrRequestController = new AbortController()
    try {
      const response = await yybApi.createQR<YybQrCreateResult>({
        apiBase: yybApiBase.value.trim(),
        apiKey: yybApiKey.value.trim(),
      }, { signal: yybQrRequestController.signal })
      if (generation !== yybQrPollGeneration)
        return
      const sessionId = response.data.data?.session_id
      if (!response.data.ok || !sessionId) {
        yybQrError.value = response.data.error || '创建扫码会话失败'
        yybQrStatus.value = 'error'
        return
      }
      yybQrSessionId.value = sessionId
      yybQrImage.value = response.data.data?.image_base64 || ''
      yybQrStatus.value = 'pending'
      scheduleYybQrPoll(generation, 0)
    }
    catch (requestError) {
      if ((requestError as RequestError).code !== 'ERR_CANCELED' && generation === yybQrPollGeneration) {
        yybQrError.value = errorMessage(requestError, '创建扫码会话失败')
        yybQrStatus.value = 'error'
      }
    }
    finally {
      if (generation === yybQrPollGeneration)
        yybQrRequestController = null
    }
  }

  function resetYybQr() {
    stopYybQrPoll()
    yybQrImage.value = ''
    yybQrSessionId.value = ''
    yybQrStatus.value = 'idle'
    yybQrError.value = ''
  }

  async function submitYyb3rdLogin() {
    yyb3rdError.value = ''
    const editData = options.editData.value
    const isEdit = !!editData
    const baseOk = !!yyb3rdApiBase.value.trim()
    const openidOk = !!yyb3rdOpenid.value.trim()
    const tokenOk = !!yyb3rdApiToken.value.trim()
    if (!baseOk || !openidOk || (!isEdit && !tokenOk)) {
      yyb3rdError.value = isEdit
        ? '请填写接口地址和 OPENID'
        : '请填写接口地址、APITOKEN 和 OPENID'
      return
    }

    yyb3rdLoading.value = true
    try {
      let code = editData?.code
      if (!isEdit || tokenOk) {
        const response = await yybApi.getThirdPartyCode<YybCodeResult>({
          apiBase: yyb3rdApiBase.value.trim(),
          apiToken: yyb3rdApiToken.value.trim(),
          openid: yyb3rdOpenid.value.trim(),
          name: yyb3rdAccountName.value.trim(),
        })
        code = response.data.data?.code
        if (!response.data.ok || !code) {
          yyb3rdError.value = response.data.error || '获取登录 code 失败'
          return
        }
      }
      const openid = yyb3rdOpenid.value.trim()
      const name = yyb3rdAccountName.value.trim() || `第三方应用宝${openid.slice(-4)}`
      const thirdparty = {
        apiBase: yyb3rdApiBase.value.trim(),
        apiToken: yyb3rdApiToken.value.trim(),
        openid,
        autoReconnect: yyb3rdAutoReconnect.value === true,
        reconnectDelayMin: Math.max(1, Number(yyb3rdReconnectDelayMin.value) || 5),
        reconnectMaxAttempts: Math.max(1, Number(yyb3rdReconnectMaxAttempts.value) || 3),
      }
      await addAccount({
        ...(editData ? { id: editData.id } : {}),
        name,
        code,
        platform: 'wx',
        loginType: 'yyb',
        provider: 'thirdparty',
        yybOpenid: openid,
        thirdparty,
      })
    }
    catch (requestError) {
      yyb3rdError.value = errorMessage(requestError, '第三方应用宝登录失败')
    }
    finally {
      yyb3rdLoading.value = false
    }
  }

  function initialize() {
    error.value = ''
    captureError.value = ''
    captureCopiedField.value = ''
    captureAccountName.value = options.editData.value?.name || ''
    capturePlatform.value = options.editData.value?.platform === 'wx' ? 'wx' : 'qq'
    captureHelpMode.value = localStorage.getItem(CAPTURE_SUCCESS_STORAGE_KEY) === '1' ? 'daily' : 'first'
    yybConfigLoaded.value = false
    void loadCaptureConfig()

    const editData = options.editData.value
    if (editData?.provider === 'thirdparty') {
      activeTab.value = 'yyb3rd'
      yyb3rdApiBase.value = editData.thirdparty?.apiBase || ''
      yyb3rdOpenid.value = editData.thirdparty?.openid || ''
      yyb3rdApiToken.value = ''
      yyb3rdTokenMasked.value = editData.thirdparty?.apiToken || ''
      yyb3rdAccountName.value = editData.name || ''
      yyb3rdAutoReconnect.value = editData.thirdparty?.autoReconnect !== false
      yyb3rdReconnectDelayMin.value = editData.thirdparty?.reconnectDelayMin || 5
      yyb3rdReconnectMaxAttempts.value = editData.thirdparty?.reconnectMaxAttempts || 3
      return
    }

    activeTab.value = 'manual'
    manualForm.name = editData?.name || ''
    manualForm.code = editData?.code || ''
    manualForm.platform = editData?.platform || 'qq'
    yyb3rdApiBase.value = ''
    yyb3rdApiToken.value = ''
    yyb3rdOpenid.value = ''
    yyb3rdAccountName.value = ''
    yyb3rdTokenMasked.value = ''
    yyb3rdAutoReconnect.value = true
    yyb3rdReconnectDelayMin.value = 5
    yyb3rdReconnectMaxAttempts.value = 3
  }

  function close() {
    stopCaptureCheck()
    void cancelCaptureSession()
    resetYybQr()
    showCaptureHelp.value = false
    options.onClose()
  }

  watch(options.show, (show) => {
    if (show) {
      initialize()
    }
    else {
      void cancelCaptureSession()
      resetYybQr()
      showCaptureHelp.value = false
    }
  }, { immediate: true })

  watch(activeTab, (tab) => {
    if (tab !== 'capture') {
      void cancelCaptureSession()
      showCaptureHelp.value = false
    }
    if (tab === 'yyb' || tab === 'yybqr')
      void loadYybConfig()
    if (tab !== 'yybqr')
      resetYybQr()
  })

  onScopeDispose(() => {
    disposed.value = true
    captureGeneration += 1
    captureActionController?.abort()
    captureActionController = null
    stopCaptureCheck()
    copiedReset.stop()
    stopYybQrPoll()
    const flowId = captureFlow.value?.id
    captureFlow.value = null
    if (flowId)
      void captureApi.deleteCaptureSession(flowId).catch(() => undefined)
  })

  return {
    activeTab,
    loading,
    error,
    manualForm,
    captureEnabled,
    captureLoading,
    captureChecking,
    captureCompleting,
    captureError,
    captureCopiedField,
    captureAccountName,
    capturePlatform,
    captureFlow,
    showCaptureHelp,
    captureHelpMode,
    captureHelpDevice,
    captureHelpSteps,
    captureDeviceSteps,
    captureCurrentStep,
    captureNextStep,
    yybApiBase,
    yybApiKey,
    yybConfigSaving,
    yybAccounts,
    yybAccountsLoading,
    yybSelectedOpenid,
    yybAccountName,
    yybLoginLoading,
    yybError,
    yybAutoReconnect,
    yybReconnectDelayMin,
    yybReconnectMaxAttempts,
    yybShowConfigEditor,
    yybConfigured,
    yybQrImage,
    yybQrStatus,
    yybQrLoading,
    yybQrError,
    yyb3rdApiBase,
    yyb3rdApiToken,
    yyb3rdOpenid,
    yyb3rdAccountName,
    yyb3rdLoading,
    yyb3rdError,
    yyb3rdTokenMasked,
    yyb3rdAutoReconnect,
    yyb3rdReconnectDelayMin,
    yyb3rdReconnectMaxAttempts,
    close,
    submitManual,
    startCaptureSession,
    cancelCaptureSession,
    completeCaptureAccount,
    openCaptureHelp,
    copyCaptureValue,
    loadYybConfig,
    saveYybConfig,
    copyYybToken,
    fetchYybAccounts,
    submitYybLogin,
    startYybQrLogin,
    resetYybQr,
    submitYyb3rdLogin,
  }
}
