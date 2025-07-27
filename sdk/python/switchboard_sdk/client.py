"""
Base Switchboard client implementation
"""

import asyncio
import json
import logging
import time
from typing import Optional, Dict, Any, List, Callable, Awaitable
from urllib.parse import urlencode
import aiohttp
import websockets
from websockets.exceptions import ConnectionClosed, ConnectionClosedError, ConnectionClosedOK

from .types import Session, Message, MessageType, ConnectionStatus
from .exceptions import (
    SwitchboardError,
    ConnectionError, 
    AuthenticationError,
    SessionNotFoundError,
    SessionEndedError,
    ReconnectionFailedError
)


logger = logging.getLogger(__name__)


class SwitchboardClient:
    """Base client for Switchboard real-time messaging system"""
    
    def __init__(self, 
                 user_id: str,
                 server_url: str = "http://localhost:8080",
                 role: str = None,
                 max_reconnect_attempts: int = 5,
                 reconnect_delay: float = 1.0):
        """
        Initialize Switchboard client
        
        Args:
            user_id: Unique identifier for this user
            server_url: Base URL for Switchboard server (without ws://)
            role: User role (student/instructor) - will be set by subclasses
            max_reconnect_attempts: Maximum reconnection attempts
            reconnect_delay: Initial delay between reconnection attempts (exponential backoff)
        """
        self.user_id = user_id
        self.server_url = server_url.rstrip('/')
        self.role = role
        self.max_reconnect_attempts = max_reconnect_attempts
        self.reconnect_delay = reconnect_delay
        
        # Connection state
        self.websocket: Optional[websockets.WebSocketServerProtocol] = None
        self.current_session_id: Optional[str] = None
        self.connected = False
        self.connection_start_time: Optional[float] = None
        self.message_count = 0
        self.reconnect_attempts = 0
        
        # Event handlers
        self.message_handlers: Dict[MessageType, List[Callable[[Message], Awaitable[None]]]] = {}
        self.connection_handlers: List[Callable[[bool], Awaitable[None]]] = []
        self.error_handlers: List[Callable[[Exception], Awaitable[None]]] = []
        
        # Internal control
        self._receive_task: Optional[asyncio.Task] = None
        self._reconnect_task: Optional[asyncio.Task] = None
        self._shutdown = False

    # HTTP API Methods
    
    async def discover_sessions(self) -> List[Session]:
        """
        Discover available sessions from the server
        
        Returns:
            List of Session objects
            
        Raises:
            SwitchboardError: If API request fails
        """
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(f"{self.server_url}/api/sessions") as response:
                    if response.status != 200:
                        raise SwitchboardError(f"Failed to fetch sessions: HTTP {response.status}")
                    
                    data = await response.json()
                    return [Session.from_dict(session_data) for session_data in data["sessions"]]
                    
        except aiohttp.ClientError as e:
            raise SwitchboardError(f"Network error fetching sessions: {e}")

    async def get_session(self, session_id: str) -> Session:
        """
        Get detailed information about a specific session
        
        Args:
            session_id: Session ID to retrieve
            
        Returns:
            Session object with details
            
        Raises:
            SessionNotFoundError: If session doesn't exist
            SwitchboardError: If API request fails
        """
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(f"{self.server_url}/api/sessions/{session_id}") as response:
                    if response.status == 404:
                        raise SessionNotFoundError(f"Session not found: {session_id}")
                    elif response.status != 200:
                        raise SwitchboardError(f"Failed to fetch session: HTTP {response.status}")
                    
                    data = await response.json()
                    session_data = data["session"]
                    session_data["connection_count"] = data.get("connection_count")
                    return Session.from_dict(session_data)
                    
        except aiohttp.ClientError as e:
            raise SwitchboardError(f"Network error fetching session: {e}")

    # WebSocket Connection Management
    
    async def connect(self, session_id: str) -> None:
        """
        Connect to a Switchboard session
        
        Args:
            session_id: Session ID to connect to
            
        Raises:
            AuthenticationError: If not authorized for session
            ConnectionError: If WebSocket connection fails
            SwitchboardError: For other connection errors
        """
        if self.role is None:
            raise SwitchboardError("Role must be set before connecting")
            
        self.current_session_id = session_id
        self._shutdown = False
        
        await self._establish_connection()
    
    async def connect_to_lobby(self) -> None:
        """
        Connect to lobby without joining any specific session
        Users will receive real-time notifications about sessions and presence
        """
        logger.info("🏛️ Connecting to lobby...")
        await self.connect("lobby")

    async def _establish_connection(self) -> None:
        """Internal method to establish WebSocket connection"""
        if not self.current_session_id:
            raise SwitchboardError("No session ID set")
            
        # Build WebSocket URL
        ws_base = self.server_url.replace("http://", "ws://").replace("https://", "wss://")
        params = {
            "user_id": self.user_id,
            "role": self.role,
            "session_id": self.current_session_id
        }
        ws_url = f"{ws_base}/ws?{urlencode(params)}"
        
        try:
            logger.info(f"Connecting to {ws_url}")
            self.websocket = await websockets.connect(ws_url)
            
            self.connected = True
            self.connection_start_time = time.time()
            self.reconnect_attempts = 0
            self.message_count = 0
            
            # Start message receiving task
            self._receive_task = asyncio.create_task(self._receive_messages())
            
            # Notify connection handlers
            await self._notify_connection_handlers(True)
            
            logger.info(f"Connected to session {self.current_session_id}")
            
        except ConnectionClosedError as e:
            if e.code == 403:
                raise AuthenticationError("Not authorized for this session")
            elif e.code == 404:
                raise SessionNotFoundError("Session not found or ended")
            else:
                raise ConnectionError(f"WebSocket connection failed: {e}")
        except Exception as e:
            raise ConnectionError(f"Failed to connect: {e}")

    async def disconnect(self) -> None:
        """Gracefully disconnect from the session"""
        logger.info("🔍 DISCONNECT() CALLED - shutting down connection")
        self._shutdown = True
        
        # Cancel reconnection attempts
        if self._reconnect_task and not self._reconnect_task.done():
            self._reconnect_task.cancel()
            
        # Cancel message receiving
        if self._receive_task and not self._receive_task.done():
            self._receive_task.cancel()
            
        # Close WebSocket
        if self.websocket:
            await self.websocket.close()
            
        self.connected = False
        self.current_session_id = None
        
        # Notify connection handlers
        await self._notify_connection_handlers(False)
        
        logger.info("Disconnected from Switchboard")

    async def _receive_messages(self) -> None:
        """Internal task to receive and process WebSocket messages"""
        logger.info("🔍 STARTING MESSAGE RECEIVE LOOP")
        try:
            async for raw_message in self.websocket:
                logger.info("🔍 ENTERING MESSAGE PROCESSING LOOP")
                try:
                    logger.info(f"🔍 RAW MESSAGE RECEIVED: {raw_message}")
                    data = json.loads(raw_message)
                    logger.info(f"🔍 PARSED MESSAGE DATA: {data}")
                    message = Message.from_dict(data)
                    logger.info(f"🔍 MESSAGE OBJECT: type={message.type}, context={message.context}, content={message.content}")
                    
                    self.message_count += 1
                    
                    # Handle system messages
                    if message.type == MessageType.SYSTEM:
                        logger.info(f"🔍 PROCESSING SYSTEM MESSAGE: {message}")
                        await self._handle_system_message(message)
                    else:
                        logger.info(f"🔍 NON-SYSTEM MESSAGE: type={message.type}")
                    
                    # Notify message handlers
                    await self._notify_message_handlers(message)
                    
                except json.JSONDecodeError as e:
                    logger.error(f"❌ JSON parsing failed: {e}")
                    logger.error(f"❌ Raw message was: {raw_message}")
                    await self._notify_error_handlers(SwitchboardError(f"Invalid message format: {e}"))
                except Exception as parse_error:
                    logger.error(f"❌ Message parsing error: {parse_error}")
                    logger.error(f"❌ Raw message was: {raw_message}")
                    logger.error(f"❌ Data was: {data if 'data' in locals() else 'N/A'}")
                    await self._notify_error_handlers(SwitchboardError(f"Message processing failed: {parse_error}"))
                    
        except ConnectionClosed:
            logger.info("WebSocket connection closed")
            self.connected = False
            
            if not self._shutdown:
                # Connection lost unexpectedly, attempt reconnection
                await self._attempt_reconnection()
                
        except Exception as e:
            logger.error(f"Error in message receiving: {e}")
            self.connected = False
            await self._notify_error_handlers(e)

    async def _handle_system_message(self, message: Message) -> None:
        """Handle system messages from the server"""
        logger.info(f"🔍 DEBUG: _handle_system_message called with message: {message}")
        logger.info(f"🔍 DEBUG: message.type: {message.type}")
        logger.info(f"🔍 DEBUG: message.context: {message.context}")
        logger.info(f"🔍 DEBUG: message.content: {message.content}")
        
        # Check both Content.event and Context field for backwards compatibility
        event = None
        if isinstance(message.content, dict) and "event" in message.content:
            event = message.content["event"]
        elif message.context:
            event = message.context
        
        logger.info(f"🔍 DEBUG: extracted event: {event}")
        
        # LOBBY SYSTEM: Handle new message types
        if event == "session_left":
            reason = message.content.get("reason", "Unknown reason") if isinstance(message.content, dict) else "Unknown reason"
            session_id = message.content.get("session_id", "unknown") if isinstance(message.content, dict) else "unknown"
            logger.info(f"🔄 DEBUG: Left session {session_id}, reason: {reason}")
            
            # Don't disconnect! Return to lobby state
            self.current_session_id = None
            await self._notify_message_handlers(message)
            logger.info("🔄 DEBUG: Returned to lobby state")
            return
            
        elif event == "session_started":
            session_data = message.content if isinstance(message.content, dict) else {}
            session_id = session_data.get("session_id", "unknown")
            logger.info(f"🚀 DEBUG: Session started notification: {session_id}")
            
            # Check if we should auto-join this session
            if await self._should_join_session(session_data):
                await self._join_session(session_id)
            
            await self._notify_message_handlers(message)
            return
            
        elif event == "presence_update":
            # Handle unified presence updates (replaces user_connected/disconnected/connection_replaced)
            user_id = message.content.get("user_id", "unknown") if isinstance(message.content, dict) else "unknown"
            session_id = message.content.get("session_id") if isinstance(message.content, dict) else None
            role = message.content.get("role", "unknown") if isinstance(message.content, dict) else "unknown"
            
            if session_id is None:
                logger.debug(f"👥 DEBUG: User {user_id} disconnected")
            elif session_id == "lobby":
                logger.debug(f"👥 DEBUG: User {user_id} ({role}) in lobby")
            else:
                logger.debug(f"👥 DEBUG: User {user_id} ({role}) joined session {session_id}")
            
            await self._notify_message_handlers(message)
            return
            
        elif event == "history_complete":
            logger.debug("Message history loaded")
            
        elif event == "message_error":
            error_msg = message.content.get("message", "Unknown message error")
            await self._notify_error_handlers(SwitchboardError(f"Server message error: {error_msg}"))

    async def _attempt_reconnection(self) -> None:
        """Attempt to reconnect with exponential backoff"""
        if self._shutdown or self.reconnect_attempts >= self.max_reconnect_attempts:
            await self._notify_error_handlers(
                ReconnectionFailedError(f"Failed to reconnect after {self.max_reconnect_attempts} attempts")
            )
            return
            
        self.reconnect_attempts += 1
        delay = self.reconnect_delay * (2 ** (self.reconnect_attempts - 1))
        
        logger.info(f"Attempting reconnection {self.reconnect_attempts}/{self.max_reconnect_attempts} in {delay}s")
        
        self._reconnect_task = asyncio.create_task(self._delayed_reconnect(delay))

    async def _delayed_reconnect(self, delay: float) -> None:
        """Perform delayed reconnection"""
        try:
            await asyncio.sleep(delay)
            if not self._shutdown:
                await self._establish_connection()
        except Exception as e:
            logger.error(f"Reconnection attempt failed: {e}")
            await self._attempt_reconnection()

    # Message Sending
    
    async def send_message(self, message: Message) -> None:
        """
        Send a message to the session
        
        Args:
            message: Message object to send
            
        Raises:
            ConnectionError: If not connected
            SwitchboardError: If send fails
        """
        if not self.connected or not self.websocket:
            raise ConnectionError("Not connected to session")
            
        try:
            message_data = message.to_dict()
            await self.websocket.send(json.dumps(message_data))
            logger.debug(f"Sent {message.type.value} message")
            
        except Exception as e:
            raise SwitchboardError(f"Failed to send message: {e}")

    # Event Handler Registration
    
    def on_message(self, message_type: MessageType, handler: Callable[[Message], Awaitable[None]]) -> None:
        """Register handler for specific message type"""
        if message_type not in self.message_handlers:
            self.message_handlers[message_type] = []
        self.message_handlers[message_type].append(handler)

    def on_connection(self, handler: Callable[[bool], Awaitable[None]]) -> None:
        """Register handler for connection state changes"""
        self.connection_handlers.append(handler)

    def on_error(self, handler: Callable[[Exception], Awaitable[None]]) -> None:
        """Register handler for errors"""
        self.error_handlers.append(handler)

    # Internal event notification
    
    async def _notify_message_handlers(self, message: Message) -> None:
        """Notify all registered message handlers"""
        if message.type in self.message_handlers:
            for handler in self.message_handlers[message.type]:
                try:
                    await handler(message)
                except Exception as e:
                    logger.error(f"Error in message handler: {e}")

    async def _notify_connection_handlers(self, connected: bool) -> None:
        """Notify all registered connection handlers"""
        for handler in self.connection_handlers:
            try:
                await handler(connected)
            except Exception as e:
                logger.error(f"Error in connection handler: {e}")

    async def _notify_error_handlers(self, error: Exception) -> None:
        """Notify all registered error handlers"""
        for handler in self.error_handlers:
            try:
                await handler(error)
            except Exception as e:
                logger.error(f"Error in error handler: {e}")

    # Status and Info
    
    def get_status(self) -> ConnectionStatus:
        """Get current connection status"""
        uptime = int(time.time() - self.connection_start_time) if self.connection_start_time else 0
        
        return ConnectionStatus(
            connected=self.connected,
            session_id=self.current_session_id,
            user_id=self.user_id,
            role=self.role,
            message_count=self.message_count,
            uptime_seconds=uptime
        )

    @property
    def is_connected(self) -> bool:
        """Check if currently connected"""
        return self.connected
        
    # LOBBY SYSTEM: Helper methods for session management
    
    def is_in_lobby(self) -> bool:
        """Check if currently in lobby state"""
        return self.connected and (self.current_session_id is None or self.current_session_id == "lobby")
    
    async def _should_join_session(self, session_data: dict) -> bool:
        """
        Override in subclasses to determine if client should auto-join session
        Default implementation: never auto-join
        """
        return False
    
    async def _join_session(self, session_id: str) -> None:
        """
        Join a specific session while maintaining connection
        This will trigger a new WebSocket connection to the session
        """
        logger.info(f"🔄 DEBUG: Joining session {session_id}")
        try:
            # Reconnect with new session_id
            await self.connect(session_id)
        except Exception as e:
            logger.error(f"❌ Failed to join session {session_id}: {e}")
            # Stay in lobby on failure
            self.current_session_id = None

    # Context manager support
    
    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.disconnect()