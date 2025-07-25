/**
 * Student client implementation for Switchboard
 */

import { BaseSwitchboardClient } from './client';
import {
  Session,
  MessageType,
  Role,
  StudentClientConfig,
  OutgoingMessage,
  MessageContent,
  IncomingMessage,
  MessageHandler
} from './types';

export class SwitchboardStudent extends BaseSwitchboardClient {
  constructor(userId: string, config: StudentClientConfig = {}) {
    super(userId, Role.STUDENT, config);
  }

  // Student-specific session discovery
  async findAvailableSessions(): Promise<Session[]> {
    const allSessions = await this.discoverSessions();
    
    // Filter to sessions where student is enrolled and active
    return allSessions.filter(session => 
      session.student_ids.includes(this.userId) && 
      session.status === 'active'
    );
  }

  async connectToAvailableSession(): Promise<Session | null> {
    const availableSessions = await this.findAvailableSessions();
    
    if (availableSessions.length === 0) {
      return null;
    }
    
    const session = availableSessions[0];
    await this.connect(session.id);
    return session;
  }

  // Student message sending methods
  async askQuestion(content: MessageContent, context = 'question'): Promise<void> {
    const message: OutgoingMessage = {
      type: MessageType.INSTRUCTOR_INBOX,
      context,
      content
    };
    
    await this.sendMessage(message);
  }

  async respondToRequest(content: MessageContent, context = 'response'): Promise<void> {
    const message: OutgoingMessage = {
      type: MessageType.REQUEST_RESPONSE,
      context,
      content
    };
    
    await this.sendMessage(message);
  }

  async sendAnalytics(content: MessageContent, context = 'progress'): Promise<void> {
    const message: OutgoingMessage = {
      type: MessageType.ANALYTICS,
      context,
      content
    };
    
    await this.sendMessage(message);
  }

  // Convenience methods for common student actions
  async reportProgress(options: {
    completionPercentage: number;
    timeSpentMinutes: number;
    currentTopic: string;
    exercisesCompleted?: number;
    exercisesTotal?: number;
  }): Promise<void> {
    const content = {
      completion_percentage: options.completionPercentage,
      time_spent_minutes: options.timeSpentMinutes,
      current_topic: options.currentTopic,
      exercises_completed: options.exercisesCompleted || 0,
      exercises_total: options.exercisesTotal || 0
    };
    
    await this.sendAnalytics(content, 'progress');
  }

  async reportEngagement(options: {
    attentionLevel: 'high' | 'medium' | 'low';
    confusionLevel: 'high' | 'medium' | 'low';
    participationScore: number;
  }): Promise<void> {
    const content = {
      attention_level: options.attentionLevel,
      confusion_level: options.confusionLevel,
      participation_score: options.participationScore,
      last_interaction: this.getStatus().uptime_seconds
    };
    
    await this.sendAnalytics(content, 'engagement');
  }

  async reportError(options: {
    errorType: string;
    errorMessage: string;
    codeContext?: string;
    attemptedFixes?: number;
    timeStuckMinutes?: number;
  }): Promise<void> {
    const content = {
      error_type: options.errorType,
      error_message: options.errorMessage,
      code_context: options.codeContext || '',
      attempted_fixes: options.attemptedFixes || 0,
      time_stuck_minutes: options.timeStuckMinutes || 0
    };
    
    await this.sendAnalytics(content, 'error');
  }

  async requestHelp(options: {
    topic: string;
    description: string;
    urgency?: 'low' | 'medium' | 'high';
    codeContext?: string;
  }): Promise<void> {
    const content = {
      text: options.description,
      topic: options.topic,
      urgency: options.urgency || 'medium',
      code_context: options.codeContext || ''
    };
    
    await this.askQuestion(content, 'help');
  }

  // Event handler convenience methods
  onInstructorResponse(handler: MessageHandler): void {
    this.onMessage(MessageType.INBOX_RESPONSE, handler);
  }

  onInstructorRequest(handler: MessageHandler): void {
    this.onMessage(MessageType.REQUEST, handler);
  }

  onInstructorBroadcast(handler: MessageHandler): void {
    this.onMessage(MessageType.INSTRUCTOR_BROADCAST, handler);
  }

  onSystemMessage(handler: MessageHandler): void {
    this.onMessage(MessageType.SYSTEM, handler);
  }

  // Convenience method to set up common event handlers
  setupEventHandlers(handlers: {
    onInstructorResponse?: (message: IncomingMessage) => void | Promise<void>;
    onInstructorRequest?: (message: IncomingMessage) => void | Promise<void>;
    onInstructorBroadcast?: (message: IncomingMessage) => void | Promise<void>;
    onSystemMessage?: (message: IncomingMessage) => void | Promise<void>;
    onConnection?: (connected: boolean) => void | Promise<void>;
    onError?: (error: Error) => void | Promise<void>;
  }): void {
    if (handlers.onInstructorResponse) {
      this.onInstructorResponse(handlers.onInstructorResponse);
    }
    
    if (handlers.onInstructorRequest) {
      this.onInstructorRequest(handlers.onInstructorRequest);
    }
    
    if (handlers.onInstructorBroadcast) {
      this.onInstructorBroadcast(handlers.onInstructorBroadcast);
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
}