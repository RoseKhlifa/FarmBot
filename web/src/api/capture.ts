import type { ApiBody, ApiEnvelope, ApiRequestConfig, Identifier } from './types'
import client from './client'
import { pathSegment } from './types'

export function getAdminCaptureConfig<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/admin/capture-config', config)
}

export function testAdminCaptureConfig<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/capture-config/test', payload, config)
}

export function saveAdminCaptureConfig<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/admin/capture-config', payload, config)
}

export function getCaptureConfig<T = unknown>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/capture/config', config)
}

export function createCaptureSession<T = unknown>(payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/capture/sessions', payload, config)
}

export function getCaptureSession<T = unknown>(flowId: Identifier, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>(`/api/capture/sessions/${pathSegment(flowId)}`, config)
}

export function deleteCaptureSession<T = unknown>(flowId: Identifier, config?: ApiRequestConfig) {
  return client.delete<ApiEnvelope<T>>(`/api/capture/sessions/${pathSegment(flowId)}`, config)
}

export function completeCaptureSession<T = unknown>(flowId: Identifier, payload: ApiBody, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>(`/api/capture/sessions/${pathSegment(flowId)}/complete`, payload, config)
}

export function getPublicCaptureCertificate<T = unknown>(flowId: Identifier, token: string, config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>(`/api/public/capture-certificate/${pathSegment(flowId)}/${pathSegment(token)}`, config)
}
