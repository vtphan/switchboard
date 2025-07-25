"""
Exception classes for Switchboard SDK
"""


class SwitchboardError(Exception):
    """Base exception for all Switchboard SDK errors"""
    def __init__(self, message: str, details: dict = None):
        super().__init__(message)
        self.message = message
        self.details = details or {}


class ConnectionError(SwitchboardError):
    """Raised when WebSocket connection fails"""
    pass


class AuthenticationError(SwitchboardError):
    """Raised when authentication fails (e.g., not enrolled in session)"""
    pass


class SessionNotFoundError(SwitchboardError):
    """Raised when attempting to access non-existent session"""
    pass


class MessageValidationError(SwitchboardError):
    """Raised when message validation fails"""
    pass


class RateLimitError(SwitchboardError):
    """Raised when rate limit is exceeded"""
    pass


class SessionEndedError(SwitchboardError):
    """Raised when session is ended while connected"""
    pass


class ReconnectionFailedError(SwitchboardError):
    """Raised when automatic reconnection fails after max attempts"""
    pass