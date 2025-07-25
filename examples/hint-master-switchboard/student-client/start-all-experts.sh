#!/bin/bash

# Start All AI Experts for Switchboard Integration
# Each expert connects to Switchboard as a student

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🎓 Starting All AI Programming Mentorship Experts (Switchboard Integration)"
echo "═════════════════════════════════════════════════════════════════════════"
echo "Make sure Switchboard server is running on http://localhost:8080"
echo "Make sure teacher has created a session with all expert user IDs"
echo "═════════════════════════════════════════════════════════════════════════"

# Check if Switchboard is running
if ! curl -s http://localhost:8080/health > /dev/null; then
    echo "❌ ERROR: Switchboard server is not running on http://localhost:8080"
    echo "Please start the Switchboard server first:"
    echo "   cd /path/to/switchboard && make run"
    exit 1
fi

echo "✅ Switchboard server is running"
echo ""

# Function to start an expert in background
start_expert() {
    local config_file="$1"
    local expert_name="$2"
    
    if [ ! -f "experts/$config_file" ]; then
        echo "❌ Config file not found: experts/$config_file"
        return 1
    fi
    
    echo "🤖 Starting $expert_name..."
    node switchboard-expert.js --config "experts/$config_file" &
    local pid=$!
    echo "   PID: $pid"
    
    # Give each expert a moment to start
    sleep 2
}

# Start all experts
echo "Starting AI Experts (each will connect as a student to Switchboard):"
echo ""

start_expert "technical-expert.json" "Technical Expert"
start_expert "emotional-support.json" "Emotional Support Coach" 
start_expert "debugging-guru.json" "Debugging Guru"
start_expert "learning-coach.json" "Learning Coach"
start_expert "architecture-expert.json" "Architecture Expert"

echo ""
echo "✅ All experts started!"
echo ""
echo "💡 What happens next:"
echo "   1. Each expert discovers sessions they're enrolled in"
echo "   2. Experts automatically connect to available sessions"
echo "   3. When teacher broadcasts problems, experts generate hints"
echo "   4. Hints are sent back via Switchboard message routing"
echo ""
echo "🎯 To use:"
echo "   1. Open teacher dashboard: http://localhost:3000"
echo "   2. Create a new session (experts will auto-connect)"
echo "   3. Configure and broadcast a programming problem"
echo "   4. Watch as AI hints arrive in real-time!"
echo ""
echo "Press Ctrl+C to stop all experts"

# Wait for all background processes
wait