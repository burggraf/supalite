const API_BASE = '/_/api'

// Token management
export const getToken = (): string | null => {
  return localStorage.getItem('access_token')
}

export const setToken = (token: string): void => {
  localStorage.setItem('access_token', token)
}

export const removeToken = (): void => {
  localStorage.removeItem('access_token')
}

// Authenticated fetch wrapper
export const authFetch = async (url: string, options: RequestInit = {}): Promise<Response> => {
  const token = getToken()
  const headers = {
    ...options.headers,
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  }

  const response = await fetch(`${API_BASE}${url}`, {
    ...options,
    headers,
  })

  if (response.status === 401) {
    // Token expired or invalid - redirect to login
    removeToken()
    window.location.href = '/_/login'
    throw new Error('Unauthorized')
  }

  return response
}

// API object with all backend methods
export const api = {
  // Auth
  login: async (email: string, password: string) => {
    const response = await fetch(`${API_BASE}/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Login failed' }))
      throw new Error(error.message || 'Login failed')
    }

    const data = await response.json()
    setToken(data.access_token)
    return data
  },

  logout: () => {
    removeToken()
    window.location.href = '/_/login'
  },

  me: async () => {
    const response = await authFetch('/me')
    if (!response.ok) throw new Error('Failed to fetch user')
    return response.json()
  },

  // Status
  getStatus: async () => {
    const response = await authFetch('/status')
    if (!response.ok) throw new Error('Failed to fetch status')
    return response.json()
  },

  // Tables
  getTables: async () => {
    const response = await authFetch('/tables')
    if (!response.ok) throw new Error('Failed to fetch tables')
    return response.json()
  },

  getTableSchema: async (tableName: string) => {
    const response = await authFetch(`/tables/${encodeURIComponent(tableName)}/schema`)
    if (!response.ok) throw new Error('Failed to fetch table schema')
    return response.json()
  },
}

export default api
