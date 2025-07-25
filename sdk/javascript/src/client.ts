/**
 * Base Switchboard client implementation
 */

import { EventEmitter } from 'events';
import {
  SwitchboardClient as ISwitchboardClient,
  SwitchboardClientConfig,
  Session,
  SessionsResponse,
  SessionResponse,
  Message,
  IncomingMessage,
  OutgoingMessage,
  MessageType,
  Role,
  ConnectionStatus,
  MessageHandler,
  ConnectionHandler,
  ErrorHandler,
  SwitchboardError,
  ConnectionError,
  AuthenticationError,
  SessionNotFoundError,
  SessionEndedError,
  ReconnectionFailedError
} from './types';

// Universal WebSocket implementation
let WebSocketImpl: any;
if (typeof window !== 'undefined') {
  // Browser environment
  WebSocketImpl = WebSocket;
} else {
  // Node.js environment
  try {
    WebSocketImpl = require('ws');
  } catch (e) {
    throw new Error('WebSocket implementation not found. Please install "ws" package for Node.js.');
  }
}

export abstract class BaseSwitchboardClient extends EventEmitter implements ISwitchboardClient {
  public readonly userId: string;
  public readonly role: Role;
  protected readonly serverUrl: string;
  protected readonly maxReconnectAttempts: number;
  protected readonly reconnectDelay: number;
  protected readonly heartbeatInterval: number;

  protected websocket: any = null;
  protected _connected = false;
  protected _currentSessionId: string | null = null;
  protected connectionStartTime: number | null = null;
  protected messageCount = 0;
  protected reconnectAttempts = 0;
  protected shutdown = false;

  // Event handlers
  protected messageHandlers = new Map<MessageType, MessageHandler[]>();
  protected connectionHandlers: ConnectionHandler[] = [];
  protected errorHandlers: ErrorHandler[] = [];

  // Timers
  protected reconnectTimer: NodeJS.Timeout | null = null;
  protected heartbeatTimer: NodeJS.Timeout | null = null;

  constructor(
    userId: string,
    role: Role,
    config: SwitchboardClientConfig = {}
  ) {
    super();
    
    this.userId = userId;
    this.role = role;
    this.serverUrl = config.serverUrl?.replace(/\/$/, '') || 'http://localhost:8080';
    this.maxReconnectAttempts = config.maxReconnectAttempts || 5;
    this.reconnectDelay = config.reconnectDelay || 1000;
    this.heartbeatInterval = config.heartbeatInterval || 30000;
  }

  // Properties
  get connected(): boolean {
    return this._connected;
  }

  get currentSessionId(): string | null {
    return this._currentSessionId;
  }

  // HTTP API Methods
  async discoverSessions(): Promise<Session[]> {
    try {
      const response = await this.fetch(`${this.serverUrl}/api/sessions`);
      
      if (!response.ok) {
        throw new SwitchboardError(`Failed to fetch sessions: HTTP ${response.status}`);
      }
      
      const data: SessionsResponse = await response.json();
      return data.sessions;
    } catch (error) {
      if (error instanceof SwitchboardError) {
        throw error;
      }
      throw new SwitchboardError(`Network error fetching sessions: ${error}`);
    }
  }

  async getSession(sessionId: string): Promise<Session> {
    try {
      const response = await this.fetch(`${this.serverUrl}/api/sessions/${sessionId}`);
      
      if (response.status === 404) {
        throw new SessionNotFoundError(`Session not found: ${sessionId}`);
      } else if (!response.ok) {
        throw new SwitchboardError(`Failed to fetch session: HTTP ${response.status}`);
      }
      
      const data: SessionResponse = await response.json();
      return {
        ...data.session,
        connection_count: data.connection_count
      };
    } catch (error) {
      if (error instanceof SwitchboardError) {
        throw error;
      }
      throw new SwitchboardError(`Network error fetching session: ${error}`);
    }
  }

  // WebSocket Connection Management
  async connect(sessionId: string): Promise<void> {
    this._currentSessionId = sessionId;
    this.shutdown = false;
    
    await this.establishConnection();
  }

  protected async establishConnection(): Promise<void> {
    if (!this._currentSessionId) {
      throw new SwitchboardError('No session ID set');
    }

    // Build WebSocket URL
    const wsBase = this.serverUrl.replace(/^http/, 'ws');
    const params = new URLSearchParams({
      user_id: this.userId,
      role: this.role,
      session_id: this._currentSessionId
    });
    const wsUrl = `${wsBase}/ws?${params.toString()}`;

    return new Promise((resolve, reject) => {
      try {
        this.websocket = new WebSocketImpl(wsUrl);

        const connectionTimeout = setTimeout(() => {
          if (!this._connected) {
            this.websocket?.close();
            reject(new ConnectionError('Connection timeout'));
          }
        }, 10000);

        this.websocket.onopen = () => {
          clearTimeout(connectionTimeout);
          this._connected = true;
          this.connectionStartTime = Date.now();
          this.reconnectAttempts = 0;
          this.messageCount = 0;

          this.startHeartbeat();
          this.notifyConnectionHandlers(true);
          this.emit('connected');
          
          console.log(`Connected to session ${this._currentSessionId}`);
          resolve();
        };

        this.websocket.onmessage = (event: MessageEvent) => {
          this.handleIncomingMessage(event.data);
        };

        this.websocket.onclose = (event: CloseEvent) => {
          this._connected = false;
          this.stopHeartbeat();
          
          console.log('WebSocket connection closed');
          
          if (!this.shutdown) {
            if (event.code === 403) {
              const error = new AuthenticationError('Not authorized for this session');
              this.notifyErrorHandlers(error);
              reject(error);
            } else if (event.code === 404) {
              const error = new SessionNotFoundError('Session not found or ended');
              this.notifyErrorHandlers(error);
              reject(error);
            } else {
              // Connection lost unexpectedly, attempt reconnection
              this.attemptReconnection();
            }
          }
          
          this.notifyConnectionHandlers(false);
          this.emit('disconnected');
        };

        this.websocket.onerror = (error: Event) => {
          console.error('WebSocket error:', error);
          
          if (!this._connected) {
            clearTimeout(connectionTimeout);
            reject(new ConnectionError('WebSocket connection failed'));
          }
        };

      } catch (error) {
        reject(new ConnectionError(`Failed to create WebSocket: ${error}`));
      }
    });
  }

  async disconnect(): Promise<void> {
    this.shutdown = true;
    
    // Clear timers
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    
    this.stopHeartbeat();
    
    // Close WebSocket
    if (this.websocket) {
      this.websocket.close(1000, 'User disconnected');
      this.websocket = null;
    }
    
    this._connected = false;
    this._currentSessionId = null;
    
    this.notifyConnectionHandlers(false);
    this.emit('disconnected');
    
    console.log('Disconnected from Switchboard');
  }

  protected handleIncomingMessage(data: string): void {
    try {
      const message: IncomingMessage = JSON.parse(data);
      
      this.messageCount++;
      
      // Handle system messages
      if (message.type === MessageType.SYSTEM) {
        this.handleSystemMessage(message);
      }
      
      // Notify message handlers
      this.notifyMessageHandlers(message);
      this.emit('message', message);
      
    } catch (error) {
      console.error('Failed to parse message:', error);
      this.notifyErrorHandlers(new SwitchboardError(`Invalid message format: ${error}`));
    }
  }

  protected handleSystemMessage(message: IncomingMessage): void {
    const event = message.content.event;
    
    switch (event) {
      case 'session_ended':
        console.log('Session ended by server');
        this.shutdown = true;
        this.disconnect();
        this.notifyErrorHandlers(new SessionEndedError('Session was ended'));
        break;
        
      case 'history_complete':
        console.log('Message history loaded');
        this.emit('historyComplete');
        break;
        
      case 'message_error':
        const errorMsg = message.content.message || 'Unknown message error';
        this.notifyErrorHandlers(new SwitchboardError(`Server message error: ${errorMsg}`));
        break;
    }
  }

  protected attemptReconnection(): void {
    if (this.shutdown || this.reconnectAttempts >= this.maxReconnectAttempts) {
      const error = new ReconnectionFailedError(
        `Failed to reconnect after ${this.maxReconnectAttempts} attempts`
      );
      this.notifyErrorHandlers(error);
      return;
    }
    
    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
    
    console.log(`Attempting reconnection ${this.reconnectAttempts}/${this.maxReconnectAttempts} in ${delay}ms`);
    
    this.reconnectTimer = setTimeout(async () => {
      try {
        if (!this.shutdown) {
          await this.establishConnection();
        }
      } catch (error) {
        console.error('Reconnection attempt failed:', error);
        this.attemptReconnection();
      }
    }, delay);
  }

  protected startHeartbeat(): void {
    this.heartbeatTimer = setInterval(() => {
      if (this.websocket && this._connected) {
        // Send ping frame (browser WebSocket handles this automatically)
        if (typeof this.websocket.ping === 'function') {
          this.websocket.ping();
        }
      }
    }, this.heartbeatInterval);
  }

  protected stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  // Message Sending
  async sendMessage(message: OutgoingMessage): Promise<void> {
    if (!this._connected || !this.websocket) {
      throw new ConnectionError('Not connected to session');
    }
    
    try {
      const messageData = {
        type: message.type,
        context: message.context || 'general',
        content: message.content,
        ...(message.to_user && { to_user: message.to_user })
      };
      
      this.websocket.send(JSON.stringify(messageData));
      console.log(`Sent ${message.type} message`);
      
    } catch (error) {
      throw new SwitchboardError(`Failed to send message: ${error}`);
    }
  }

  // Event Handler Registration
  onMessage(messageType: MessageType, handler: MessageHandler): void {
    if (!this.messageHandlers.has(messageType)) {
      this.messageHandlers.set(messageType, []);
    }
    this.messageHandlers.get(messageType)!.push(handler);
  }

  onConnection(handler: ConnectionHandler): void {
    this.connectionHandlers.push(handler);
  }

  onError(handler: ErrorHandler): void {
    this.errorHandlers.push(handler);
  }

  // Event notification
  protected notifyMessageHandlers(message: IncomingMessage): void {
    const handlers = this.messageHandlers.get(message.type);
    if (handlers) {
      handlers.forEach(handler => {
        try {
          const result = handler(message);
          if (result instanceof Promise) {
            result.catch(error => {
              console.error('Error in message handler:', error);
            });
          }
        } catch (error) {
          console.error('Error in message handler:', error);
        }
      });
    }
  }

  protected notifyConnectionHandlers(connected: boolean): void {
    this.connectionHandlers.forEach(handler => {
      try {
        const result = handler(connected);
        if (result instanceof Promise) {
          result.catch(error => {
            console.error('Error in connection handler:', error);
          });
        }
      } catch (error) {
        console.error('Error in connection handler:', error);
      }
    });
  }

  protected notifyErrorHandlers(error: Error): void {
    this.errorHandlers.forEach(handler => {
      try {
        const result = handler(error);
        if (result instanceof Promise) {
          result.catch(handlerError => {
            console.error('Error in error handler:', handlerError);
          });
        }
      } catch (handlerError) {
        console.error('Error in error handler:', handlerError);
      }
    });
    
    this.emit('error', error);
  }

  // Status
  getStatus(): ConnectionStatus {
    const uptime = this.connectionStartTime ? 
      Math.floor((Date.now() - this.connectionStartTime) / 1000) : 0;
    
    return {
      connected: this._connected,
      session_id: this._currentSessionId,
      user_id: this.userId,
      role: this.role,
      message_count: this.messageCount,
      uptime_seconds: uptime
    };
  }

  // Utility methods
  protected async fetch(url: string, options?: RequestInit): Promise<Response> {
    // Use appropriate fetch implementation
    if (typeof window !== 'undefined' && window.fetch) {
      return window.fetch(url, options);
    } else if (typeof global !== 'undefined' && (global as any).fetch) {
      return (global as any).fetch(url, options);
    } else {
      // Try to use node-fetch in Node.js environment
      try {
        const fetch = require('node-fetch');
        return fetch(url, options);
      } catch (e) {
        throw new Error('Fetch implementation not found. Please install "node-fetch" for Node.js or use a modern browser.');
      }
    }
  }
}