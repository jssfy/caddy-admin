import { useEffect, useState } from 'react'
import type { CertInfo } from '../types'
import { api } from '../api/client'

const s: Record<string, React.CSSProperties> = {
  heading: { fontSize: 22, fontWeight: 700, marginBottom: 24 },
  card: { background: '#fff', borderRadius: 10, border: '1px solid #e2e8f0', overflow: 'hidden' },
  table: { width: '100%', borderCollapse: 'collapse' },
  th: { padding: '12px 16px', textAlign: 'left', fontSize: 12, fontWeight: 600, color: '#64748b', textTransform: 'uppercase', letterSpacing: 0.5, borderBottom: '1px solid #e2e8f0', background: '#f8fafc' },
  td: { padding: '14px 16px', fontSize: 14, borderBottom: '1px solid #f1f5f9' },
  badge: { display: 'inline-block', padding: '2px 8px', borderRadius: 9999, fontSize: 12, fontWeight: 500 },
  ok: { background: '#dcfce7', color: '#15803d' },
  warn: { background: '#fef9c3', color: '#854d0e' },
  expired: { background: '#fee2e2', color: '#991b1b' },
  sourceLE: { background: '#dbeafe', color: '#1d4ed8' },
  sourceLocal: { background: '#f3e8ff', color: '#7e22ce' },
  sourceOther: { background: '#f1f5f9', color: '#64748b' },
  error: { padding: '20px 16px', color: '#dc2626', fontSize: 14 },
  empty: { padding: '40px 16px', textAlign: 'center', color: '#94a3b8', fontSize: 14 },
}

function daysBadge(cert: CertInfo) {
  if (cert.isExpired) return <span style={{ ...s.badge, ...s.expired }}>Expired</span>
  if (cert.daysLeft < 14) return <span style={{ ...s.badge, ...s.warn }}>{cert.daysLeft}d left</span>
  return <span style={{ ...s.badge, ...s.ok }}>{cert.daysLeft}d left</span>
}

function sourceBadge(source: CertInfo['source']) {
  const map: Record<string, React.CSSProperties> = {
    letsencrypt: s.sourceLE,
    zerossl: s.sourceLE,
    local: s.sourceLocal,
  }
  return <span style={{ ...s.badge, ...(map[source] ?? s.sourceOther) }}>{source}</span>
}

function fmtDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-GB', { year: 'numeric', month: 'short', day: '2-digit' })
}

export default function CertList() {
  const [certs, setCerts] = useState<CertInfo[]>([])
  const [message, setMessage] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.certs()
      .then(r => { setCerts(r.certs ?? []); setMessage(r.message ?? null) })
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <p style={{ color: '#94a3b8', fontSize: 14 }}>Loading certificates...</p>

  return (
    <div>
      <h1 style={s.heading}>Certificates <span style={{ fontSize: 16, fontWeight: 400, color: '#64748b' }}>({certs.length})</span></h1>
      {message && <p style={{ marginBottom: 16, fontSize: 13, color: '#94a3b8', background: '#f8fafc', padding: '8px 12px', borderRadius: 6 }}>{message}</p>}
      <div style={s.card}>
        {error && <p style={s.error}>{error}</p>}
        {!error && certs.length === 0 && (
          <p style={s.empty}>
            {message ? message : 'No certificates found. Caddy stores certs in /data/caddy/certificates/'}
          </p>
        )}
        {certs.length > 0 && (
          <table style={s.table}>
            <thead>
              <tr>
                <th style={s.th}>Domain</th>
                <th style={s.th}>Issuer</th>
                <th style={s.th}>Source</th>
                <th style={s.th}>Valid until</th>
                <th style={s.th}>Status</th>
              </tr>
            </thead>
            <tbody>
              {certs.map(cert => (
                <tr key={cert.domain}>
                  <td style={{ ...s.td, fontWeight: 600 }}>{cert.domain}</td>
                  <td style={{ ...s.td, color: '#475569' }}>{cert.issuer || '—'}</td>
                  <td style={s.td}>{sourceBadge(cert.source)}</td>
                  <td style={{ ...s.td, color: '#475569', fontFamily: 'monospace', fontSize: 13 }}>{fmtDate(cert.notAfter)}</td>
                  <td style={s.td}>{daysBadge(cert)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
