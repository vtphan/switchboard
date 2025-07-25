/**
 * Teacher client implementation for Switchboard
 */

import { BaseSwitchboardClient } from './client';
import {
  Session,
  MessageType,
  Role,
  TeacherClientConfig,
  OutgoingMessage,
  MessageContent,
  IncomingMessage,
  MessageHandler,
  CreateSessionRequest,
  CreateSessionResponse,
  SwitchboardError,
  SessionNotFoundError
} from './types';

export class SwitchboardTeacher extends BaseSwitchboardClient {
  constructor(userId: string, config: TeacherClientConfig = {}) {
    super(userId, Role.INSTRUCTOR, config);
  }

  // Session management (Teacher-specific)
  async createSession(name: string, studentIds: string[]): Promise<Session> {
    try {
      const payload: CreateSessionRequest = {
        name,
        instructor_id: this.userId,
        student_ids: studentIds
      };
      
      const response = await this.fetch(`${this.serverUrl}/api/sessions`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(payload)
      });
      
      if (response.status === 201) {
        const data: CreateSessionResponse = await response.json();
        return data.session;
      } else {
        const errorText = await response.text();
        throw new SwitchboardError(`Failed to create session: HTTP ${response.status} - ${errorText}`);
      }
    } catch (error) {
      if (error instanceof SwitchboardError) {
        throw error;
      }
      throw new SwitchboardError(`Network error creating session: ${error}`);
    }
  }

  async endSession(sessionId: string): Promise<void> {
    try {
      const response = await this.fetch(`${this.serverUrl}/api/sessions/${sessionId}`, {
        method: 'DELETE'
      });
      
      if (response.status === 404) {
        throw new SessionNotFoundError(`Session not found: ${sessionId}`);
      } else if (response.status !== 200) {
        const errorText = await response.text();
        throw new SwitchboardError(`Failed to end session: HTTP ${response.status} - ${errorText}`);
      }
    } catch (error) {
      if (error instanceof SwitchboardError) {
        throw error;
      }
      throw new SwitchboardError(`Network error ending session: ${error}`);
    }
  }

  async listActiveSessions(): Promise<Session[]> {
    const allSessions = await this.discoverSessions();
    return allSessions.filter(session => session.status === 'active');
  }

  // Teacher message sending methods
  async respondToStudent(
    studentId: string, 
    content: MessageContent, 
    context = 'answer'
  ): Promise<void> {
    const message: OutgoingMessage = {
      type: MessageType.INBOX_RESPONSE,
      context,
      content,
      to_user: studentId
    };
    
    await this.sendMessage(message);
  }

  async requestFromStudent(
    studentId: string, 
    content: MessageContent, 
    context = 'request'
  ): Promise<void> {
    const message: OutgoingMessage = {
      type: MessageType.REQUEST,
      context,
      content,
      to_user: studentId
    };
    
    await this.sendMessage(message);
  }

  async broadcastToStudents(
    content: MessageContent, 
    context = 'announcement'
  ): Promise<void> {
    const message: OutgoingMessage = {
      type: MessageType.INSTRUCTOR_BROADCAST,
      context,
      content
    };
    
    await this.sendMessage(message);
  }

  // Convenience methods for common teacher actions
  async announce(text: string, additionalContent: Record<string, any> = {}): Promise<void> {
    const content = { text, ...additionalContent };
    await this.broadcastToStudents(content, 'announcement');
  }

  async giveInstruction(instruction: string, additionalContent: Record<string, any> = {}): Promise<void> {
    const content = { text: instruction, ...additionalContent };
    await this.broadcastToStudents(content, 'instruction');
  }

  async requestCodeFromStudent(options: {
    studentId: string;
    prompt: string;
    requirements?: string[];
    deadline?: string;
  }): Promise<void> {
    const content = {
      text: options.prompt,
      requirements: options.requirements || [],
      deadline: options.deadline
    };
    
    await this.requestFromStudent(options.studentId, content, 'code');
  }

  async provideFeedback(options: {
    studentId: string;
    feedback: string;
    codeExample?: string;
    resources?: string[];
  }): Promise<void> {
    const content = {
      text: options.feedback,
      code_example: options.codeExample,
      additional_resources: options.resources || []
    };
    
    await this.respondToStudent(options.studentId, content, 'feedback');
  }

  async scheduleBreak(options: {
    durationMinutes: number;
    resumeTime?: string;
    instructions?: string;
  }): Promise<void> {
    const content = {
      text: `We'll take a ${options.durationMinutes}-minute break.`,
      break_duration: options.durationMinutes * 60, // Convert to seconds
      resume_time: options.resumeTime,
      instructions: options.instructions
    };
    
    await this.broadcastToStudents(content, 'break');
  }

  async broadcastProblem(options: {
    problem: string;
    code?: string;
    timeOnTask?: number;
    remainingTime?: number;
    frustrationLevel?: number;
  }): Promise<void> {
    const content = {
      problem: options.problem,
      code: options.code || '',
      timeOnTask: options.timeOnTask || 0,
      remainingTime: options.remainingTime || 30,
      frustrationLevel: options.frustrationLevel || 2
    };
    
    await this.broadcastToStudents(content, 'problem');
  }

  // Session flow management
  async createAndConnect(sessionName: string, studentIds: string[]): Promise<Session> {
    const session = await this.createSession(sessionName, studentIds);
    await this.connect(session.id);
    return session;
  }

  async endCurrentSession(): Promise<void> {
    if (!this.currentSessionId) {
      throw new SwitchboardError('No session currently connected');
    }
    
    await this.endSession(this.currentSessionId);
    await this.disconnect();
  }

  // Event handler convenience methods
  onStudentQuestion(handler: MessageHandler): void {
    this.onMessage(MessageType.INSTRUCTOR_INBOX, handler);
  }

  onStudentResponse(handler: MessageHandler): void {
    this.onMessage(MessageType.REQUEST_RESPONSE, handler);
  }

  onStudentAnalytics(handler: MessageHandler): void {
    this.onMessage(MessageType.ANALYTICS, handler);
  }

  onSystemMessage(handler: MessageHandler): void {
    this.onMessage(MessageType.SYSTEM, handler);
  }

  // Convenience method to set up common event handlers
  setupEventHandlers(handlers: {
    onStudentQuestion?: (message: IncomingMessage) => void | Promise<void>;
    onStudentResponse?: (message: IncomingMessage) => void | Promise<void>;
    onStudentAnalytics?: (message: IncomingMessage) => void | Promise<void>;
    onSystemMessage?: (message: IncomingMessage) => void | Promise<void>;
    onConnection?: (connected: boolean) => void | Promise<void>;
    onError?: (error: Error) => void | Promise<void>;
  }): void {
    if (handlers.onStudentQuestion) {
      this.onStudentQuestion(handlers.onStudentQuestion);
    }
    
    if (handlers.onStudentResponse) {
      this.onStudentResponse(handlers.onStudentResponse);
    }
    
    if (handlers.onStudentAnalytics) {
      this.onStudentAnalytics(handlers.onStudentAnalytics);
    }
    
    if (handlers.onSystemMessage) {
      this.onSystemMessage(handlers.onSystemMessage);
    }
    
    if (handlers.onConnection) {
      this.onConnection(handlers.onConnection);
    }
    
    if (handlers.onError) {
      this.onError(handlers.onError);
    }
  }

  // Batch operations
  async requestFromMultipleStudents(
    studentIds: string[], 
    content: MessageContent, 
    context = 'request'
  ): Promise<void> {
    const promises = studentIds.map(studentId => 
      this.requestFromStudent(studentId, content, context)
    );
    
    await Promise.all(promises);
  }

  async provideFeedbackToMultipleStudents(
    feedbackMap: Map<string, { feedback: string; codeExample?: string; resources?: string[] }>
  ): Promise<void> {
    const promises = Array.from(feedbackMap.entries()).map(([studentId, options]) =>
      this.provideFeedback({ studentId, ...options })
    );
    
    await Promise.all(promises);
  }
}