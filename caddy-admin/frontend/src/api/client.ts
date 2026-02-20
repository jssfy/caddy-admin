import type { CertsResponse, ServicesResponse, SiteInfo, SitesResponse, StatusResponse } from '../types'

const BASE = '/api'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path)
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? 'request failed')
  }
  return res.json()
}

async function del<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path, { method: 'DELETE' })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? 'request failed')
  }
  return res.json()
}

export const api = {
  status: () => get<StatusResponse>('/status'),
  sites: () => get<SitesResponse>('/sites'),
  site: (domain: string) => get<SiteInfo>(`/sites/${encodeURIComponent(domain)}`),
  certs: () => get<CertsResponse>('/certs'),
  services: () => get<ServicesResponse>('/services'),
  deleteService: (name: string) => del<{ deleted: boolean; name: string }>(`/services/${encodeURIComponent(name)}`),
}
