/**
 * Switchboard JavaScript/TypeScript SDK
 * 
 * A comprehensive client library for Switchboard real-time educational messaging system.
 * Supports both Node.js and browser environments with TypeScript support.
 */

// Core client classes
export { BaseSwitchboardClient } from './client';
import { SwitchboardStudent } from './student';
export { SwitchboardStudent };
export { SwitchboardTeacher } from './teacher';

// React hooks (only exported if React is available)
let reactHooks: any = {};
try {
  if (typeof window !== 'undefined' || (typeof require !== 'undefined' && require.resolve('react'))) {
    reactHooks = require('./react');
  }
} catch (e) {
  // React not available, hooks will be empty
}

export const {
  useSwitchboardStudent,
  useSwitchboardTeacher,
  useMessagesByType,
  useLatestMessage,
  useConnectionStatus
} = reactHooks;

// Type definitions
export type {
  // Interfaces
  SwitchboardClient,
  Session,
  Message,
  IncomingMessage,
  OutgoingMessage,
  MessageContent,
  ConnectionStatus,
  
  // Configuration
  SwitchboardClientConfig,
  StudentClientConfig,
  TeacherClientConfig,
  UseSwitchboardOptions,
  SwitchboardHookReturn,
  
  // Handler types
  MessageHandler,
  ConnectionHandler,
  ErrorHandler,
  
  // API types
  SessionsResponse,
  SessionResponse,
  CreateSessionRequest,
  CreateSessionResponse
} from './types';

// Enums
export { MessageType, Role } from './types';

// Error classes
export {
  SwitchboardError,
  ConnectionError,
  AuthenticationError,
  SessionNotFoundError,
  MessageValidationError,
  RateLimitError,
  SessionEndedError,
  ReconnectionFailedError
} from './types';

// Utility functions
export function createStudentClient(userId: string, options?: any): SwitchboardStudent {
  return new SwitchboardStudent(userId, options);
}

export function createTeacherClient(userId: string, options?: any): SwitchboardTeacher {
  return new SwitchboardTeacher(userId, options);
}

// Version
export const VERSION = '1.0.0';

// Default export for convenience
export default {
  SwitchboardStudent,
  SwitchboardTeacher,
  createStudentClient,
  createTeacherClient,
  MessageType,
  Role,
  VERSION
};