/* eslint-disable style/max-statements-per-line */
import type { Ref } from 'vue'
import { onScopeDispose, watch } from 'vue'

export interface AccountScopeOptions {
  immediate?: boolean
}

/** Runs one account transition handler and owns its watcher cleanup. */
export function useAccountScope<T extends string | number | null | undefined>(accountId: Ref<T>, onChange: (next: T, previous: T | undefined) => void | Promise<void>, options: AccountScopeOptions = {}) {
  const stop = watch(accountId, (next, previous) => { void onChange(next, previous) }, { immediate: options.immediate ?? true })
  onScopeDispose(stop)
  return { stop }
}
