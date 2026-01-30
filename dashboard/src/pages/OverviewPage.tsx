import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import StatusCard from '../components/StatusCard'
import ApiKeyCard from '../components/ApiKeyCard'

interface StatusResponse {
  status: string
  timestamp: string
  uptime: string
  version: string
}

interface TablesResponse {
  tables: Array<{
    name: string
    schema: string
    rows?: number
    size_bytes?: string
  }>
}

function OverviewPage() {
  const [status, setStatus] = useState<StatusResponse | null>(null)
  const [tables, setTables] = useState<TablesResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    const loadData = async () => {
      try {
        const [statusData, tablesData] = await Promise.all([
          api.getStatus(),
          api.getTables(),
        ])
        setStatus(statusData)
        setTables(tablesData)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load data')
      } finally {
        setLoading(false)
      }
    }

    loadData()
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-indigo-600"></div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
        {error}
      </div>
    )
  }

  const totalRows = tables?.tables.reduce((sum, t) => sum + (t.rows || 0), 0) || 0

  return (
    <div>
      <div className="md:flex md:items-center md:justify-between">
        <div className="flex-1 min-w-0">
          <h2 className="text-2xl font-bold leading-7 text-gray-900 sm:text-3xl sm:truncate">
            Overview
          </h2>
        </div>
      </div>

      {/* Status Cards */}
      <div className="mt-8">
        <h3 className="text-lg leading-6 font-medium text-gray-900">System Status</h3>
        <dl className="mt-5 grid grid-cols-1 gap-5 sm:grid-cols-3">
          <StatusCard
            title="Status"
            value={status?.status || 'Unknown'}
            description={status?.uptime && `Uptime: ${status.uptime}`}
          />
          <StatusCard
            title="Tables"
            value={tables?.tables.length.toString() || '0'}
            description="Total tables in database"
          />
          <StatusCard
            title="Total Rows"
            value={totalRows.toString()}
            description="Across all tables"
          />
        </dl>
      </div>

      {/* API Keys */}
      <div className="mt-8">
        <h3 className="text-lg leading-6 font-medium text-gray-900">API Keys</h3>
        <p className="mt-1 text-sm text-gray-500">
          Use these keys to interact with your Supalite instance
        </p>
        <div className="mt-4 grid grid-cols-1 gap-5 sm:grid-cols-2">
          <ApiKeyCard
            label="Anon Key"
            value={localStorage.getItem('anon_key') || 'Not set'}
          />
          <ApiKeyCard
            label="Service Role Key"
            value={localStorage.getItem('service_role_key') || 'Not set'}
          />
        </div>
      </div>

      {/* Tables Preview */}
      <div className="mt-8">
        <h3 className="text-lg leading-6 font-medium text-gray-900">Database Tables</h3>
        <div className="mt-4 bg-white shadow overflow-hidden rounded-md">
          <ul className="divide-y divide-gray-200">
            {tables?.tables.slice(0, 5).map((table) => (
              <li key={`${table.schema}-${table.name}`} className="px-4 py-4 sm:px-6 hover:bg-gray-50">
                <div className="flex items-center justify-between">
                  <div className="text-sm font-medium text-indigo-600 truncate">
                    {table.schema}.{table.name}
                  </div>
                  <div className="ml-2 flex-shrink-0 flex">
                    <p className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-green-100 text-green-800">
                      {table.rows || 0} rows
                    </p>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  )
}

export default OverviewPage
