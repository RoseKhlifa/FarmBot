import type { AxiosRequestConfig, AxiosResponse } from 'axios'

export type Identifier = string | number
export type ApiBody = Record<string, unknown>

export interface ApiEnvelope<T = unknown> {
  ok: boolean
  data?: T
  error?: string
  message?: string
  [key: string]: unknown
}

export type ApiResponse<T = unknown> = AxiosResponse<ApiEnvelope<T>>

// Authentication and account headers belong exclusively to the shared client.
export type ApiRequestConfig<D = unknown> = Omit<
  AxiosRequestConfig<D>,
  'baseURL' | 'data' | 'headers' | 'method' | 'url'
>

export function pathSegment(value: Identifier): string {
  return encodeURIComponent(String(value))
}
