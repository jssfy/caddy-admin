import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { SiteInfo } from '../types'

const s: Record<string, React.CSSProperties> = {
  back: { color: '#64748b', fontSize: 13, marginBottom: 20, display: 'inline-block' },
  heading: { fontSize: 22, fontWeight: 700, marginBottom: 24 },
  card: { background: '#fff', borderRadius: 10, border: '1px solid #e2e8f0', padding: '24px' },
  grid: { display: 'grid', gap: 20 },
  row: { display: 'grid', gridTemplateColumns: '160px 1fr', gap: 12, alignItems: 'start', paddingBottom: 16, borderBottom: '1px solid #f1f5f9' },
  rowLast: { display: 'grid', gridTemplateColumns: '160px 1fr', gap: 12, alignItems: 'start' },
  label: { fontSize: 12, fontWeight: 600, color: '#64748b', textTransform: 'uppercase', letterSpacing: 0.5, paddingTop: 2 },
  value: { fontSize: 14, color: '#1e293b' },
  mono: { fontSize: 13, fontFamily: 'monospace', color: '#1e293b', background: '#f8fafc', padding: '4px 8px', borderRadius: 4, display: 'inline-block' },
  badge: { display: 'inline-block', padding: '2px 8px', borderRadius: 9999, fontSize: 12, fontWeight: 500 },
  tls: { background: '#dcfce7', color: '#15803d' },
  noTls: { background: '#fef9c3', color: '#854d0e' },
  headerTable: { borderCollapse: 'collapse', width: '100%' },
  htd: { padding: '4px 8px', fontSize: 13, fontFamily: 'monospace', color: '#1e293b' },
  error: { color: '#dc2626', fontSize: 14 },
}

export default function SiteDetail() {
  const { domain } = useParams<{ domain: string }>()
  const [site, setSite] = useState<SiteInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!domain) return
    api.site(domain)
      .then(setSite)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [domain])

  if (loading) return <p style={{ color: '#94a3b8', fontSize: 14 }}>Loading...</p>
  if (error) return <p style={s.error}>{error}</p>
  if (!site) return null

  const rows: { label: string; value: React.ReactNode }[] = [
    { label: 'Domain', value: <span style={s.mono}>{site.domain}</span> },
    {
      label: 'Type',
      value: <span style={{ ...s.badge, background: site.type === 'proxy' ? '#dbeafe' : site.type === 'static' ? '#f3e8ff' : '#f1f5f9', color: site.type === 'proxy' ? '#1d4ed8' : site.type === 'static' ? '#7e22ce' : '#64748b' }}>{site.type}</span>
    },
    ...(site.type === 'static' ? [{ label: 'Root directory', value: <span style={s.mono}>{site.root ?? '—'}</span> }] : []),
    ...(site.type === 'proxy' ? [{ label: 'Upstream', value: <span style={s.mono}>{site.upstream ?? '—'}</span> }] : []),
    { label: 'TLS', value: <span style={{ ...s.badge, ...(site.hasTLS ? s.tls : s.noTls) }}>{site.hasTLS ? '✓ Managed' : '— None'}</span> },
    {
      label: 'Response headers',
      value: site.headers && Object.keys(site.headers).length > 0
        ? <table style={s.headerTable}>
            <tbody>
              {Object.entries(site.headers).map(([k, v]) => (
                <tr key={k}>
                  <td style={{ ...s.htd, color: '#7c3aed', paddingLeft: 0 }}>{k}</td>
                  <td style={{ ...s.htd, color: '#475569' }}>{v}</td>
                </tr>
              ))}
            </tbody>
          </table>
        : <span style={{ color: '#94a3b8' }}>—</span>
    },
  ]

  return (
    <div>
      <Link to="/sites" style={s.back}>← Back to Sites</Link>
      <h1 style={s.heading}>{site.domain}</h1>
      <div style={s.card}>
        <div style={s.grid}>
          {rows.map((row, i) => (
            <div key={row.label} style={i === rows.length - 1 ? s.rowLast : s.row}>
              <div style={s.label}>{row.label}</div>
              <div style={s.value}>{row.value}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
