import type { ApiBody, ApiEnvelope, ApiRequestConfig, Identifier } from './types'
import client from './client'
import { pathSegment } from './types'

export function getAdminCaptureConfig<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/capture-config', config)
}

export function testAdminCaptureConfig<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/capture-config/test', payload, config)
}

export function saveAdminCaptureConfig<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/capture-config', payload, config)
}

export function getCaptureConfig<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/capture/config', config)
}

export function createCaptureSession<T = any>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/capture/sessions', payload, config)
}

export function getCaptureSession<T = any>(flowId: Identifier, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>(`/api/capture/sessions/${pathSegment(flowId)}`, config)
}

export function deleteCaptureSession<T = any>(flowId: Identifier, config?: ApiRequestConfig) {
  return client.delete<ApiEnvelope<T>>(`/api/capture/sessions/${pathSegment(flowId)}`, config)
}

export function completeCaptureSession<T = any>(flowId: Identifier, payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/capture/sessions/${pathSegment(flowId)}/complete`, payload, config)
}

export function getPublicCaptureCertificate<T = any>(flowId: Identifier, token: string, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>(`/api/public/capture-certificate/${pathSegment(flowId)}/${pathSegment(token)}`, config)
}
