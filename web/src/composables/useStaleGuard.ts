import { useAccountStore } from '@/stores/account'

function normalizeAccountId(value: unknown): string {
  if (value && typeof value === 'object' && 'value' in value)
    return String((value as { value?: unknown }).value ?? '')
  return String(value ?? '')
}

export function isCurrentAccount(accountId: string | number | null | undefined): boolean {
  const accountStore = useAccountStore()
  return normalizeAccountId(accountStore.currentAccountId) === normalizeAccountId(accountId)
}

export function useStaleGuard() {
  return { isCurrentAccount }
}
