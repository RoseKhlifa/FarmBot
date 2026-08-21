import type { AxiosRequestConfig, AxiosResponse } from 'axios'

export type Identifier = string | number
// Payload helpers accept any structured request object. Individual API modules
// still expose narrower input interfaces where the endpoint contract is known.
export type ApiBody = object

export interface ApiEnvelope<T = any> {
  ok: boolean
  // Unparameterized legacy callers get an intentionally permissive response;
  // typed callers retain their concrete response model.
  data?: T
  error?: string
  message?: string
  [key: string]: any
}

export type ApiResponse<T = any> = AxiosResponse<ApiEnvelope<T>>

// Authentication and account headers belong exclusively to the shared client.
export type ApiRequestConfig<D = unknown> = Omit<
  AxiosRequestConfig<D>,
  'baseURL' | 'data' | 'headers' | 'method' | 'url'
>

export function pathSegment(value: Identifier): string {
  return encodeURIComponent(String(value))
}
