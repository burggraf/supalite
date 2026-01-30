import { useState } from 'react'

interface ApiKeyCardProps {
  label: string
  value: string
}

function ApiKeyCard({ label, value }: ApiKeyCardProps) {
  const [copied, setCopied] = useState(false)

  const copyToClipboard = () => {
    navigator.clipboard.writeText(value)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  // Show only first and last 8 characters
  const maskedValue = value.length > 16
    ? `${value.slice(0, 8)}...${value.slice(-8)}`
    : value

  return (
    <div className="bg-white overflow-hidden shadow rounded-lg">
      <div className="p-5">
        <dt className="text-sm font-medium text-gray-500 truncate">{label}</dt>
        <dd className="mt-1">
          <div className="flex items-center justify-between">
            <code className="text-sm text-gray-900 font-mono">{maskedValue}</code>
            <button
              onClick={copyToClipboard}
              className="ml-2 inline-flex items-center px-2.5 py-0.5 border border-gray-300 shadow-sm text-xs font-medium rounded text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
            >
              {copied ? 'Copied!' : 'Copy'}
            </button>
          </div>
        </dd>
      </div>
    </div>
  )
}

export default ApiKeyCard
