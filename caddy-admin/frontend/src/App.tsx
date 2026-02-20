import { useEffect, useState } from 'react'
import { Link, Route, Routes, useLocation } from 'react-router-dom'
import { api } from './api/client'
import CertList from './pages/CertList'
import ServiceList from './pages/ServiceList'
import SiteDetail from './pages/SiteDetail'
import SiteList from './pages/SiteList'

const s: Record<string, React.CSSProperties> = {
  layout: { minHeight: '100vh', display: 'flex', flexDirection: 'column' },
  nav: {
    background: '#0f172a', color: '#f8fafc', padding: '0 24px',
    display: 'flex', alignItems: 'center', gap: 32, height: 56,
  },
  brand: { fontWeight: 700, fontSize: 18, letterSpacing: -0.5 },
  navLink: { fontSize: 14, color: '#94a3b8', cursor: 'pointer' },
  navLinkActive: { fontSize: 14, color: '#f8fafc', fontWeight: 600 },
  dot: { width: 8, height: 8, borderRadius: '50%', display: 'inline-block', marginRight: 6 },
  main: { flex: 1, padding: '32px 24px', maxWidth: 960, margin: '0 auto', width: '100%' },
}

export default function App() {
  const location = useLocation()
  const [caddyRunning, setCaddyRunning] = useState<boolean | null>(null)

  useEffect(() => {
    api.status().then(r => setCaddyRunning(r.caddy)).catch(() => setCaddyRunning(false))
    const id = setInterval(() => {
      api.status().then(r => setCaddyRunning(r.caddy)).catch(() => setCaddyRunning(false))
    }, 10000)
    return () => clearInterval(id)
  }, [])

  const isActive = (path: string) => location.pathname === path || location.pathname.startsWith(path + '/')

  return (
    <div style={s.layout}>
      <nav style={s.nav}>
        <Link to="/" style={s.brand}>
          <span style={{ ...s.dot, background: caddyRunning === null ? '#64748b' : caddyRunning ? '#22c55e' : '#ef4444' }} />
          Caddy Admin
        </Link>
        <Link to="/sites" style={isActive('/sites') ? s.navLinkActive : s.navLink}>Sites</Link>
        <Link to="/certs" style={isActive('/certs') ? s.navLinkActive : s.navLink}>Certificates</Link>
        <Link to="/services" style={isActive('/services') ? s.navLinkActive : s.navLink}>Services</Link>
        <span style={{ marginLeft: 'auto', fontSize: 13, color: '#475569' }}>
          Caddy: {caddyRunning === null ? '...' : caddyRunning ? '● running' : '● offline'}
        </span>
      </nav>
      <main style={s.main}>
        <Routes>
          <Route path="/" element={<SiteList />} />
          <Route path="/sites" element={<SiteList />} />
          <Route path="/sites/:domain" element={<SiteDetail />} />
          <Route path="/certs" element={<CertList />} />
          <Route path="/services" element={<ServiceList />} />
        </Routes>
      </main>
    </div>
  )
}
