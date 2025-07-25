"""
Teacher client implementation for Switchboard
"""

from typing import List, Dict, Any, Optional
import aiohttp
from .client import SwitchboardClient
from .types import Session, Message, MessageType, TeacherRole
from .exceptions import SwitchboardError, SessionNotFoundError


class SwitchboardTeacher(SwitchboardClient):
    """
    Teacher client for Switchboard real-time messaging
    
    Teachers have universal access and can:
    - Create and manage sessions
    - Connect to any active session
    - Send: inbox_response, request, instructor_broadcast
    - Receive: instructor_inbox, request_response, analytics
    """
    
    def __init__(self, 
                 user_id: str,
                 server_url: str = "http://localhost:8080",
                 max_reconnect_attempts: int = 5,
                 reconnect_delay: float = 1.0):
        """
        Initialize Teacher client
        
        Args:
            user_id: Teacher's unique identifier
            server_url: Base URL for Switchboard server
            max_reconnect_attempts: Maximum reconnection attempts
            reconnect_delay: Initial delay between reconnection attempts
        """
        super().__init__(
            user_id=user_id,
            server_url=server_url,
            role=TeacherRole.INSTRUCTOR.value,
            max_reconnect_attempts=max_reconnect_attempts,
            reconnect_delay=reconnect_delay
        )

    # Session Management (Teacher-specific)
    
    async def create_session(self, 
                           name: str, 
                           student_ids: List[str]) -> Session:
        """
        Create a new session
        
        Args:
            name: Session name
            student_ids: List of student IDs to enroll
            
        Returns:
            Created Session object
            
        Raises:
            SwitchboardError: If session creation fails
        """
        try:
            async with aiohttp.ClientSession() as session:
                payload = {
                    "name": name,
                    "instructor_id": self.user_id,
                    "student_ids": student_ids
                }
                
                async with session.post(
                    f"{self.server_url}/api/sessions",
                    json=payload,
                    headers={"Content-Type": "application/json"}
                ) as response:
                    if response.status == 201:
                        data = await response.json()
                        return Session.from_dict(data["session"])
                    else:
                        error_text = await response.text()
                        raise SwitchboardError(f"Failed to create session: HTTP {response.status} - {error_text}")
                        
        except aiohttp.ClientError as e:
            raise SwitchboardError(f"Network error creating session: {e}")

    async def end_session(self, session_id: str) -> None:
        """
        End an active session
        
        Args:
            session_id: Session ID to end
            
        Raises:
            SessionNotFoundError: If session doesn't exist
            SwitchboardError: If ending session fails
        """
        try:
            async with aiohttp.ClientSession() as session:
                async with session.delete(f"{self.server_url}/api/sessions/{session_id}") as response:
                    if response.status == 404:
                        raise SessionNotFoundError(f"Session not found: {session_id}")
                    elif response.status != 200:
                        error_text = await response.text()
                        raise SwitchboardError(f"Failed to end session: HTTP {response.status} - {error_text}")
                        
        except aiohttp.ClientError as e:
            raise SwitchboardError(f"Network error ending session: {e}")

    async def list_active_sessions(self) -> List[Session]:
        """
        List all active sessions (teachers can see all sessions)
        
        Returns:
            List of all active Session objects
        """
        all_sessions = await self.discover_sessions()
        return [session for session in all_sessions if session.status == "active"]

    # Teacher Message Sending Methods
    
    async def respond_to_student(self, 
                               student_id: str,
                               content: Dict[str, Any], 
                               context: str = "answer") -> None:
        """
        Send a direct response to a specific student
        
        Args:
            student_id: Target student's user ID
            content: Response content dictionary
            context: Response context (e.g., "answer", "clarification", "feedback")
            
        Raises:
            SwitchboardError: If sending fails
        """
        message = Message(
            type=MessageType.INBOX_RESPONSE,
            context=context,
            content=content,
            to_user=student_id
        )
        
        await self.send_message(message)

    async def request_from_student(self, 
                                 student_id: str,
                                 content: Dict[str, Any], 
                                 context: str = "request") -> None:
        """
        Send a direct request to a specific student
        
        Args:
            student_id: Target student's user ID
            content: Request content dictionary
            context: Request context (e.g., "code", "explanation", "demo")
            
        Raises:
            SwitchboardError: If sending fails
        """
        message = Message(
            type=MessageType.REQUEST,
            context=context,
            content=content,
            to_user=student_id
        )
        
        await self.send_message(message)

    async def broadcast_to_students(self, 
                                  content: Dict[str, Any], 
                                  context: str = "announcement") -> None:
        """
        Send a broadcast message to all students in the session
        
        Args:
            content: Broadcast content dictionary
            context: Broadcast context (e.g., "announcement", "instruction", "emergency")
            
        Raises:
            SwitchboardError: If sending fails
        """
        message = Message(
            type=MessageType.INSTRUCTOR_BROADCAST,
            context=context,
            content=content
        )
        
        await self.send_message(message)

    # Convenience Methods for Common Teacher Actions
    
    async def announce(self, text: str, **kwargs) -> None:
        """
        Make an announcement to all students
        
        Args:
            text: Announcement text
            **kwargs: Additional content fields
        """
        content = {"text": text, **kwargs}
        await self.broadcast_to_students(content, context="announcement")

    async def give_instruction(self, instruction: str, **kwargs) -> None:
        """
        Give an instruction to all students
        
        Args:
            instruction: Instruction text
            **kwargs: Additional content fields
        """
        content = {"text": instruction, **kwargs}
        await self.broadcast_to_students(content, context="instruction")

    async def request_code_from_student(self, 
                                      student_id: str,
                                      prompt: str,
                                      requirements: List[str] = None,
                                      deadline: str = None) -> None:
        """
        Request code submission from a specific student
        
        Args:
            student_id: Target student's user ID
            prompt: Code request prompt
            requirements: List of specific requirements
            deadline: Deadline for submission
        """
        content = {
            "text": prompt,
            "requirements": requirements or [],
            "deadline": deadline
        }
        
        await self.request_from_student(student_id, content, context="code")

    async def provide_feedback(self, 
                             student_id: str,
                             feedback: str,
                             code_example: str = None,
                             resources: List[str] = None) -> None:
        """
        Provide feedback to a specific student
        
        Args:
            student_id: Target student's user ID
            feedback: Feedback text
            code_example: Example code if applicable
            resources: List of additional resources
        """
        content = {
            "text": feedback,
            "code_example": code_example,
            "additional_resources": resources or []
        }
        
        await self.respond_to_student(student_id, content, context="feedback")

    async def schedule_break(self, 
                           duration_minutes: int,
                           resume_time: str = None,
                           instructions: str = None) -> None:
        """
        Announce a break to all students
        
        Args:
            duration_minutes: Break duration in minutes
            resume_time: When to resume (e.g., "10:15 AM")
            instructions: Additional instructions for the break
        """
        content = {
            "text": f"We'll take a {duration_minutes}-minute break.",
            "break_duration": duration_minutes * 60,  # Convert to seconds
            "resume_time": resume_time,
            "instructions": instructions
        }
        
        await self.broadcast_to_students(content, context="break")

    async def broadcast_problem(self, 
                              problem: str,
                              code: str = "",
                              time_on_task: int = 0,
                              remaining_time: int = 30,
                              frustration_level: int = 2) -> None:
        """
        Broadcast a problem for AI experts or tutoring systems
        
        Args:
            problem: Problem description
            code: Code context
            time_on_task: Time spent on task in minutes
            remaining_time: Remaining time in minutes
            frustration_level: Frustration level (1-5)
        """
        content = {
            "problem": problem,
            "code": code,
            "timeOnTask": time_on_task,
            "remainingTime": remaining_time,
            "frustrationLevel": frustration_level
        }
        
        await self.broadcast_to_students(content, context="problem")

    # Session Flow Management
    
    async def create_and_connect(self, 
                               session_name: str, 
                               student_ids: List[str]) -> Session:
        """
        Create a session and immediately connect to it
        
        Args:
            session_name: Name for the new session
            student_ids: List of student IDs to enroll
            
        Returns:
            Created Session object
            
        Raises:
            SwitchboardError: If creation or connection fails
        """
        session = await self.create_session(session_name, student_ids)
        await self.connect(session.id)
        return session

    async def end_current_session(self) -> None:
        """
        End the currently connected session
        
        Raises:
            SwitchboardError: If no session connected or ending fails
        """
        if not self.current_session_id:
            raise SwitchboardError("No session currently connected")
            
        await self.end_session(self.current_session_id)
        await self.disconnect()

    # Event Handler Convenience Methods
    
    def on_student_question(self, handler):
        """Register handler for student questions"""
        self.on_message(MessageType.INSTRUCTOR_INBOX, handler)

    def on_student_response(self, handler):
        """Register handler for student responses"""
        self.on_message(MessageType.REQUEST_RESPONSE, handler)

    def on_student_analytics(self, handler):
        """Register handler for student analytics"""
        self.on_message(MessageType.ANALYTICS, handler)

    def on_system_message(self, handler):
        """Register handler for system messages"""
        self.on_message(MessageType.SYSTEM, handler)