import { useState } from 'react'
import { NavLink, Route, Routes } from 'react-router-dom'
import BrandMark from './components/BrandMark'
import TaskActivity from './components/TaskActivity'
import Overview from './pages/Overview'
import Packages from './pages/Packages'
import PackageDetail from './pages/PackageDetail'
import Install from './pages/Install'
import Outdated from './pages/Outdated'
import Ext from './pages/Ext'
import Cache from './pages/Cache'
import Config from './pages/Config'
import Tasks from './pages/Tasks'

// `key` is the short form the rail shows while collapsed, so the collapsed rail
// still names its destinations without a row of unlabelled glyphs.
const navigation = [
  { to: '/', label: 'Overview', key: 'ov', end: true },
  { to: '/packages', label: 'Packages', key: 'pk' },
  { to: '/install', label: 'Install', key: 'in' },
  { to: '/outdated', label: 'Outdated', key: 'ou' },
  { to: '/ext', label: 'External', key: 'ex' },
  { to: '/cache', label: 'Cache', key: 'ca' },
  { to: '/config', label: 'Config', key: 'cf' },
  { to: '/tasks', label: 'Tasks', key: 'tk' },
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
          <div className="brand" title="eget web">
            <BrandMark />
            <span className="brand-label">
              eget<span>web</span>
            </span>
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
              <span className="nav-key">{item.key}</span>
              <span className="nav-label">{item.label}</span>
            </NavLink>
          ))}
        </nav>
        <TaskActivity />
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
