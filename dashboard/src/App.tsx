import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import LoginPage from './pages/LoginPage'
import ProtectedRoute from './components/ProtectedRoute'

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <div className="min-h-screen bg-gray-50">
                <div className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
                  <h1 className="text-3xl font-bold text-gray-900">Dashboard Home</h1>
                  <p className="mt-2 text-gray-600">Welcome to the Supalite Dashboard</p>
                </div>
              </div>
            </ProtectedRoute>
          }
        />
      </Routes>
    </Router>
  )
}

export default App
