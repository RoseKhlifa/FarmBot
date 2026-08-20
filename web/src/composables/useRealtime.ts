import type { Ref } from 'vue'
import type {
  RealtimeClientOptions,
  RealtimeEventName,
  RealtimeFrame,
  RealtimeState,
  RealtimeStatus,
} from '@/realtime/client'
import { getCurrentScope, onScopeDispose, readonly, ref } from 'vue'
import { RealtimeClient } from '@/realtime/client'

export interface UseRealtimeOptions extends RealtimeClientOptions {
  client?: RealtimeClient
  disconnectOnScopeDispose?: boolean
}

export interface UseRealtime {
  client: RealtimeClient
  connected: Readonly<Ref<boolean>>
  status: Readonly<Ref<RealtimeStatus>>
  accountId: Readonly<Ref<string>>
  lastError: Readonly<Ref<string>>
  connect: (accountId: string | number | null | undefined, token?: string) => void
  subscribe: (accountId: string | number | null | undefined) => void
  disconnect: () => void
  on: (event: RealtimeEventName, listener: (data: unknown, frame: RealtimeFrame) => void) => () => void
  onStatus: (listener: (state: RealtimeState) => void) => () => void
}

let sharedClient: RealtimeClient | null = null

function getSharedClient(options: UseRealtimeOptions): RealtimeClient {
  if (!sharedClient)
    sharedClient = new RealtimeClient(options)
  return sharedClient
}

export function useRealtime(options: UseRealtimeOptions = {}): UseRealtime {
  const client = options.client || getSharedClient(options)
  const connected = ref(client.connected)
  const status = ref<RealtimeStatus>(client.status)
  const accountId = ref(client.currentAccountId)
  const lastError = ref(client.lastError)
  const cleanups = new Set<() => void>()

  const stopStatus = client.onStatus((state) => {
    connected.value = state.connected
    status.value = state.status
    accountId.value = state.accountId
    lastError.value = state.error
  })
  cleanups.add(stopStatus)

  function on(event: RealtimeEventName, listener: (data: unknown, frame: RealtimeFrame) => void): () => void {
    const stop = client.on(event, listener)
    cleanups.add(stop)
    return () => {
      stop()
      cleanups.delete(stop)
    }
  }

  function onStatus(listener: (state: RealtimeState) => void): () => void {
    const stop = client.onStatus(listener)
    cleanups.add(stop)
    return () => {
      stop()
      cleanups.delete(stop)
    }
  }

  if (getCurrentScope()) {
    onScopeDispose(() => {
      for (const cleanup of cleanups)
        cleanup()
      cleanups.clear()
      if (options.disconnectOnScopeDispose)
        client.disconnect()
    })
  }

  return {
    client,
    connected: readonly(connected),
    status: readonly(status),
    accountId: readonly(accountId),
    lastError: readonly(lastError),
    connect: (nextAccountId, token) => client.connect(nextAccountId, token),
    subscribe: nextAccountId => client.subscribe(nextAccountId),
    disconnect: () => client.disconnect(),
    on,
    onStatus,
  }
}
