interface StatusCardProps {
  title: string
  value: string
  description?: string
  icon?: React.ReactNode
}

function StatusCard({ title, value, description, icon }: StatusCardProps) {
  return (
    <div className="bg-white overflow-hidden shadow rounded-lg">
      <div className="p-5">
        <div className="flex items-center">
          <div className="flex-shrink-0">
            {icon || (
              <div className="h-6 w-6 rounded bg-indigo-500 flex items-center justify-center">
                <span className="text-white text-xs">✓</span>
              </div>
            )}
          </div>
          <div className="ml-5 w-0 flex-1">
            <dl>
              <dt className="text-sm font-medium text-gray-500 truncate">{title}</dt>
              <dd className="flex items-baseline">
                <div className="text-2xl font-semibold text-gray-900">{value}</div>
              </dd>
            </dl>
          </div>
        </div>
      </div>
      {description && (
        <div className="bg-gray-50 px-5 py-3">
          <div className="text-sm text-gray-500">{description}</div>
        </div>
      )}
    </div>
  )
}

export default StatusCard
