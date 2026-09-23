// Types mirror the JSON contract of internal/app/web/model.go.

export interface Overview {
  version: string
  configPath: string
  configExists: boolean
  packages: number
  installed: number
  ext: ExtBrief[]
  cache?: CacheBrief
  tasks: TaskCounts
}

export interface CacheBrief {
  dir: string
  files: number
  size: number
}

export interface TaskCounts {
  running: number
  queued: number
}

export interface ExtBrief {
  manager: string
  available: boolean
  bin?: string
}

export interface ExtManager extends ExtBrief {
  packages: number
}

export interface ExtManagersResponse {
  managers: ExtManager[]
  failures?: Failure[]
}

export interface ExtPackage {
  manager: string
  name: string
  version?: string
  latest?: string
}

export interface ExtPackagesResponse {
  manager: string
  packages: ExtPackage[]
}

export interface PackageItem {
  name: string
  repo: string
  source: string
  manager?: string
  target?: string
  tag?: string
  version?: string
  installed: boolean
  installedTag?: string
  installedAt?: string
  asset?: string
  assetSize?: number
  url?: string
  isGui?: boolean
  installMode?: string
  ignoreUpdate?: boolean
}

export interface PackageDetail extends PackageItem {
  desc?: string
  homepage?: string
  repoUrl?: string
  configured: boolean
  configTarget?: string
  installTarget?: string
  assetUrl?: string
  tool?: string
  extractedFiles?: string[]
  options?: Record<string, unknown>
  updatedAt?: string
}

export interface PackagesResponse {
  total: number
  items: PackageItem[]
}

export interface OutdatedItem {
  name: string
  repo: string
  source: string
  manager?: string
  target?: string
  installedTag: string
  latestTag: string
  installedAt?: string
  publishedAt?: string
}

export interface OutdatedResponse {
  checked: number
  items: OutdatedItem[]
  failures?: Failure[]
}

export interface Failure {
  name: string
  repo?: string
  error: string
}

export interface CacheListFile {
  kind: string
  path: string
  path_key?: string
  size: number
  mod_time: string
}

export interface CacheList {
  cache_dir: string
  root: string
  total_files: number
  total_size: number
  files: CacheListFile[]
}

export interface CacheStatus {
  cache_dir: string
  generated_at: string
  total_files: number
  total_size: number
  kinds: Record<string, { files: number; size: number }>
  serve_command: string
  cache_mirror: { enable: boolean; url?: string; timeout?: number; fallback: boolean }
}

export interface ConfigView {
  path: string
  exists: boolean
  content: string
}
