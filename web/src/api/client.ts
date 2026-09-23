import type {
  CacheList,
  CacheStatus,
  ConfigView,
  ExtManagersResponse,
  ExtPackagesResponse,
  OutdatedResponse,
  Overview,
  PackageDetail,
  PackagesResponse,
} from './types'

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export const TOKEN_HEADER = 'X-EGET-Token'

// The console is same-origin: the auth cookie planted by the first ?token=
// visit travels automatically, so requests need no explicit credentials.
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: 'same-origin',
    ...init,
    headers: { Accept: 'application/json', ...(init?.headers ?? {}) },
  })
  if (!response.ok) {
    throw new ApiError(response.status, await errorMessage(response))
  }
  if (response.status === 204) {
    return undefined as T
  }
  return (await response.json()) as T
}

async function errorMessage(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as { error?: { message?: string; code?: string } }
    if (body.error?.message) {
      return body.error.message
    }
    if (body.error?.code) {
      return body.error.code
    }
  } catch {
    // Fall through to the status line for non-JSON bodies.
  }
  return `${response.status} ${response.statusText}`
}

function query(params: Record<string, string | number | boolean | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === '' || value === false) {
      continue
    }
    search.set(key, String(value))
  }
  const text = search.toString()
  return text ? `?${text}` : ''
}

export const api = {
  overview: () => request<Overview>('/api/overview'),
  packages: (params: { scope?: string; manager?: string; q?: string; installed?: boolean } = {}) =>
    request<PackagesResponse>(`/api/packages${query(params)}`),
  packageDetail: (name: string) =>
    request<PackageDetail>(`/api/packages/${encodeURIComponent(name)}`),
  outdated: () => request<OutdatedResponse>('/api/outdated'),
  ext: () => request<ExtManagersResponse>('/api/ext'),
  extPackages: (manager: string) =>
    request<ExtPackagesResponse>(`/api/ext/${encodeURIComponent(manager)}/packages`),
  cache: (root?: string) => request<CacheList>(`/api/cache${query({ root })}`),
  cacheStatus: () => request<CacheStatus>('/api/cache/status'),
  config: () => request<ConfigView>('/api/config'),
}

export function formatBytes(size: number): string {
  if (size < 1024) {
    return `${size} B`
  }
  const units = ['KB', 'MB', 'GB', 'TB', 'PB']
  let value = size / 1024
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index++
  }
  return `${value.toFixed(1)} ${units[index]}`
}

export function formatTime(value?: string): string {
  if (!value) {
    return '-'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}
