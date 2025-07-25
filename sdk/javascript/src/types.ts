/**
 * Type definitions for Switchboard SDK
 */

export enum MessageType {
  INSTRUCTOR_INBOX = 'instructor_inbox',
  INBOX_RESPONSE = 'inbox_response',
  REQUEST = 'request',
  REQUEST_RESPONSE = 'request_response',
  ANALYTICS = 'analytics',
  INSTRUCTOR_BROADCAST = 'instructor_broadcast',
  SYSTEM = 'system'
}

export enum Role {
  STUDENT = 'student',
  INSTRUCTOR = 'instructor'
}

export interface Session {
  id: string;
  name: string;
  created_by: string;
  student_ids: string[];
  start_time: string;
  end_time?: string | null;
  status: string;
  connection_count?: number;
}

export interface MessageContent {
  [key: string]: any;
}

export interface Message {
  id?: string;
  type: MessageType;
  context: string;
  content: MessageContent;
  to_user?: string;
  from_user?: string;
  session_id?: string;
  timestamp?: string;
}

export interface IncomingMessage extends Message {
  id: string;
  from_user: string;
  session_id: string;
  timestamp: string;
}

export interface OutgoingMessage {
  type: MessageType;
  context: string;
  content: MessageContent;
  to_user?: string;
}

export interface ConnectionStatus {
  connected: boolean;
  session_id?: string;
  user_id?: string;
  role?: string;
  last_heartbeat?: Date;
  message_count: number;
  uptime_seconds: number;
}

export interface SessionsResponse {
  sessions: Session[];
}

export interface SessionResponse {
  session: Session;
  connection_count: number;
}

export interface CreateSessionRequest {
  name: string;
  instructor_id: string;
  student_ids: string[];
}

export interface CreateSessionResponse {
  session: Session;
}

// Event handler types
export type MessageHandler = (message: IncomingMessage) => void | Promise<void>;
export type ConnectionHandler = (connected: boolean) => void | Promise<void>;
export type ErrorHandler = (error: Error) => void | Promise<void>;

// Configuration interfaces
export interface SwitchboardClientConfig {
  serverUrl?: string;
  maxReconnectAttempts?: number;
  reconnectDelay?: number;
  heartbeatInterval?: number;
}

export interface StudentClientConfig extends SwitchboardClientConfig {
  // Student-specific configuration
}

export interface TeacherClientConfig extends SwitchboardClientConfig {
  // Teacher-specific configuration
}

// React hook types
export interface UseSwitchboardOptions extends SwitchboardClientConfig {
  autoConnect?: boolean;
  sessionId?: string;
}

export interface SwitchboardHookReturn {
  client: SwitchboardClient | null;
  connected: boolean;
  session: Session | null;
  messages: IncomingMessage[];
  connect: (sessionId: string) => Promise<void>;
  disconnect: () => Promise<void>;
  sendMessage: (message: OutgoingMessage) => Promise<void>;
  error: Error | null;
}

// Base client interface
export interface SwitchboardClient {
  readonly userId: string;
  readonly role: Role;
  readonly connected: boolean;
  readonly currentSessionId: string | null;
  
  // Connection methods
  connect(sessionId: string): Promise<void>;
  disconnect(): Promise<void>;
  
  // Session discovery
  discoverSessions(): Promise<Session[]>;
  getSession(sessionId: string): Promise<Session>;
  
  // Message sending
  sendMessage(message: OutgoingMessage): Promise<void>;
  
  // Event handlers
  onMessage(messageType: MessageType, handler: MessageHandler): void;
  onConnection(handler: ConnectionHandler): void;
  onError(handler: ErrorHandler): void;
  
  // Status
  getStatus(): ConnectionStatus;
}

// Error types
export class SwitchboardError extends Error {
  public readonly details?: Record<string, any>;
  
  constructor(message: string, details?: Record<string, any>) {
    super(message);
    this.name = 'SwitchboardError';
    this.details = details;
  }
}

export class ConnectionError extends SwitchboardError {
  constructor(message: string, details?: Record<string, any>) {
    super(message, details);
    this.name = 'ConnectionError';
  }
}

export class AuthenticationError extends SwitchboardError {
  constructor(message: string, details?: Record<string, any>) {
    super(message, details);
    this.name = 'AuthenticationError';
  }
}

export class SessionNotFoundError extends SwitchboardError {
  constructor(message: string, details?: Record<string, any>) {
    super(message, details);
    this.name = 'SessionNotFoundError';
  }
}

export class MessageValidationError extends SwitchboardError {
  constructor(message: string, details?: Record<string, any>) {
    super(message, details);
    this.name = 'MessageValidationError';
  }
}

export class RateLimitError extends SwitchboardError {
  constructor(message: string, details?: Record<string, any>) {
    super(message, details);
    this.name = 'RateLimitError';
  }
}

export class SessionEndedError extends SwitchboardError {
  constructor(message: string, details?: Record<string, any>) {
    super(message, details);
    this.name = 'SessionEndedError';
  }
}

export class ReconnectionFailedError extends SwitchboardError {
  constructor(message: string, details?: Record<string, any>) {
    super(message, details);
    this.name = 'ReconnectionFailedError';
  }
}