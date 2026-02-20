export interface SiteInfo {
  domain: string
  type: 'static' | 'proxy' | 'unknown'
  root?: string
  upstream?: string
  headers?: Record<string, string>
  hasTLS: boolean
}

export interface CertInfo {
  domain: string
  issuer: string
  notBefore: string
  notAfter: string
  daysLeft: number
  isExpired: boolean
  source: 'letsencrypt' | 'zerossl' | 'local' | 'unknown'
}

export interface SitesResponse {
  sites: SiteInfo[]
  total: number
}

export interface CertsResponse {
  certs: CertInfo[]
  total: number
  message?: string
}

export interface ServiceInfo {
  name: string
  domain: string
  upstream: string
}

export interface ServicesResponse {
  services: ServiceInfo[]
  total: number
}

export interface StatusResponse {
  caddy: boolean
}
