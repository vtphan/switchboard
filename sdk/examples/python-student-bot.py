#!/usr/bin/env python3
"""
AI Expert Bot Example using Switchboard Python SDK

This example shows how to create an AI expert bot that connects as a student
and provides hints when instructors broadcast problems.

Usage:
    python python-student-bot.py --config expert-config.json
"""

import asyncio
import json
import sys
import argparse
from pathlib import Path

# Add the SDK to the path for this example
sys.path.insert(0, str(Path(__file__).parent.parent / "python"))

from switchboard_sdk import SwitchboardStudent


class AIExpertBot(SwitchboardStudent):
    """AI Expert Bot that provides hints for programming problems"""
    
    def __init__(self, config):
        super().__init__(config["user_id"])
        self.config = config
        self.expert_name = config["name"]
        self.expertise = config["expertise"]
        
        # Set up event handlers
        self.on_instructor_broadcast(self.handle_problem_broadcast)
        self.on_instructor_request(self.handle_instructor_request)
        self.on_connection(self.handle_connection_change)
        self.on_error(self.handle_error)
    
    async def handle_problem_broadcast(self, message):
        """Handle problem broadcasts from instructors"""
        if message.context == "problem":
            print(f"🎯 Processing problem: {message.content.get('problem', 'Unknown problem')[:50]}...")
            
            try:
                # Generate hint
                hint = await self.generate_hint(message.content)
                
                # Send hint back to instructors
                await self.ask_question({
                    "hint": hint,
                    "expert": {
                        "name": self.expert_name,
                        "expertise": self.expertise
                    },
                    "problem_context": {
                        "frustration_level": message.content.get("frustrationLevel", 0),
                        "time_on_task": message.content.get("timeOnTask", 0),
                        "remaining_time": message.content.get("remainingTime", 0)
                    },
                    "timestamp": message.timestamp.isoformat() if message.timestamp else None
                }, context="hint")
                
                print(f"✅ Sent hint to instructors ({len(hint)} chars)")
                
            except Exception as e:
                print(f"❌ Failed to generate hint: {e}")
                
                # Send error notification
                await self.ask_question({
                    "error": str(e),
                    "expert": {
                        "name": self.expert_name
                    },
                    "timestamp": message.timestamp.isoformat() if message.timestamp else None
                }, context="error")
    
    async def handle_instructor_request(self, message):
        """Handle direct requests from instructors"""
        print(f"📩 Request from {message.from_user}: {message.content.get('text', 'No text')}")
        
        # Respond with expert capabilities
        await self.respond_to_request({
            "text": f"I'm {self.expert_name}, specialized in {self.expertise}. How can I help?",
            "capabilities": [
                "Code analysis and debugging",
                "Learning path suggestions", 
                "Best practice recommendations",
                "Problem-solving hints"
            ]
        })
    
    async def handle_connection_change(self, connected):
        """Handle connection status changes"""
        if connected:
            print(f"🟢 {self.expert_name} connected to session")
            
            # Send connection analytics
            await self.send_analytics({
                "event": "expert_connected",
                "expert": {
                    "name": self.expert_name,
                    "expertise": self.expertise
                },
                "timestamp": self.get_status().uptime_seconds
            }, context="connection")
        else:
            print(f"🔴 {self.expert_name} disconnected")
    
    async def handle_error(self, error):
        """Handle errors"""
        print(f"❌ Error: {error}")
    
    async def generate_hint(self, problem_data):
        """Generate hint for the given problem (mock implementation)"""
        problem = problem_data.get("problem", "")
        code = problem_data.get("code", "")
        frustration_level = problem_data.get("frustrationLevel", 0)
        
        # In a real implementation, this would call an AI service like OpenAI, Anthropic, etc.
        hints_by_expertise = {
            "Python": [
                "Check your indentation - Python is sensitive to whitespace",
                "Consider using list comprehensions for cleaner code",
                "Remember to handle exceptions with try/except blocks",
                "Use descriptive variable names to make your code more readable"
            ],
            "JavaScript": [
                "Check for undefined variables - use const/let instead of var",
                "Remember that JavaScript is case-sensitive",
                "Use === for strict equality comparisons",
                "Check the browser console for error messages"
            ],
            "React": [
                "Make sure you're importing React components correctly",
                "Check that your state updates are immutable",
                "Use useEffect with proper dependencies",
                "Remember that props are read-only"
            ],
            "Debugging": [
                "Add console.log statements to track variable values",
                "Check for typos in function and variable names",
                "Verify that all brackets and parentheses are properly closed",
                "Use the debugger or breakpoints to step through your code"
            ]
        }
        
        # Select hints based on expertise
        available_hints = hints_by_expertise.get(self.expertise, [
            "Break the problem down into smaller steps",
            "Check your syntax carefully",
            "Consider edge cases in your solution",
            "Test your code with simple examples first"
        ])
        
        # Adjust hint based on frustration level
        if frustration_level >= 4:
            hint_prefix = "Take a deep breath! "
        elif frustration_level >= 3:
            hint_prefix = "Don't worry, this is challenging. "
        else:
            hint_prefix = ""
        
        # Select appropriate hint
        hint_index = min(frustration_level, len(available_hints) - 1)
        base_hint = available_hints[hint_index]
        
        # Add problem-specific context if available
        if "error" in problem.lower() or "bug" in problem.lower():
            context_hint = " Focus on error messages - they often point directly to the issue."
        elif "loop" in problem.lower():
            context_hint = " Check your loop conditions and make sure they will eventually terminate."
        elif "function" in problem.lower():
            context_hint = " Verify your function parameters and return statements."
        else:
            context_hint = ""
        
        return f"{hint_prefix}{base_hint}{context_hint}"


async def main():
    parser = argparse.ArgumentParser(description="AI Expert Bot for Switchboard")
    parser.add_argument("--config", required=True, help="Path to expert configuration JSON file")
    args = parser.parse_args()
    
    # Load configuration
    try:
        with open(args.config, 'r') as f:
            config = json.load(f)
    except FileNotFoundError:
        print(f"❌ Config file not found: {args.config}")
        return
    except json.JSONDecodeError as e:
        print(f"❌ Invalid JSON in config file: {e}")
        return
    
    # Validate configuration
    required_fields = ["user_id", "name", "expertise"]
    for field in required_fields:
        if field not in config:
            print(f"❌ Missing required config field: {field}")
            return
    
    print(f"🤖 Starting {config['name']} ({config['expertise']})")
    print(f"👤 User ID: {config['user_id']}")
    
    # Create and start the bot
    bot = AIExpertBot(config)
    
    try:
        # Connect to available session
        session = await bot.connect_to_available_session()
        
        if session:
            print(f"✅ Connected to session: {session.name}")
            print(f"📊 Session has {len(session.student_ids)} students")
            
            # Keep the bot running
            print("🎓 Expert bot is ready and waiting for problems!")
            print("Press Ctrl+C to stop")
            
            # Send initial connection analytics
            await bot.send_analytics({
                "event": "bot_started",
                "expert": {
                    "name": bot.expert_name,
                    "expertise": bot.expertise
                },
                "session_info": {
                    "session_id": session.id,
                    "session_name": session.name,
                    "student_count": len(session.student_ids)
                }
            }, context="lifecycle")
            
            # Keep running
            try:
                await asyncio.sleep(3600 * 24)  # Run for 24 hours
            except KeyboardInterrupt:
                print("\n🛑 Shutting down expert bot...")
            
        else:
            print("⚠️ No available sessions found")
            print("Make sure you're enrolled in an active session")
    
    except Exception as e:
        print(f"❌ Failed to start expert bot: {e}")
    
    finally:
        await bot.disconnect()
        print("👋 Expert bot stopped")


if __name__ == "__main__":
    asyncio.run(main())