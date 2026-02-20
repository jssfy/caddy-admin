import { useEffect, useState } from 'react'
import type { ServiceInfo } from '../types'
import { api } from '../api/client'

const s: Record<string, React.CSSProperties> = {
  heading: { fontSize: 22, fontWeight: 700, marginBottom: 24 },
  card: { background: '#fff', borderRadius: 10, border: '1px solid #e2e8f0', overflow: 'hidden' },
  table: { width: '100%', borderCollapse: 'collapse' },
  th: { padding: '12px 16px', textAlign: 'left', fontSize: 12, fontWeight: 600, color: '#64748b', textTransform: 'uppercase', letterSpacing: 0.5, borderBottom: '1px solid #e2e8f0', background: '#f8fafc' },
  td: { padding: '14px 16px', fontSize: 14, borderBottom: '1px solid #f1f5f9' },
  error: { padding: '20px 16px', color: '#dc2626', fontSize: 14 },
  empty: { padding: '40px 16px', textAlign: 'center', color: '#94a3b8', fontSize: 14 },
  deleteBtn: { background: 'none', border: 'none', color: '#dc2626', cursor: 'pointer', fontSize: 13, fontWeight: 500, padding: '4px 8px' },
  deleteBtnDisabled: { background: 'none', border: 'none', color: '#94a3b8', cursor: 'not-allowed', fontSize: 13, fontWeight: 500, padding: '4px 8px' },
}

export default function ServiceList() {
  const [services, setServices] = useState<ServiceInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState<string | null>(null)

  useEffect(() => {
    api.services()
      .then(r => setServices(r.services ?? []))
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  async function handleDelete(name: string) {
    setDeleting(name)
    setError(null)
    try {
      await api.deleteService(name)
      setServices(prev => prev.filter(svc => svc.name !== name))
    } catch (e: any) {
      setError(e.message)
    } finally {
      setDeleting(null)
    }
  }

  if (loading) return <p style={{ color: '#94a3b8', fontSize: 14 }}>Loading services...</p>

  return (
    <div>
      <h1 style={s.heading}>Services <span style={{ fontSize: 16, fontWeight: 400, color: '#64748b' }}>({services.length})</span></h1>
      <div style={s.card}>
        {error && <p style={s.error}>{error}</p>}
        {!error && services.length === 0 && (
          <p style={s.empty}>No dynamically registered services.</p>
        )}
        {services.length > 0 && (
          <table style={s.table}>
            <thead>
              <tr>
                <th style={s.th}>Name</th>
                <th style={s.th}>Domain</th>
                <th style={s.th}>Upstream</th>
                <th style={s.th}>Action</th>
              </tr>
            </thead>
            <tbody>
              {services.map(svc => (
                <tr key={svc.name}>
                  <td style={{ ...s.td, fontWeight: 600 }}>{svc.name}</td>
                  <td style={{ ...s.td, color: '#475569' }}>{svc.domain}</td>
                  <td style={{ ...s.td, fontFamily: 'monospace', fontSize: 13, color: '#475569' }}>{svc.upstream}</td>
                  <td style={s.td}>
                    <button
                      style={deleting === svc.name ? s.deleteBtnDisabled : s.deleteBtn}
                      disabled={deleting === svc.name}
                      onClick={() => handleDelete(svc.name)}
                    >
                      {deleting === svc.name ? 'Deleting...' : 'Delete'}
                    </button>
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
