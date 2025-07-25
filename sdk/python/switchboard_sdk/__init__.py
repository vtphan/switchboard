"""
Switchboard Python SDK

A Python client library for connecting to Switchboard real-time messaging system.
Provides high-level abstractions for both student and instructor clients.
"""

from .client import SwitchboardClient
from .student import SwitchboardStudent  
from .teacher import SwitchboardTeacher
from .exceptions import (
    SwitchboardError,
    ConnectionError,
    AuthenticationError,
    SessionNotFoundError,
    MessageValidationError,
    RateLimitError
)
from .types import (
    Session,
    Message,
    MessageType,
    StudentRole,
    TeacherRole
)

__version__ = "1.0.0"
__all__ = [
    "SwitchboardClient",
    "SwitchboardStudent", 
    "SwitchboardTeacher",
    "SwitchboardError",
    "ConnectionError", 
    "AuthenticationError",
    "SessionNotFoundError",
    "MessageValidationError",
    "RateLimitError",
    "Session",
    "Message", 
    "MessageType",
    "StudentRole",
    "TeacherRole"
]