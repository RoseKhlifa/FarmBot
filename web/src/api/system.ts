import type { ApiBody, ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export function getLoginLinks<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/login-links', config)
}

export function saveLoginLinks<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/login-links', payload, config)
}

export function resetLoginLinks<T = any>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/login-links/reset', {}, config)
}

export function getSystemConfig<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/system-config', config)
}

export function saveSystemConfig<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/system-config', payload, config)
}

export function resetSystemConfig<T = any>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/system-config/reset', {}, config)
}

export function getWXConfig<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/wx-config', config)
}

export function saveWXConfig<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/wx-config', payload, config)
}

export function saveAnnouncement<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/announcement', payload, config)
}

export function saveLoginLogo<T = any>(payload: ApiBody | FormData, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/login-logo', payload, config)
}

export function saveSuperAdminAnnouncement<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin/announcement', payload, config)
}

export function getSuperAdminAnnouncement<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/super-admin-announcement', config)
}

export function saveSuperAdminAnnouncementLegacy<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin-announcement', payload, config)
}

export function verifySuperAdminAnnouncement<T = any>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin-announcement/verify', payload, config)
}

export function getSuperAdminAntiResaleConfig<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/super-admin/anti-resale-config', config)
}

export function saveSuperAdminAntiResaleConfig<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin/anti-resale-config', payload, config)
}

export function checkAccountLimit<T = any>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin/check-account-limit', payload, config)
}

export function clearData<T = any>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/super-admin/clear-data', payload, config)
}

export function getSystemInfo<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/system/info', config)
}

export function getPublicLoginLinks<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/public/login-links', config)
}

export function getDebugItemConfig<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/debug/item-config', config)
}

export function getPublicValue<T = any>(name: 'user-count' | 'anti-resale-config' | 'changelog' | 'scheduler', config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>(`/api/${name}`, config)
}

export function getAnnouncement<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/announcement', config)
}

export function readAnnouncement<T = any>(payload: ApiBody = {}, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/announcement/read', payload, config)
}

export function clearAnnouncement<T = any>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/announcement/clear', {}, config)
}

export function restartSystem<T = any>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/system/restart', {}, config)
}

export function saveSystemRuntimeConfig<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/system/config', payload, config)
}

export function getGameVersion<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/game-version', config)
}

export function validateAuth<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/auth/validate', config)
}
