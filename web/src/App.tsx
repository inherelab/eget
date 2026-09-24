import { useState } from 'react'
import { NavLink, Route, Routes } from 'react-router-dom'
import Overview from './pages/Overview'
import Packages from './pages/Packages'
import PackageDetail from './pages/PackageDetail'
import Install from './pages/Install'
import Outdated from './pages/Outdated'
import Ext from './pages/Ext'
import Cache from './pages/Cache'
import Config from './pages/Config'
import Tasks from './pages/Tasks'

const navigation = [
  { to: '/', label: 'Overview', icon: '◍', end: true },
  { to: '/packages', label: 'Packages', icon: '▤' },
  { to: '/install', label: 'Install', icon: '＋' },
  { to: '/outdated', label: 'Outdated', icon: '↑' },
  { to: '/ext', label: 'External', icon: '⊕' },
  { to: '/cache', label: 'Cache', icon: '▣' },
  { to: '/config', label: 'Config', icon: '⚙' },
  { to: '/tasks', label: 'Tasks', icon: '⟳' },
]

const sidebarKey = 'eget-web-sidebar'

export default function App() {
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem(sidebarKey) === 'collapsed')

  const toggleSidebar = () => {
    setCollapsed((previous) => {
      const next = !previous
      localStorage.setItem(sidebarKey, next ? 'collapsed' : 'expanded')
      return next
    })
  }

  return (
    <div className={collapsed ? 'shell collapsed' : 'shell'}>
      <aside className="sidebar">
        <div className="sidebar-head">
          <div className="brand">
            {collapsed ? 'e' : (
              <>
                eget<span>web</span>
              </>
            )}
          </div>
          <button
            type="button"
            className="sidebar-toggle"
            onClick={toggleSidebar}
            title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            {collapsed ? '»' : '«'}
          </button>
        </div>
        <nav>
          {navigation.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              title={item.label}
              className={({ isActive }) => (isActive ? 'nav-item active' : 'nav-item')}
            >
              <span className="nav-icon">{item.icon}</span>
              <span className="nav-label">{item.label}</span>
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
          <Route path="/install" element={<Install />} />
          <Route path="/outdated" element={<Outdated />} />
          <Route path="/ext" element={<Ext />} />
          <Route path="/cache" element={<Cache />} />
          <Route path="/config" element={<Config />} />
          <Route path="/tasks" element={<Tasks />} />
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
