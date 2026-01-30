#!/bin/bash
set -e

# Script to run the Supalite dashboard in development mode with hot module replacement (HMR)
# Usage: ./scripts/dev-dashboard.sh
#
# This script starts the Vite dev server for the dashboard on port 5173.
# The Vite server proxies API requests to the backend (port 8080).
#
# Proxy configuration:
#   - /rest/* → http://localhost:8080 (PostgREST API)
#   - /auth/* → http://localhost:8080 (GoTrue Auth)
#   - /_/api/* → http://localhost:8080 (Dashboard API endpoints)
#
# IMPORTANT: The Supalite backend must be running on port 8080 for the dashboard to work.
# Start the backend with: ./supalite serve
# Or run in the background: ./supalite serve &

echo "=========================================="
echo "Supalite Dashboard Dev Server"
echo "=========================================="
echo ""

# Check if backend is running on port 8080
BACKEND_URL="http://localhost:8080"
if ! curl -s -f "$BACKEND_URL/health" > /dev/null 2>&1; then
    echo "⚠️  WARNING: Supalite backend is not running on port 8080"
    echo ""
    echo "The dashboard needs the backend to handle API requests."
    echo "Please start the backend in another terminal:"
    echo ""
    echo "  ./supalite serve"
    echo ""
    echo "Or run it in the background:"
    echo "  ./supalite serve &"
    echo ""
    read -p "Press Enter to continue anyway, or Ctrl+C to exit..."
fi

# Change to dashboard directory
DASHBOARD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/dashboard"
cd "$DASHBOARD_DIR"

echo "Starting Vite dev server..."
echo "Dashboard URL: http://localhost:5173"
echo "Backend API: $BACKEND_URL"
echo ""
echo "Press Ctrl+C to stop the dev server"
echo "=========================================="
echo ""

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "⚠️  node_modules not found. Installing dependencies..."
    npm install
    echo ""
fi

# Start Vite dev server
npm run dev
