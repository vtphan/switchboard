#!/usr/bin/env python3
"""
AI Expert Hint Agent using Switchboard Python SDK

This agent connects to Switchboard as a student and provides AI-generated hints
when instructors broadcast problems.
"""

import asyncio
import json
import sys
import signal
from pathlib import Path
from typing import Dict, Any, Optional, Union
import aiohttp
from dataclasses import dataclass

# Import Switchboard SDK
sys.path.append(str(Path(__file__).parent.parent.parent / "python"))
from switchboard_sdk import SwitchboardStudent
from switchboard_sdk.types import Message


@dataclass
class ExpertConfig:
    """Configuration for an expert agent"""
    user_id: str
    name: str
    expertise: str
    description: str
    response_style: str
    switchboard_url: str
    gemini_api_key: str
    gemini_model: str
    gemini_temperature: float
    gemini_max_output_tokens: int
    prompt_template: str


class HintAgent:
    """AI Expert Agent that provides hints using Gemini API"""
    
    def __init__(self, config_path: str):
        self.config_path = Path(config_path)
        self.config: Optional[ExpertConfig] = None
        self.client: Optional[SwitchboardStudent] = None
        self.session = None
        self.shutdown_requested = False
        
        # Statistics
        self.start_time = None
        self.message_count = 0
        self.hints_generated = 0
        
        self._load_config()
        self._setup_shutdown_handlers()
    
    def _load_config(self) -> None:
        """Load expert configuration from JSON file"""
        try:
            with open(self.config_path) as f:
                data = json.load(f)
            
            # Validate required fields
            required_fields = [
                'user_id', 'name', 'expertise', 'description', 'response_style',
                'switchboard_url', 'gemini_api_key', 'gemini_model', 
                'gemini_temperature', 'gemini_max_output_tokens', 'prompt_template'
            ]
            
            for field in required_fields:
                if field not in data:
                    raise ValueError(f"Missing required config field: {field}")
            
            if data['gemini_api_key'] == 'your_gemini_api_key_here':
                raise ValueError("Please set your actual Gemini API key in the config file")
            
            self.config = ExpertConfig(
                user_id=data['user_id'],
                name=data['name'],
                expertise=data['expertise'],
                description=data['description'],
                response_style=data['response_style'],
                switchboard_url=data['switchboard_url'],
                gemini_api_key=data['gemini_api_key'],
                gemini_model=data['gemini_model'],
                gemini_temperature=data['gemini_temperature'],
                gemini_max_output_tokens=data['gemini_max_output_tokens'],
                prompt_template=data['prompt_template']
            )
            
            print(f"🤖 {self.config.name} initialized")
            print(f"   User ID: {self.config.user_id}")
            print(f"   Expertise: {self.config.expertise}")
            print(f"   Switchboard: {self.config.switchboard_url}")
            
        except Exception as e:
            print(f"❌ Failed to load config: {e}")
            sys.exit(1)
    
    def _setup_shutdown_handlers(self) -> None:
        """Setup graceful shutdown handlers"""
        def handle_shutdown(signum, frame):
            print(f"\n🛑 Received signal {signum}, shutting down {self.config.name if self.config else 'agent'}...")
            self.shutdown_requested = True
        
        signal.signal(signal.SIGINT, handle_shutdown)
        signal.signal(signal.SIGTERM, handle_shutdown)
    
    def _get_timestamp(self) -> str:
        """Get current timestamp in ISO format"""
        from datetime import datetime
        return datetime.now().isoformat()
    
    async def start(self) -> None:
        """Start the hint agent"""
        print(f"🚀 Starting {self.config.name}...")
        
        try:
            # Create SDK client
            self.client = SwitchboardStudent(self.config.user_id, self.config.switchboard_url)
            
            # Set up event handlers
            self._setup_event_handlers()
            
            # Connect to available session
            self.session = await self.client.connect_to_available_session()
            
            if not self.session:
                print("⚠️ No available sessions found. Waiting for sessions...")
                await self._wait_for_sessions()
            else:
                print(f"✅ Connected to session: {self.session.name} ({self.session.id})")
            
            # Send connection analytics
            await self._send_connection_analytics('connected')
            
            print(f"✅ {self.config.name} ready and waiting for problems!")
            
            # Keep running until shutdown
            await self._keep_alive()
            
        except Exception as e:
            print(f"❌ Failed to start agent: {e}")
            sys.exit(1)
    
    def _setup_event_handlers(self) -> None:
        """Setup event handlers for the Switchboard client"""
        
        @self.client.on_instructor_broadcast
        async def handle_broadcast(message: Message) -> None:
            """Handle instructor broadcasts"""
            try:
                self.message_count += 1
                print(f"📨 Received broadcast: {message.type}")
                
                if message.context == 'problem':
                    await self._handle_problem_broadcast(message.content)
                elif message.context == 'status':
                    await self._handle_status_update(message.content)
                    
            except Exception as e:
                print(f"❌ Error handling broadcast: {e}")
        
        @self.client.on_instructor_response
        async def handle_response(message: Message) -> None:
            """Handle instructor responses"""
            from_user = getattr(message, 'from_user', 'instructor')
            text = message.content.get('text', 'Message received')
            print(f"💬 Response from {from_user}: {text}")
        
        @self.client.on_connection
        async def handle_connection(connected: bool) -> None:
            """Handle connection changes"""
            if connected:
                print("🟢 Connected to session")
            else:
                print("🔴 Disconnected from session")
                if not self.shutdown_requested:
                    # Try to send analytics, but don't crash if it fails
                    try:
                        await self._send_connection_analytics('disconnected')
                    except Exception as e:
                        print(f"⚠️ Could not send disconnection analytics: {e}")
                    
                    # Wait for new sessions to become available
                    print("⏳ Waiting for new sessions...")
                    await self._wait_for_sessions()
        
        @self.client.on_error
        async def handle_error(error: Exception) -> None:
            """Handle connection errors"""
            print(f"❌ Connection error: {error}")
        
        @self.client.on_system_message
        async def handle_system_message(message: Message) -> None:
            """Handle system messages from the server"""
            try:
                print(f"🔍 DEBUG: Received system message: {message}")
                print(f"🔍 DEBUG: message.type: {message.type}")
                print(f"🔍 DEBUG: message.context: {message.context}")
                print(f"🔍 DEBUG: message.content: {message.content}")
                
                # Check both Content.event and Context field for session_ended
                event = None
                if isinstance(message.content, dict) and "event" in message.content:
                    event = message.content["event"]
                elif message.context:
                    event = message.context
                
                print(f"🔍 DEBUG: extracted event: {event}")
                
                if event == "session_ended":
                    reason = message.content.get("reason", "Unknown reason") if isinstance(message.content, dict) else "Unknown reason"
                    print(f"🛑 Session ended by server: {reason}")
                    print(f"⏳ {self.config.name} will wait for the next session...")
                    # Don't set shutdown_requested = True, let the client handle disconnection
                    # The agent will continue running and wait for new sessions
                    
                elif event == "history_complete":
                    print("📚 Message history loaded")
                    
                else:
                    print(f"ℹ️ System message: {event}")
                    
            except Exception as e:
                print(f"❌ Error handling system message: {e}")
    
    async def _handle_problem_broadcast(self, problem_data: Dict[str, Any]) -> None:
        """Handle problem broadcast and generate hint"""
        problem = problem_data.get('problem', '')
        code = problem_data.get('code', '')
        frustration_level = problem_data.get('frustrationLevel', 1)
        time_on_task = problem_data.get('timeOnTask', 0)
        remaining_time = problem_data.get('remainingTime', 30)
        
        print(f"🎯 Processing problem: \"{problem[:50]}...\"")
        print(f"   Time on task: {time_on_task}min, Remaining: {remaining_time}min")
        print(f"   Frustration level: {frustration_level}/5")
        
        try:
            # Generate hint using Gemini
            hint = await self._generate_hint(problem_data)
            
            # Send hint back to instructors
            await self._send_hint(hint, problem_data)
            
            self.hints_generated += 1
            
        except Exception as e:
            print(f"❌ Failed to generate hint: {e}")
            await self._send_error_notification(problem_data, e)
    
    async def _handle_status_update(self, status_data: Dict[str, Any]) -> None:
        """Handle status updates"""
        message = status_data.get('message', 'Status update')
        print(f"📢 Status update: {message}")
    
    async def _generate_hint(self, problem_data: Dict[str, Any]) -> str:
        """Generate hint using Gemini API"""
        print("🧠 Generating hint with Gemini...")
        
        # Build prompt using template from config
        prompt = self._build_prompt(problem_data)
        
        print(f"📝 Using model: {self.config.gemini_model}")
        print(f"📊 Frustration level: {problem_data.get('frustrationLevel', 1)}/5")
        
        # Call Gemini API
        api_url = f"https://generativelanguage.googleapis.com/v1beta/models/{self.config.gemini_model}:generateContent"
        
        headers = {
            'Content-Type': 'application/json',
            'x-goog-api-key': self.config.gemini_api_key
        }
        
        payload = {
            'contents': [{
                'parts': [{'text': prompt}]
            }],
            'generationConfig': {
                'maxOutputTokens': self.config.gemini_max_output_tokens,
                'temperature': self.config.gemini_temperature
            }
        }
        
        async with aiohttp.ClientSession() as session:
            async with session.post(api_url, headers=headers, json=payload) as response:
                if not response.ok:
                    error_text = await response.text()
                    raise Exception(f"Gemini API error: {response.status} {error_text}")
                
                result = await response.json()
        
        # Extract hint from response
        if not result.get('candidates', [{}])[0].get('content', {}).get('parts', [{}])[0].get('text'):
            raise Exception('Invalid Gemini API response format')
        
        hint = result['candidates'][0]['content']['parts'][0]['text'].strip()
        
        print(f"💡 Generated hint ({len(hint)} chars)")
        return hint
    
    def _build_prompt(self, problem_data: Dict[str, Any]) -> str:
        """Build prompt using template from configuration"""
        problem = problem_data.get('problem', '')
        code = problem_data.get('code', '')
        frustration_level = problem_data.get('frustrationLevel', 1)
        time_on_task = problem_data.get('timeOnTask', 0)
        
        # Prepare code context section
        code_context = f"Code context:\n{code}\n\n" if code else ""
        
        # Use the prompt template from config with variable substitution
        prompt = self.config.prompt_template.format(
            name=self.config.name,
            expertise=self.config.expertise,
            response_style=self.config.response_style,
            problem=problem,
            code_context=code_context,
            time_on_task=time_on_task,
            frustration_level=frustration_level
        )
        
        return prompt
    
    async def _send_hint(self, hint: str, problem_data: Dict[str, Any]) -> None:
        """Send hint to instructors via instructor_inbox"""
        if not self.client or not self.client.connected:
            print("⚠️ Not connected, cannot send hint")
            return
        
        hint_content = {
            'hint': hint,
            'expert': {
                'name': self.config.name,
                'user_id': self.config.user_id,
                'expertise': self.config.expertise
            },
            'problemContext': {
                'frustrationLevel': problem_data.get('frustrationLevel', 1),
                'timeOnTask': problem_data.get('timeOnTask', 0),
                'remainingTime': problem_data.get('remainingTime', 30)
            },
            'timestamp': self._get_timestamp()
        }
        
        await self.client.ask_question(hint_content, 'hint')
        print(f"✅ Sent hint to instructors ({len(hint)} chars)")
    
    async def _send_error_notification(self, problem_data: Dict[str, Any], error: Exception) -> None:
        """Send error notification to instructors"""
        if not self.client or not self.client.connected:
            return
        
        # Categorize error
        error_message = "Unknown error occurred"
        if "401" in str(error) or "403" in str(error):
            error_message = "Invalid or missing Gemini API key"
        elif "429" in str(error):
            error_message = "Gemini API rate limit exceeded"
        elif "500" in str(error) or "503" in str(error):
            error_message = "Gemini API service unavailable"
        else:
            error_message = str(error)
        
        error_content = {
            'error': error_message,
            'expert': {
                'name': self.config.name,
                'user_id': self.config.user_id
            },
            'timestamp': self._get_timestamp()
        }
        
        await self.client.ask_question(error_content, 'error')
        print(f"❌ Sent error notification: {error_message}")
    
    async def _send_connection_analytics(self, event: str) -> None:
        """Send connection analytics"""
        if not self.client:
            return
        
        analytics_content = {
            'event': event,
            'expert': {
                'name': self.config.name,
                'user_id': self.config.user_id,
                'expertise': self.config.expertise
            },
            'uptime': (asyncio.get_event_loop().time() - (self.start_time or 0)),
            'messageCount': self.message_count,
            'hintsGenerated': self.hints_generated
        }
        
        await self.client.send_analytics(analytics_content, 'connection')
    
    async def _wait_for_sessions(self) -> None:
        """Wait for available sessions"""
        while not self.shutdown_requested:
            await asyncio.sleep(30)  # Poll every 30 seconds
            
            try:
                self.session = await self.client.connect_to_available_session()
                if self.session:
                    print(f"✅ Connected to session: {self.session.name} ({self.session.id})")
                    await self._send_connection_analytics('connected')
                    break
            except Exception as e:
                print(f"❌ Session polling error: {e}")
    
    async def _keep_alive(self) -> None:
        """Keep the agent running"""
        self.start_time = asyncio.get_event_loop().time()
        
        while not self.shutdown_requested:
            await asyncio.sleep(1)
        
        await self._shutdown()
    
    async def _shutdown(self) -> None:
        """Graceful shutdown"""
        print(f"🛑 Stopping {self.config.name}...")
        
        if self.client and self.client.connected:
            await self._send_connection_analytics('disconnected')
            await self.client.disconnect()
        
        print(f"✅ {self.config.name} stopped")
        print(f"   Messages processed: {self.message_count}")
        print(f"   Hints generated: {self.hints_generated}")


async def main():
    """Main entry point"""
    if len(sys.argv) != 2:
        print("Usage: python hint_agent.py <config_file>")
        print("Example: python hint_agent.py configs/technical-expert.json")
        sys.exit(1)
    
    config_path = sys.argv[1]
    
    if not Path(config_path).exists():
        print(f"❌ Config file not found: {config_path}")
        sys.exit(1)
    
    print("🎓 AI Programming Mentorship - Hint Agent")
    print("=" * 50)
    print(f"Config: {config_path}")
    print("=" * 50)
    
    agent = HintAgent(config_path)
    
    try:
        await agent.start()
    except Exception as e:
        print(f"💥 Fatal error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main())