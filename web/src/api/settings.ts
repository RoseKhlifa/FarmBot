import type { ApiBody, ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function getDefaultPlan<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/settings/default-plan', config)
}

export function saveDefaultPlan<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.put<ApiEnvelope<T>>('/api/settings/default-plan', payload, config)
}

export function importDefaultPlan<T = any>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/settings/default-plan/import', payload, config)
}

export function applyDefaultPlan<T = any>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/settings/default-plan/apply', payload, config)
}

export function resetDefaultPlan<T = any>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/settings/default-plan/reset', {}, config)
}

export function getSettings<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/settings', config)
}

export function saveSettings<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/settings/save', payload, config)
}

export function saveTheme<T = any>(payload: { theme: string }, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/settings/theme', payload, config)
}

export function saveAutoCodeRefresh<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/settings/auto-code-refresh', payload, config)
}

export function runAutoCodeRefresh<T = any>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/settings/auto-code-refresh/run', {}, config)
}

export function saveOfflineReminder<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/settings/offline-reminder', payload, config)
}

export function testOfflineReminder<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/settings/offline-reminder/test', payload, config)
}

export function getDefaultSettings<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/settings/default', config)
}

export function saveAutomation<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/automation', payload, config)
}
