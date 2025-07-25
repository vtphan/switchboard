"""
Type definitions for Switchboard SDK
"""

from dataclasses import dataclass
from typing import Optional, Dict, Any, List
from datetime import datetime
from enum import Enum


class MessageType(str, Enum):
    """Valid message types in Switchboard protocol"""
    INSTRUCTOR_INBOX = "instructor_inbox"
    INBOX_RESPONSE = "inbox_response" 
    REQUEST = "request"
    REQUEST_RESPONSE = "request_response"
    ANALYTICS = "analytics"
    INSTRUCTOR_BROADCAST = "instructor_broadcast"
    SYSTEM = "system"


class StudentRole(str, Enum):
    """Student role constant"""
    STUDENT = "student"


class TeacherRole(str, Enum): 
    """Teacher role constant"""
    INSTRUCTOR = "instructor"


@dataclass
class Session:
    """Session information"""
    id: str
    name: str
    created_by: str
    student_ids: List[str]
    start_time: datetime
    end_time: Optional[datetime] = None
    status: str = "active"
    connection_count: Optional[int] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'Session':
        """Create Session from API response dictionary"""
        return cls(
            id=data["id"],
            name=data["name"],
            created_by=data["created_by"],
            student_ids=data["student_ids"],
            start_time=datetime.fromisoformat(data["start_time"].replace("Z", "+00:00")),
            end_time=datetime.fromisoformat(data["end_time"].replace("Z", "+00:00")) if data.get("end_time") else None,
            status=data.get("status", "active"),
            connection_count=data.get("connection_count")
        )


@dataclass
class Message:
    """Message structure for Switchboard communication"""
    type: MessageType
    context: str
    content: Dict[str, Any]
    to_user: Optional[str] = None
    from_user: Optional[str] = None
    session_id: Optional[str] = None
    timestamp: Optional[datetime] = None
    id: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'Message':
        """Create Message from WebSocket message dictionary"""
        return cls(
            id=data.get("id"),
            type=MessageType(data["type"]),
            context=data.get("context", "general"),
            content=data["content"],
            to_user=data.get("to_user"),
            from_user=data.get("from_user"),
            session_id=data.get("session_id"),
            timestamp=datetime.fromisoformat(data["timestamp"].replace("Z", "+00:00")) if data.get("timestamp") else None
        )

    def to_dict(self) -> Dict[str, Any]:
        """Convert message to dictionary for WebSocket sending"""
        result = {
            "type": self.type.value,
            "context": self.context,
            "content": self.content
        }
        
        if self.to_user is not None:
            result["to_user"] = self.to_user
            
        return result


@dataclass
class ConnectionStatus:
    """Connection status information"""
    connected: bool
    session_id: Optional[str] = None
    user_id: Optional[str] = None
    role: Optional[str] = None
    last_heartbeat: Optional[datetime] = None
    message_count: int = 0
    uptime_seconds: int = 0