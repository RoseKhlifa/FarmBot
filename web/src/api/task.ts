import type { ApiEnvelope, ApiRequestConfig } from './types'
import client from './client'

export interface ClaimTaskInput {
  id: number
  shared?: boolean
}

export function getTasks<T = any>(config?: ApiRequestConfig) {
  return client.get<ApiEnvelope<T>>('/api/tasks', config)
}

export function claimTask<T = any>(payload: ClaimTaskInput, config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/tasks/claim', payload, config)
}

export function claimAllTasks<T = any>(config?: ApiRequestConfig) {
  return client.post<ApiEnvelope<T>>('/api/tasks/claim-all', {}, config)
}
