/**
 * React hooks for Switchboard SDK
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import { SwitchboardStudent } from './student';
import { SwitchboardTeacher } from './teacher';
import {
  Session,
  IncomingMessage,
  OutgoingMessage,
  UseSwitchboardOptions,
  SwitchboardHookReturn,
  Role,
  MessageHandler,
  ConnectionHandler,
  ErrorHandler
} from './types';

// Student hook
export function useSwitchboardStudent(
  userId: string,
  options: UseSwitchboardOptions = {}
): SwitchboardHookReturn {
  const clientRef = useRef<SwitchboardStudent | null>(null);
  const [connected, setConnected] = useState(false);
  const [session, setSession] = useState<Session | null>(null);
  const [messages, setMessages] = useState<IncomingMessage[]>([]);
  const [error, setError] = useState<Error | null>(null);

  // Initialize client
  useEffect(() => {
    clientRef.current = new SwitchboardStudent(userId, options);
    
    const client = clientRef.current;

    // Set up event handlers
    const connectionHandler: ConnectionHandler = (isConnected) => {
      setConnected(isConnected);
      if (!isConnected) {
        setSession(null);
      }
    };

    const messageHandler: MessageHandler = (message) => {
      setMessages(prev => [...prev, message]);
    };

    const errorHandler: ErrorHandler = (err) => {
      setError(err);
    };

    client.onConnection(connectionHandler);
    client.onMessage('instructor_inbox', messageHandler);
    client.onMessage('inbox_response', messageHandler);
    client.onMessage('request', messageHandler);
    client.onMessage('request_response', messageHandler);
    client.onMessage('analytics', messageHandler);
    client.onMessage('instructor_broadcast', messageHandler);
    client.onMessage('system', messageHandler);
    client.onError(errorHandler);

    // Auto-connect if specified
    if (options.autoConnect && options.sessionId) {
      client.connect(options.sessionId).catch(errorHandler);
    }

    return () => {
      client.disconnect();
    };
  }, [userId, options.serverUrl, options.maxReconnectAttempts, options.reconnectDelay]);

  const connect = useCallback(async (sessionId: string) => {
    if (!clientRef.current) return;
    
    try {
      setError(null);
      await clientRef.current.connect(sessionId);
      
      // Get session details
      const sessionDetails = await clientRef.current.getSession(sessionId);
      setSession(sessionDetails);
    } catch (err) {
      setError(err as Error);
      throw err;
    }
  }, []);

  const disconnect = useCallback(async () => {
    if (!clientRef.current) return;
    
    await clientRef.current.disconnect();
    setSession(null);
    setMessages([]);
  }, []);

  const sendMessage = useCallback(async (message: OutgoingMessage) => {
    if (!clientRef.current) {
      throw new Error('Client not initialized');
    }
    
    try {
      setError(null);
      await clientRef.current.sendMessage(message);
    } catch (err) {
      setError(err as Error);
      throw err;
    }
  }, []);

  return {
    client: clientRef.current,
    connected,
    session,
    messages,
    connect,
    disconnect,
    sendMessage,
    error
  };
}

// Teacher hook
export function useSwitchboardTeacher(
  userId: string,
  options: UseSwitchboardOptions = {}
): SwitchboardHookReturn & {
  createSession: (name: string, studentIds: string[]) => Promise<Session>;
  endSession: (sessionId: string) => Promise<void>;
  listActiveSessions: () => Promise<Session[]>;
} {
  const clientRef = useRef<SwitchboardTeacher | null>(null);
  const [connected, setConnected] = useState(false);
  const [session, setSession] = useState<Session | null>(null);
  const [messages, setMessages] = useState<IncomingMessage[]>([]);
  const [error, setError] = useState<Error | null>(null);

  // Initialize client
  useEffect(() => {
    clientRef.current = new SwitchboardTeacher(userId, options);
    
    const client = clientRef.current;

    // Set up event handlers
    const connectionHandler: ConnectionHandler = (isConnected) => {
      setConnected(isConnected);
      if (!isConnected) {
        setSession(null);
      }
    };

    const messageHandler: MessageHandler = (message) => {
      setMessages(prev => [...prev, message]);
    };

    const errorHandler: ErrorHandler = (err) => {
      setError(err);
    };

    client.onConnection(connectionHandler);
    client.onMessage('instructor_inbox', messageHandler);
    client.onMessage('inbox_response', messageHandler);
    client.onMessage('request', messageHandler);
    client.onMessage('request_response', messageHandler);
    client.onMessage('analytics', messageHandler);
    client.onMessage('instructor_broadcast', messageHandler);
    client.onMessage('system', messageHandler);
    client.onError(errorHandler);

    // Auto-connect if specified
    if (options.autoConnect && options.sessionId) {
      client.connect(options.sessionId).catch(errorHandler);
    }

    return () => {
      client.disconnect();
    };
  }, [userId, options.serverUrl, options.maxReconnectAttempts, options.reconnectDelay]);

  const connect = useCallback(async (sessionId: string) => {
    if (!clientRef.current) return;
    
    try {
      setError(null);
      await clientRef.current.connect(sessionId);
      
      // Get session details
      const sessionDetails = await clientRef.current.getSession(sessionId);
      setSession(sessionDetails);
    } catch (err) {
      setError(err as Error);
      throw err;
    }
  }, []);

  const disconnect = useCallback(async () => {
    if (!clientRef.current) return;
    
    await clientRef.current.disconnect();
    setSession(null);
    setMessages([]);
  }, []);

  const sendMessage = useCallback(async (message: OutgoingMessage) => {
    if (!clientRef.current) {
      throw new Error('Client not initialized');
    }
    
    try {
      setError(null);
      await clientRef.current.sendMessage(message);
    } catch (err) {
      setError(err as Error);
      throw err;
    }
  }, []);

  const createSession = useCallback(async (name: string, studentIds: string[]) => {
    if (!clientRef.current) {
      throw new Error('Client not initialized');
    }
    
    try {
      setError(null);
      const newSession = await clientRef.current.createSession(name, studentIds);
      return newSession;
    } catch (err) {
      setError(err as Error);
      throw err;
    }
  }, []);

  const endSession = useCallback(async (sessionId: string) => {
    if (!clientRef.current) {
      throw new Error('Client not initialized');
    }
    
    try {
      setError(null);
      await clientRef.current.endSession(sessionId);
    } catch (err) {
      setError(err as Error);
      throw err;
    }
  }, []);

  const listActiveSessions = useCallback(async () => {
    if (!clientRef.current) {
      throw new Error('Client not initialized');
    }
    
    try {
      setError(null);
      return await clientRef.current.listActiveSessions();
    } catch (err) {
      setError(err as Error);
      throw err;
    }
  }, []);

  return {
    client: clientRef.current,
    connected,
    session,
    messages,
    connect,
    disconnect,
    sendMessage,
    createSession,
    endSession,
    listActiveSessions,
    error
  };
}

// Custom hook for filtering messages by type
export function useMessagesByType(messages: IncomingMessage[], messageType: string) {
  return messages.filter(message => message.type === messageType);
}

// Custom hook for latest message of a type
export function useLatestMessage(messages: IncomingMessage[], messageType: string) {
  const filteredMessages = useMessagesByType(messages, messageType);
  return filteredMessages[filteredMessages.length - 1] || null;
}

// Custom hook for connection status
export function useConnectionStatus(client: SwitchboardStudent | SwitchboardTeacher | null) {
  const [status, setStatus] = useState({
    connected: false,
    uptime: 0,
    messageCount: 0
  });

  useEffect(() => {
    if (!client) return;

    const updateStatus = () => {
      const clientStatus = client.getStatus();
      setStatus({
        connected: clientStatus.connected,
        uptime: clientStatus.uptime_seconds,
        messageCount: clientStatus.message_count
      });
    };

    // Update immediately
    updateStatus();

    // Update every second while connected
    const interval = setInterval(updateStatus, 1000);

    return () => clearInterval(interval);
  }, [client]);

  return status;
}