import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import Header from '../components/Header'

interface Table {
  name: string
  schema: string
  rows?: number
  size_bytes?: string
}

interface Column {
  name: string
  type: string
  nullable: boolean
  key?: string
}

interface TableSchema {
  table_name: string
  schema: string
  columns: Column[]
}

function TablesPage() {
  const [tables, setTables] = useState<Table[]>([])
  const [selectedTable, setSelectedTable] = useState<Table | null>(null)
  const [tableSchema, setTableSchema] = useState<TableSchema | null>(null)
  const [userEmail, setUserEmail] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [schemaLoading, setSchemaLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const loadData = async () => {
      try {
        const [tablesData, userData] = await Promise.all([
          api.getTables(),
          api.me(),
        ])
        setTables(tablesData.tables)
        setUserEmail(userData.email)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load data')
      } finally {
        setLoading(false)
      }
    }

    loadData()
  }, [])

  const handleTableClick = async (table: Table) => {
    setSelectedTable(table)
    setSchemaLoading(true)
    setError('')

    try {
      const schema = await api.getTableSchema(table.name)
      setTableSchema(schema)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load table schema')
    } finally {
      setSchemaLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-indigo-600"></div>
      </div>
    )
  }

  if (error && !selectedTable) {
    return (
      <>
        <Header userEmail={userEmail} />
        <div className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
          <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
            {error}
          </div>
        </div>
      </>
    )
  }

  return (
    <>
      <Header userEmail={userEmail} />
      <div className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        <div className="md:flex md:items-center md:justify-between mb-6">
          <div className="flex-1 min-w-0">
            <h2 className="text-2xl font-bold leading-7 text-gray-900 sm:text-3xl sm:truncate">
              Tables
            </h2>
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Table List */}
          <div className="lg:col-span-1">
            <div className="bg-white shadow overflow-hidden rounded-md">
              <ul className="divide-y divide-gray-200">
                {tables.map((table) => (
                  <li
                    key={`${table.schema}-${table.name}`}
                    onClick={() => handleTableClick(table)}
                    className={`px-4 py-4 sm:px-6 hover:bg-gray-50 cursor-pointer ${
                      selectedTable?.name === table.name ? 'bg-indigo-50' : ''
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <div className="text-sm font-medium text-indigo-600 truncate">
                        {table.name}
                      </div>
                      <div className="ml-2 flex-shrink-0 flex">
                        <p className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-green-100 text-green-800">
                          {table.rows || 0} rows
                        </p>
                      </div>
                    </div>
                    <div className="mt-1 sm:flex sm:justify-between">
                      <div className="sm:flex">
                        <p className="flex items-center text-sm text-gray-500">
                          {table.schema}
                        </p>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          </div>

          {/* Table Schema */}
          <div className="lg:col-span-2">
            {schemaLoading ? (
              <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-indigo-600"></div>
              </div>
            ) : tableSchema ? (
              <div className="bg-white shadow overflow-hidden sm:rounded-lg">
                <div className="px-4 py-5 sm:px-6">
                  <h3 className="text-lg leading-6 font-medium text-gray-900">
                    {tableSchema.schema}.{tableSchema.table_name}
                  </h3>
                  <p className="mt-1 max-w-2xl text-sm text-gray-500">
                    Table schema and column information
                  </p>
                </div>
                <div className="border-t border-gray-200">
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                      <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Column
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Type
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Nullable
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Key
                        </th>
                      </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                      {tableSchema.columns.map((column) => (
                        <tr key={column.name}>
                          <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                            {column.name}
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                            {column.type}
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                            {column.nullable ? 'Yes' : 'No'}
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                            {column.key || '-'}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : (
              <div className="bg-white shadow rounded-lg p-6 text-center text-gray-500">
                Select a table to view its schema
              </div>
            )}
          </div>
        </div>
      </div>
    </>
  )
}

export default TablesPage
