"""
Student client implementation for Switchboard
"""

from typing import List, Dict, Any, Optional
from .client import SwitchboardClient
from .types import Session, Message, MessageType, StudentRole
from .exceptions import SwitchboardError


class SwitchboardStudent(SwitchboardClient):
    """
    Student client for Switchboard real-time messaging
    
    Students have restricted access and can only:
    - Connect to sessions where they are enrolled
    - Send: instructor_inbox, request_response, analytics
    - Receive: inbox_response, request, instructor_broadcast
    """
    
    def __init__(self, 
                 user_id: str,
                 server_url: str = "http://localhost:8080",
                 max_reconnect_attempts: int = 5,
                 reconnect_delay: float = 1.0):
        """
        Initialize Student client
        
        Args:
            user_id: Student's unique identifier
            server_url: Base URL for Switchboard server
            max_reconnect_attempts: Maximum reconnection attempts
            reconnect_delay: Initial delay between reconnection attempts
        """
        super().__init__(
            user_id=user_id,
            server_url=server_url,
            role=StudentRole.STUDENT.value,
            max_reconnect_attempts=max_reconnect_attempts,
            reconnect_delay=reconnect_delay
        )

    # Session Discovery (Student-specific)
    
    async def find_available_sessions(self) -> List[Session]:
        """
        Find sessions where this student is enrolled
        
        Returns:
            List of sessions this student can join
        """
        all_sessions = await self.discover_sessions()
        
        # Filter to sessions where student is enrolled and active
        available_sessions = [
            session for session in all_sessions
            if self.user_id in session.student_ids and session.status == "active"
        ]
        
        return available_sessions

    async def connect_to_available_session(self) -> Optional[Session]:
        """
        Automatically connect to first available session
        
        Returns:
            Session object if connection successful, None if no sessions available
            
        Raises:
            SwitchboardError: If connection fails
        """
        available_sessions = await self.find_available_sessions()
        
        if not available_sessions:
            return None
            
        session = available_sessions[0]
        await self.connect(session.id)
        return session

    # Student Message Sending Methods
    
    async def ask_question(self, 
                          content: Dict[str, Any], 
                          context: str = "question") -> None:
        """
        Send a question to all instructors in the session
        
        Args:
            content: Question content dictionary
            context: Message context (e.g., "question", "help", "clarification")
            
        Raises:
            SwitchboardError: If sending fails
        """
        message = Message(
            type=MessageType.INSTRUCTOR_INBOX,
            context=context,
            content=content
        )
        
        await self.send_message(message)

    async def respond_to_request(self, 
                                content: Dict[str, Any], 
                                context: str = "response") -> None:
        """
        Respond to an instructor request
        
        Args:
            content: Response content dictionary
            context: Response context (should match original request context)
            
        Raises:
            SwitchboardError: If sending fails
        """
        message = Message(
            type=MessageType.REQUEST_RESPONSE,
            context=context,
            content=content
        )
        
        await self.send_message(message)

    async def send_analytics(self, 
                           content: Dict[str, Any], 
                           context: str = "progress") -> None:
        """
        Send analytics data to instructors
        
        Args:
            content: Analytics data dictionary
            context: Analytics type (e.g., "progress", "engagement", "performance", "error")
            
        Raises:
            SwitchboardError: If sending fails
        """
        message = Message(
            type=MessageType.ANALYTICS,
            context=context,
            content=content
        )
        
        await self.send_message(message)

    # Convenience Methods for Common Student Actions
    
    async def report_progress(self, 
                            completion_percentage: int,
                            time_spent_minutes: int,
                            current_topic: str,
                            exercises_completed: int = 0,
                            exercises_total: int = 0) -> None:
        """
        Report learning progress to instructors
        
        Args:
            completion_percentage: Percentage of work completed (0-100)
            time_spent_minutes: Time spent on current task
            current_topic: Current learning topic
            exercises_completed: Number of exercises completed
            exercises_total: Total number of exercises
        """
        content = {
            "completion_percentage": completion_percentage,
            "time_spent_minutes": time_spent_minutes,
            "current_topic": current_topic,
            "exercises_completed": exercises_completed,
            "exercises_total": exercises_total
        }
        
        await self.send_analytics(content, context="progress")

    async def report_engagement(self, 
                              attention_level: str,
                              confusion_level: str,
                              participation_score: int) -> None:
        """
        Report engagement metrics to instructors
        
        Args:
            attention_level: Current attention level ("high", "medium", "low")
            confusion_level: Current confusion level ("high", "medium", "low")
            participation_score: Participation score (0-100)
        """
        content = {
            "attention_level": attention_level,
            "confusion_level": confusion_level,
            "participation_score": participation_score,
            "last_interaction": self.get_status().uptime_seconds
        }
        
        await self.send_analytics(content, context="engagement")

    async def report_error(self, 
                          error_type: str,
                          error_message: str,
                          code_context: str = "",
                          attempted_fixes: int = 0,
                          time_stuck_minutes: int = 0) -> None:
        """
        Report an error or problem to instructors
        
        Args:
            error_type: Type of error (e.g., "syntax_error", "runtime_error", "logic_error")
            error_message: Error message or description
            code_context: Code snippet where error occurred
            attempted_fixes: Number of fix attempts made
            time_stuck_minutes: How long stuck on this error
        """
        content = {
            "error_type": error_type,
            "error_message": error_message,
            "code_context": code_context,
            "attempted_fixes": attempted_fixes,
            "time_stuck_minutes": time_stuck_minutes
        }
        
        await self.send_analytics(content, context="error")

    async def request_help(self, 
                          topic: str,
                          description: str,
                          urgency: str = "medium",
                          code_context: str = "") -> None:
        """
        Request help from instructors
        
        Args:
            topic: Help topic or subject
            description: Detailed description of what help is needed
            urgency: Urgency level ("low", "medium", "high")
            code_context: Related code context if applicable
        """
        content = {
            "text": description,
            "topic": topic,
            "urgency": urgency,
            "code_context": code_context
        }
        
        await self.ask_question(content, context="help")

    # Event Handler Convenience Methods
    
    def on_instructor_response(self, handler):
        """Register handler for instructor responses"""
        self.on_message(MessageType.INBOX_RESPONSE, handler)

    def on_instructor_request(self, handler):
        """Register handler for instructor requests"""
        self.on_message(MessageType.REQUEST, handler)

    def on_instructor_broadcast(self, handler):
        """Register handler for instructor broadcasts"""
        self.on_message(MessageType.INSTRUCTOR_BROADCAST, handler)

    def on_system_message(self, handler):
        """Register handler for system messages"""
        self.on_message(MessageType.SYSTEM, handler)