import { Link, useParams } from 'react-router-dom'
import PackagePanel from '../components/PackagePanel'

// PackageDetail is the addressable page for one package. The packages list opens
// the same panel in a drawer; this route stays for links and history.
export default function PackageDetail() {
  const { name = '' } = useParams()

  return (
    <section className="page">
      <div className="page-head">
        <h1>{name}</h1>
        <Link to="/packages">← Back to packages</Link>
      </div>
      <PackagePanel name={name} />
    </section>
  )
}
