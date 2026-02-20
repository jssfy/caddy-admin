import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { SiteInfo } from '../types'

const s: Record<string, React.CSSProperties> = {
  heading: { fontSize: 22, fontWeight: 700, marginBottom: 24 },
  card: { background: '#fff', borderRadius: 10, border: '1px solid #e2e8f0', overflow: 'hidden' },
  table: { width: '100%', borderCollapse: 'collapse' },
  th: { padding: '12px 16px', textAlign: 'left', fontSize: 12, fontWeight: 600, color: '#64748b', textTransform: 'uppercase', letterSpacing: 0.5, borderBottom: '1px solid #e2e8f0', background: '#f8fafc' },
  td: { padding: '14px 16px', fontSize: 14, borderBottom: '1px solid #f1f5f9' },
  trHover: { cursor: 'pointer', background: '#f8fafc' },
  badge: { display: 'inline-block', padding: '2px 8px', borderRadius: 9999, fontSize: 12, fontWeight: 500 },
  tls: { background: '#dcfce7', color: '#15803d' },
  noTls: { background: '#fef9c3', color: '#854d0e' },
  typeProxy: { background: '#dbeafe', color: '#1d4ed8' },
  typeStatic: { background: '#f3e8ff', color: '#7e22ce' },
  typeUnknown: { background: '#f1f5f9', color: '#64748b' },
  error: { padding: '20px 16px', color: '#dc2626', fontSize: 14 },
  empty: { padding: '40px 16px', textAlign: 'center', color: '#94a3b8', fontSize: 14 },
}

function typeBadge(type: SiteInfo['type']) {
  const style = type === 'proxy' ? s.typeProxy : type === 'static' ? s.typeStatic : s.typeUnknown
  return <span style={{ ...s.badge, ...style }}>{type}</span>
}

export default function SiteList() {
  const [sites, setSites] = useState<SiteInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [hovered, setHovered] = useState<string | null>(null)
  const navigate = useNavigate()

  useEffect(() => {
    api.sites()
      .then(r => setSites(r.sites ?? []))
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <p style={{ color: '#94a3b8', fontSize: 14 }}>Loading sites...</p>

  return (
    <div>
      <h1 style={s.heading}>Sites <span style={{ fontSize: 16, fontWeight: 400, color: '#64748b' }}>({sites.length})</span></h1>
      <div style={s.card}>
        {error && <p style={s.error}>{error}</p>}
        {!error && sites.length === 0 && <p style={s.empty}>No sites found. Is Caddy running?</p>}
        {sites.length > 0 && (
          <table style={s.table}>
            <thead>
              <tr>
                <th style={s.th}>Domain</th>
                <th style={s.th}>Type</th>
                <th style={s.th}>Target</th>
                <th style={s.th}>TLS</th>
              </tr>
            </thead>
            <tbody>
              {sites.map(site => (
                <tr
                  key={site.domain}
                  style={hovered === site.domain ? { ...s.trHover } : {}}
                  onMouseEnter={() => setHovered(site.domain)}
                  onMouseLeave={() => setHovered(null)}
                  onClick={() => navigate(`/sites/${encodeURIComponent(site.domain)}`)}
                >
                  <td style={s.td}><strong>{site.domain}</strong></td>
                  <td style={s.td}>{typeBadge(site.type)}</td>
                  <td style={{ ...s.td, color: '#475569', fontFamily: 'monospace', fontSize: 13 }}>
                    {site.type === 'proxy' ? site.upstream : site.root ?? '—'}
                  </td>
                  <td style={s.td}>
                    <span style={{ ...s.badge, ...(site.hasTLS ? s.tls : s.noTls) }}>
                      {site.hasTLS ? '✓ TLS' : '— none'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
