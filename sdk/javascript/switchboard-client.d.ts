/**
 * TypeScript definitions for Switchboard Client SDK
 */

export = SwitchboardClient;

declare class SwitchboardClient {
  constructor(options: SwitchboardClientOptions);

  // Properties
  readonly userId: string;
  readonly role: 'student' | 'instructor';
  readonly wsUrl: string;
  readonly apiUrl: string;
  readonly connectionState: ConnectionState;
  readonly sessionActive: boolean;
  readonly currentSession: Session | null;
  readonly messages: Message[];
  readonly historyDelivered: boolean;
  debug: boolean;

  // Connection Methods
  connect(): Promise<void>;
  disconnect(): void;
  isConnected(): boolean;
  isSessionActive(): boolean;
  getConnectionStatus(): ConnectionState;
  getCurrentSession(): Session | null;

  // Message Factory Methods
  Message(type: MessageType, name: string, toUser?: string | null): MessageBuilder;
  broadcast_to_instructors(name: string): MessageBuilder;
  broadcast_to_students(name: string): MessageBuilder;
  direct_message(toUser: string, name: string): MessageBuilder;

  // Session Management (Instructors)
  startSession(sessionName: string): Promise<SessionResponse>;
  endSession(): Promise<EndSessionResponse>;
  getActiveSession(): Promise<Session | null>;

  // Message History
  getMessages(filterType?: MessageType | null, limit?: number | null): Message[];
  clearMessages(): void;

  // Search/Filter API
  readonly search: MessageSearchAPI;

  // Helpers
  readonly helpers: MessageHelpers;

  // Rate Limiting
  getRateLimitStatus(): RateLimitStatus;

  // Debugging
  setDebug(enabled: boolean): void;

  // Static Constants
  static readonly MessageType: {
    readonly BROADCAST_TO_INSTRUCTORS: 'broadcast_to_instructors';
    readonly DIRECT_MESSAGE: 'direct_message';
    readonly BROADCAST_TO_STUDENTS: 'broadcast_to_students';
  };

  static readonly MessageContext: {
    readonly QUESTION: 'question';
    readonly RESPONSE: 'response';
    readonly ANNOUNCEMENT: 'announcement';
    readonly INSTRUCTION: 'instruction';
    readonly EMERGENCY: 'emergency';
    readonly SUBMISSION: 'submission';
    readonly ANALYTICS: 'analytics';
    readonly REQUEST: 'request';
    readonly PEER_HELP: 'peer_help';
    readonly GENERAL: 'general';
  };

  static readonly ConnectionState: {
    readonly DISCONNECTED: 'disconnected';
    readonly CONNECTING: 'connecting';
    readonly CONNECTED: 'connected';
  };
}

interface SwitchboardClientOptions {
  userId: string;
  role: 'student' | 'instructor';
  wsUrl: string;
  apiUrl?: string;
  maxReconnectAttempts?: number;
  queueMessages?: boolean;
  maxStoredMessages?: number;
  debug?: boolean;
  hooks?: EventHooks;
}

interface EventHooks {
  // Connection events
  onConnecting?: () => void;
  onConnected?: () => void;
  onDisconnected?: (code: number, reason: string) => void;
  onReconnecting?: (attempt: number, delay: number) => void;
  onConnectionError?: (error: Error) => void;

  // Message events
  onMessage?: (message: Message) => void;
  onBroadcastToInstructors?: (message: Message) => void;
  onBroadcastToStudents?: (message: Message) => void;
  onDirectMessage?: (message: Message) => void;

  // Session events
  onSessionStarted?: (session: Session, message: SystemMessage) => void;
  onSessionEnded?: (session: Session | null, message: SystemMessage) => void;
  onSessionActive?: (session: Session, message: SystemMessage) => void;
  onWaitingForSession?: (message: SystemMessage) => void;
  onHistoryDelivered?: (message: SystemMessage) => void;
  onSystemMessage?: (message: SystemMessage) => void;

  // Error events
  onError?: (error: ErrorMessage) => void;
  onRateLimited?: (error: ErrorMessage) => void;
  onNoActiveSession?: (error: ErrorMessage) => void;
  onMessageTooLarge?: (error: ErrorMessage) => void;
}

declare class MessageBuilder {
  withText(text: string): this;
  withCode(code: string, language?: string | null): this;
  withLineNumber(lineNumber: number): this;
  withData(data: Record<string, any>): this;
  withContext(context: MessageContext): this;
  markAsImportant(): this;
  withUrgency(level: 'low' | 'medium' | 'high' | 'urgent'): this;
  withTags(...tags: string[]): this;
  referencingMessage(messageId: string): this;
  send(): Message;
}

// Search/Filter API
interface MessageSearchAPI {
  where(field: MessageField, value: any): MessageSearchAPI;
  where(field: 'type', value: MessageType): MessageSearchAPI;
  where(field: 'context', value: MessageContext): MessageSearchAPI;
  where(field: 'from_user', value: string): MessageSearchAPI;
  where(field: 'to_user', value: string): MessageSearchAPI;
  where(field: string, value: any): MessageSearchAPI;
  
  containing(text: string): MessageSearchAPI;
  having(field: string): MessageSearchAPI;
  notHaving(field: string): MessageSearchAPI;
  
  since(time: Date | string | number): MessageSearchAPI;
  before(time: Date | string | number): MessageSearchAPI;
  between(start: Date | string | number, end: Date | string | number): MessageSearchAPI;
  
  referencedBy(messageId: string): MessageSearchAPI;
  notReferenced(): MessageSearchAPI;
  
  limit(count: number): MessageSearchAPI;
  offset(count: number): MessageSearchAPI;
  
  orderBy(field: MessageField, direction?: 'asc' | 'desc'): MessageSearchAPI;
  groupBy(field: MessageField): MessageGroupedResult;
  
  get(): Message[];
  count(): number;
  first(): Message | null;
  last(): Message | null;
}

interface MessageGroupedResult {
  [key: string]: Message[];
  having(predicate: (messages: Message[]) => boolean): MessageGroupedResult;
  get(): Record<string, Message[]>;
}

// Helper Types
interface MessageHelpers {
  formatMessage(message: Message): FormattedMessage;
  formatTimestamp(timestamp: string | Date): string;
  getRelativeTime(timestamp: string | Date): string;
  getConnectionStatus(): ConnectionStatus;
  groupMessageThreads(messages: Message[]): MessageThread[];
  isOwnMessage(message: Message): boolean;
  extractMentions(text: string): string[];
  parseCode(content: MessageContent): ParsedCode | null;
}

interface FormattedMessage {
  displayName: string;
  timeAgo: string;
  timestamp: string;
  isOwn: boolean;
  excerpt: string;
  hasCode: boolean;
  hasData: boolean;
  urgency: 'low' | 'medium' | 'high' | 'urgent' | 'normal';
  tags: string[];
  type: MessageType;
  context: MessageContext;
}

interface ConnectionStatus {
  state: ConnectionState;
  sessionActive: boolean;
  sessionName: string | null;
  sessionId: string | null;
  canSendMessages: boolean;
  isReconnecting: boolean;
  reconnectAttempt: number;
}

interface MessageThread {
  original: Message;
  responses: Message[];
  participants: string[];
  lastActivity: Date;
  isResolved: boolean;
}

interface ParsedCode {
  snippet: string;
  language: string | null;
  lineNumber: number | null;
  isComplete: boolean;
}

// Core Types
type MessageType = 'broadcast_to_instructors' | 'direct_message' | 'broadcast_to_students';
type MessageContext = 'question' | 'response' | 'announcement' | 'instruction' | 
  'emergency' | 'submission' | 'analytics' | 'request' | 'peer_help' | 'general';
type ConnectionState = 'disconnected' | 'connecting' | 'connected';
type MessageField = 'id' | 'type' | 'context' | 'from_user' | 'to_user' | 
  'timestamp' | 'session_id' | 'content';

interface Message {
  id: string;
  session_id: string;
  type: MessageType;
  context: MessageContext;
  from_user: string;
  to_user: string | null;
  content: MessageContent;
  timestamp: string;
}

interface MessageContent {
  name?: string;
  text?: string;
  code_snippet?: string;
  language?: string;
  line_number?: number;
  important?: boolean;
  urgency?: 'low' | 'medium' | 'high' | 'urgent';
  tags?: string[];
  reference_message_id?: string;
  [key: string]: any; // Additional arbitrary data
}

interface SystemMessage {
  type: 'system';
  content: {
    event: SystemEvent;
    message?: string;
    session_id?: string;
    session_name?: string;
    started_by?: string;
    instructor?: string;
    start_time?: string;
    user_id?: string;
    role?: string;
  };
  timestamp: string;
}

interface ErrorMessage {
  type: 'error';
  error: ErrorCode;
  message: string;
  timestamp: string;
}

interface Session {
  id: string;
  name: string;
  startedBy: string;
  startTime: Date;
}

interface SessionResponse {
  session: {
    id: string;
    name: string;
    created_by: string;
    status: 'active';
    start_time: string;
  };
}

interface EndSessionResponse {
  session_id: string;
  status: 'ended';
  ended_at: string;
}

interface RateLimitStatus {
  messagesSent: number;
  maxMessages: number;
  windowMs: number;
  canSend: boolean;
}

type SystemEvent = 'session_started' | 'session_ended' | 'waiting_for_session' | 
  'session_active' | 'history_delivered' | 'user_connected' | 'user_disconnected';

type ErrorCode = 'no_active_session' | 'rate_limit_exceeded' | 'invalid_message' | 
  'invalid_message_type' | 'invalid_recipient' | 'message_too_large';

// Optional UI Components (separate import)
declare module 'switchboard-client/ui' {
  export class SwitchboardUI {
    constructor(client: SwitchboardClient, options?: UIOptions);
    
    createMessageList(selector: string, options?: MessageListOptions): MessageList;
    createMessageInput(selector: string, options?: MessageInputOptions): MessageInput;
    createStatusBadge(selector: string, options?: StatusBadgeOptions): StatusBadge;
    createDashboard(selector: string, options?: DashboardOptions): Dashboard;
    
    enableNotifications(options?: NotificationOptions): void;
    setTheme(theme: Theme | 'light' | 'dark' | 'auto'): void;
  }

  interface UIOptions {
    theme?: 'light' | 'dark' | 'auto';
    animations?: boolean;
    sounds?: boolean;
    customCSS?: string;
  }

  interface MessageListOptions {
    groupByUser?: boolean;
    showAvatars?: boolean;
    enableReactions?: boolean;
    codeHighlighting?: 'prism' | 'highlight.js' | false;
    virtualized?: boolean;
    threadLines?: boolean;
    bubbles?: boolean;
    themes?: {
      student?: string;
      instructor?: string;
      system?: string;
    };
  }

  interface MessageInputOptions {
    allowMarkdown?: boolean;
    codeButton?: boolean;
    dragDropFiles?: boolean;
    autoComplete?: boolean;
    characterLimit?: boolean;
    templates?: string[];
    mathJax?: boolean;
  }

  interface StatusBadgeOptions {
    showUserCount?: boolean;
    showSessionInfo?: boolean;
    pulseOnActivity?: boolean;
  }

  interface DashboardOptions {
    layout: 'student' | 'instructor';
    components: Array<'messageList' | 'activeQuestions' | 'studentRoster' | 'analytics'>;
  }

  interface NotificationOptions {
    desktop?: boolean;
    sounds?: boolean;
    vibration?: boolean;
    customSounds?: Record<string, string>;
  }

  interface Theme {
    colors: Record<string, string>;
    fonts: Record<string, string>;
    spacing: Record<string, string>;
  }

  interface MessageList {
    addMessage(message: Message): void;
    clear(): void;
    scrollToBottom(): void;
    highlight(messageId: string): void;
  }

  interface MessageInput {
    getValue(): string;
    setValue(text: string): void;
    clear(): void;
    focus(): void;
    insertCode(code: string, language?: string): void;
  }

  interface StatusBadge {
    update(status: ConnectionStatus): void;
  }

  interface Dashboard {
    refresh(): void;
    showComponent(name: string): void;
    hideComponent(name: string): void;
  }
}