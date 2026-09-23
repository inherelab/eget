import { NavLink, Route, Routes } from 'react-router-dom'
import Overview from './pages/Overview'
import Packages from './pages/Packages'
import PackageDetail from './pages/PackageDetail'
import Outdated from './pages/Outdated'
import Ext from './pages/Ext'
import Cache from './pages/Cache'
import Config from './pages/Config'

const navigation = [
  { to: '/', label: 'Overview', end: true },
  { to: '/packages', label: 'Packages' },
  { to: '/outdated', label: 'Outdated' },
  { to: '/ext', label: 'External' },
  { to: '/cache', label: 'Cache' },
  { to: '/config', label: 'Config' },
]

export default function App() {
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          eget<span>web</span>
        </div>
        <nav>
          {navigation.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) => (isActive ? 'nav-item active' : 'nav-item')}
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-foot">served by eget web</div>
      </aside>
      <main className="content">
        <Routes>
          <Route path="/" element={<Overview />} />
          <Route path="/packages" element={<Packages />} />
          <Route path="/packages/:name" element={<PackageDetail />} />
          <Route path="/outdated" element={<Outdated />} />
          <Route path="/ext" element={<Ext />} />
          <Route path="/cache" element={<Cache />} />
          <Route path="/config" element={<Config />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </main>
    </div>
  )
}

function NotFoundPage() {
  return (
    <section className="page">
      <h1>Not found</h1>
      <p className="muted">This console route does not exist.</p>
    </section>
  )
}
