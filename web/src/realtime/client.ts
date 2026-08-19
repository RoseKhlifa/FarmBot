export const REALTIME_EVENTS = {
  status: 'status:update',
  log: 'log:new',
  accountLog: 'account-log:new',
  logsSnapshot: 'logs:snapshot',
  accountLogsSnapshot: 'account-logs:snapshot',
  subscribed: 'subscribed',
  ready: 'ready',
  pong: 'pong',
} as const

export type RealtimeEventName = typeof REALTIME_EVENTS[keyof typeof REALTIME_EVENTS] | string

export type RealtimeStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

export interface RealtimeFrame<T = unknown> {
  event: string
  data: T
}

export interface RealtimeState {
  status: RealtimeStatus
  connected: boolean
  accountId: string
  retryCount: number
  error: string
}

export interface RealtimeSocket {
  readonly readyState: number
  onopen: (() => void) | null
  onmessage: ((event: { data: unknown }) => void) | null
  onerror: ((event: unknown) => void) | null
  onclose: ((event: { code?: number, reason?: string }) => void) | null
  send: (data: string) => void
  close: (code?: number, reason?: string) => void
}
export interface RealtimeClientOptions {
  url?: string | (() => string)
  getToken?: () => string
  reconnect?: boolean
  maxRetries?: number
  reconnectDelay?: number
  maxReconnectDelay?: number
  socketFactory?: (url: string) => RealtimeSocket
}

type RealtimeListener = (data: unknown, frame: RealtimeFrame) => void
type StatusListener = (state: RealtimeState) => void

const SOCKET_CONNECTING = 0
const SOCKET_OPEN = 1

function defaultSocketFactory(url: string): RealtimeSocket {
  return new WebSocket(url) as unknown as RealtimeSocket
}

function defaultRealtimeURL(): string {
  if (typeof window === 'undefined')
    return 'ws://localhost/ws'

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/ws`
}

function normalizeAccountId(accountId: string | number | null | undefined): string {
  return String(accountId ?? '').trim()
}

function readToken(getToken: (() => string) | undefined, override: string): string {
  if (override.trim())
    return override.trim()
  try {
    return String(getToken?.() || '').trim()
  }
  catch {
    return ''
  }
}

export class RealtimeClient {
  private readonly options: Required<Pick<RealtimeClientOptions, 'reconnect' | 'maxRetries' | 'reconnectDelay' | 'maxReconnectDelay'>> & RealtimeClientOptions
  private readonly listeners = new Map<string, Set<RealtimeListener>>()
  private readonly statusListeners = new Set<StatusListener>()
  private socket: RealtimeSocket | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private shouldReconnect = false
  private retryCount = 0
  private accountId = ''
  private tokenOverride = ''
  private currentStatus: RealtimeStatus = 'idle'
  private currentError = ''

  constructor(options: RealtimeClientOptions = {}) {
    this.options = {
      ...options,
      reconnect: options.reconnect ?? true,
      maxRetries: options.maxRetries ?? Infinity,
      reconnectDelay: Math.max(100, options.reconnectDelay ?? 1000),
      maxReconnectDelay: Math.max(100, options.maxReconnectDelay ?? 10000),
    }
  }

  get status(): RealtimeStatus {
    return this.currentStatus
  }

  get connected(): boolean {
    return this.currentStatus === 'connected'
  }

  get currentAccountId(): string {
    return this.accountId
  }

  get lastError(): string {
    return this.currentError
  }

  get state(): RealtimeState {
    return {
      status: this.currentStatus,
      connected: this.connected,
      accountId: this.accountId,
      retryCount: this.retryCount,
      error: this.currentError,
    }
  }

  on(event: RealtimeEventName, listener: RealtimeListener): () => void {
    const key = String(event)
    let listeners = this.listeners.get(key)
    if (!listeners) {
      listeners = new Set<RealtimeListener>()
      this.listeners.set(key, listeners)
    }
    listeners.add(listener)
    return () => {
      listeners?.delete(listener)
      if (listeners?.size === 0)
        this.listeners.delete(key)
    }
  }

  onStatus(listener: StatusListener): () => void {
    this.statusListeners.add(listener)
    listener(this.state)
    return () => this.statusListeners.delete(listener)
  }

  connect(accountId: string | number | null | undefined, token = ''): void {
    const nextAccountId = normalizeAccountId(accountId)
    this.accountId = nextAccountId
    this.tokenOverride = token.trim()

    if (!nextAccountId || !readToken(this.options.getToken, this.tokenOverride)) {
      this.disconnect()
      return
    }

    this.shouldReconnect = true
    if (this.socket?.readyState === SOCKET_OPEN) {
      this.sendSubscription()
      return
    }
    if (this.socket?.readyState === SOCKET_CONNECTING)
      return

    this.clearReconnectTimer()
    this.open()
  }

  subscribe(accountId: string | number | null | undefined): void {
    const nextAccountId = normalizeAccountId(accountId)
    this.accountId = nextAccountId
    if (!nextAccountId) {
      this.disconnect()
      return
    }
    if (!this.shouldReconnect || !this.socket) {
      this.connect(nextAccountId)
      return
    }
    if (this.socket.readyState === SOCKET_OPEN)
      this.sendSubscription()
  }

  disconnect(): void {
    this.shouldReconnect = false
    this.clearReconnectTimer()
    this.retryCount = 0
    const socket = this.socket
    this.socket = null
    this.tokenOverride = ''
    if (socket && (socket.readyState === SOCKET_CONNECTING || socket.readyState === SOCKET_OPEN))
      socket.close(1000, 'client disconnect')
    this.setStatus('disconnected')
  }

  close(): void {
    this.disconnect()
  }

  send(frame: RealtimeFrame): boolean {
    if (!this.socket || this.socket.readyState !== SOCKET_OPEN)
      return false
    try {
      this.socket.send(JSON.stringify(frame))
      return true
    }
    catch (error) {
      this.setStatus(this.currentStatus, error instanceof Error ? error.message : String(error))
      return false
    }
  }

  private open(): void {
    const token = readToken(this.options.getToken, this.tokenOverride)
    if (!token || !this.accountId) {
      this.setStatus('disconnected')
      return
    }

    const url = this.buildURL(token, this.accountId)
    const factory = this.options.socketFactory || defaultSocketFactory
    let socket: RealtimeSocket
    try {
      socket = factory(url)
    }
    catch (error) {
      this.handleConnectionFailure(error)
      return
    }

    this.socket = socket
    this.setStatus(this.retryCount > 0 ? 'reconnecting' : 'connecting')
    socket.onopen = () => {
      if (this.socket !== socket)
        return
      this.retryCount = 0
      this.setStatus('connected')
      this.sendSubscription()
    }
    socket.onmessage = (event) => {
      if (this.socket === socket)
        this.handleMessage(event.data)
    }
    socket.onerror = (event) => {
      if (this.socket !== socket)
        return
      const message = event instanceof Error ? event.message : '实时连接错误'
      this.setStatus(this.currentStatus, message)
    }
    socket.onclose = (event) => {
      if (this.socket !== socket)
        return
      this.socket = null
      if (this.shouldReconnect) {
        this.scheduleReconnect(event.reason || `连接关闭 (${event.code ?? 1000})`)
      }
      else {
        this.setStatus('disconnected')
      }
    }
  }

  private sendSubscription(): void {
    this.send({
      event: 'subscribe',
      data: { accountId: this.accountId },
    })
  }

  private handleMessage(raw: unknown): void {
    if (typeof raw === 'string') {
      this.parseMessage(raw)
      return
    }
    if (raw instanceof ArrayBuffer) {
      this.parseMessage(new TextDecoder().decode(raw))
      return
    }
    if (typeof Blob !== 'undefined' && raw instanceof Blob) {
      raw.text().then(text => this.parseMessage(text)).catch(() => {})
    }
  }

  private parseMessage(raw: string): void {
    let frame: RealtimeFrame
    try {
      frame = JSON.parse(raw) as RealtimeFrame
    }
    catch {
      return
    }
    if (!frame || typeof frame.event !== 'string' || !frame.event.trim())
      return

    const listeners = this.listeners.get(frame.event)
    if (!listeners)
      return
    for (const listener of [...listeners]) {
      try {
        listener(frame.data, frame)
      }
      catch (error) {
        console.error(`[realtime] 事件 ${frame.event} 处理失败`, error)
      }
    }
  }

  private buildURL(token: string, accountId: string): string {
    const configured = typeof this.options.url === 'function' ? this.options.url() : this.options.url
    const base = configured || defaultRealtimeURL()
    const url = new URL(base, typeof window === 'undefined' ? 'http://localhost' : window.location.origin)
    url.protocol = url.protocol === 'https:' ? 'wss:' : url.protocol === 'http:' ? 'ws:' : url.protocol
    url.searchParams.set('token', token)
    url.searchParams.set('accountId', accountId)
    return url.toString()
  }

  private handleConnectionFailure(error: unknown): void {
    const message = error instanceof Error ? error.message : String(error || '实时连接失败')
    this.socket = null
    if (this.shouldReconnect)
      this.scheduleReconnect(message)
    else
      this.setStatus('disconnected', message)
  }

  private scheduleReconnect(error = ''): void {
    if (!this.shouldReconnect || !this.options.reconnect)
      return
    if (Number.isFinite(this.options.maxRetries) && this.retryCount >= this.options.maxRetries) {
      this.shouldReconnect = false
      this.setStatus('disconnected', error || '实时重连次数已耗尽')
      return
    }
    this.retryCount += 1
    const delay = Math.min(
      this.options.maxReconnectDelay,
      this.options.reconnectDelay * 2 ** Math.min(this.retryCount - 1, 10),
    )
    this.setStatus('reconnecting', error)
    this.clearReconnectTimer()
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      if (this.shouldReconnect)
        this.open()
    }, delay)
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  private setStatus(status: RealtimeStatus, error = ''): void {
    this.currentStatus = status
    this.currentError = error
    const snapshot = this.state
    for (const listener of [...this.statusListeners]) {
      try {
        listener(snapshot)
      }
      catch (listenerError) {
        console.error('[realtime] 状态监听器处理失败', listenerError)
      }
    }
  }
}
