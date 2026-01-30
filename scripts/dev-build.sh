#!/bin/bash
set -e

# Script to start the Supalite dashboard in development mode
# Usage: ./scripts/dev-build.sh
#
# This script runs the Vite dev server with hot module replacement (HMR)
# for local development of the dashboard frontend.
#
# Unlike the production build (make build-dashboard), this does NOT copy
# the built files to internal/dashboard for embedding. It simply runs the
# dev server for local development and testing.
#
# The dev server will be available at http://localhost:5173 by default
# (or the port configured in dashboard/vite.config.ts)

echo "=========================================="
echo "Supalite Dashboard - Development Mode"
echo "=========================================="
echo ""

# Check if node_modules exists
if [ ! -d "dashboard/node_modules" ]; then
    echo "Dependencies not found. Installing..."
    cd dashboard && npm install && cd ..
    echo ""
fi

echo "Starting Vite dev server..."
echo "The dashboard will be available at http://localhost:5173"
echo ""
echo "Press Ctrl+C to stop the dev server"
echo ""
echo "=========================================="
echo ""

# Run the dev server from the dashboard directory
cd dashboard && npm run dev
