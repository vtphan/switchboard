#!/bin/bash

# Switchboard V4 Fresh Start Script
# Removes database, rebuilds, and starts the server with a clean state

set -e  # Exit on any error

echo "🧹 Switchboard V4 Fresh Start"
echo "=============================="

# Check if we're in the right directory
if [ ! -f "Makefile" ]; then
    echo "❌ Error: Run this script from the switchboard project root directory"
    exit 1
fi

# Stop any running server processes
echo "🛑 Stopping any running server processes..."
pkill -f "switchboard" || true
sleep 2

# Remove database files
echo "🗑️  Removing database files..."
if [ -f "db/switchboard.db" ]; then
    rm -f db/switchboard.db*
    echo "   ✅ Database files removed"
else
    echo "   ℹ️  No database files found"
fi

# Clean build artifacts
echo "🧹 Cleaning build artifacts..."
make clean || true

# Rebuild the application
echo "🔨 Building application..."
make build

# Initialize fresh database
echo "💾 Initializing fresh database..."
make init-db

# Start the server
echo "🚀 Starting server..."
echo ""
echo "📍 Server will be available at:"
echo "   - Web UI: http://localhost:8080"
echo "   - WebSocket: ws://localhost:8080/ws"
echo "   - Teacher Client: http://localhost:8080/sdk/javascript/examples/teacher/"
echo "   - Student Client: http://localhost:8080/sdk/javascript/examples/student/"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""

exec make run