/**
 * React Student Component Example using Switchboard SDK
 * 
 * This example shows how to build a React component for students using
 * the Switchboard SDK React hooks.
 */

import React, { useState } from 'react';
import { useSwitchboardStudent, useLatestMessage, useConnectionStatus } from '@switchboard/sdk';

function StudentDashboard({ userId, sessionId }) {
  const {
    client,
    connected,
    session,
    messages,
    connect,
    disconnect,
    error
  } = useSwitchboardStudent(userId, {
    serverUrl: 'http://localhost:8080',
    autoConnect: !!sessionId,
    sessionId
  });

  const [questionText, setQuestionText] = useState('');
  const [urgency, setUrgency] = useState('medium');
  const [showProgress, setShowProgress] = useState(false);
  const [progressData, setProgressData] = useState({
    completion: 0,
    timeSpent: 0,
    topic: ''
  });

  // Get latest messages by type
  const latestResponse = useLatestMessage(messages, 'inbox_response');
  const latestBroadcast = useLatestMessage(messages, 'instructor_broadcast');
  const latestRequest = useLatestMessage(messages, 'request');

  // Get connection status with live updates
  const connectionStatus = useConnectionStatus(client);

  const handleAskQuestion = async () => {
    if (!client || !questionText.trim()) return;

    try {
      await client.askQuestion({
        text: questionText,
        urgency: urgency,
        timestamp: new Date().toISOString()
      }, 'question');

      setQuestionText('');
      console.log('Question sent successfully');
    } catch (err) {
      console.error('Failed to send question:', err);
    }
  };

  const handleReportProgress = async () => {
    if (!client) return;

    try {
      await client.reportProgress({
        completionPercentage: progressData.completion,
        timeSpentMinutes: progressData.timeSpent,
        currentTopic: progressData.topic
      });

      console.log('Progress reported successfully');
      setShowProgress(false);
    } catch (err) {
      console.error('Failed to report progress:', err);
    }
  };

  const handleRespondToRequest = async (requestMessage) => {
    const response = prompt('Enter your response:');
    if (!response || !client) return;

    try {
      await client.respondToRequest({
        text: response,
        original_request: requestMessage.content.text,
        timestamp: new Date().toISOString()
      }, requestMessage.context);

      console.log('Response sent successfully');
    } catch (err) {
      console.error('Failed to send response:', err);
    }
  };

  const handleReportError = async () => {
    const errorDescription = prompt('Describe the error you encountered:');
    if (!errorDescription || !client) return;

    try {
      await client.reportError({
        errorType: 'user_reported',
        errorMessage: errorDescription,
        timeStuckMinutes: 5,
        attemptedFixes: 1
      });

      console.log('Error reported successfully');
    } catch (err) {
      console.error('Failed to report error:', err);
    }
  };

  if (error) {
    return (
      <div className="error-container">
        <h2>Connection Error</h2>
        <p>{error.message}</p>
        <button onClick={() => window.location.reload()}>
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="student-dashboard">
      <header className="dashboard-header">
        <h1>Student Dashboard</h1>
        <div className="connection-status">
          <span className={`status-indicator ${connected ? 'connected' : 'disconnected'}`}>
            {connected ? '🟢' : '🔴'}
          </span>
          <span>
            {connected ? 'Connected' : 'Disconnected'}
            {session && ` - ${session.name}`}
          </span>
        </div>
        
        {connected && (
          <div className="session-info">
            <small>
              Uptime: {Math.floor(connectionStatus.uptime / 60)}m {connectionStatus.uptime % 60}s | 
              Messages: {connectionStatus.messageCount}
            </small>
          </div>
        )}
      </header>

      {!connected && !sessionId && (
        <div className="connection-prompt">
          <p>Enter a session ID to connect:</p>
          <input 
            type="text" 
            placeholder="Session ID"
            onKeyPress={(e) => {
              if (e.key === 'Enter') {
                connect(e.target.value);
              }
            }}
          />
        </div>
      )}

      {connected && (
        <div className="dashboard-content">
          {/* Ask Question Section */}
          <section className="question-section">
            <h3>Ask a Question</h3>
            <div className="question-form">
              <textarea
                value={questionText}
                onChange={(e) => setQuestionText(e.target.value)}
                placeholder="What would you like to ask your instructor?"
                rows={3}
              />
              <div className="question-options">
                <select 
                  value={urgency} 
                  onChange={(e) => setUrgency(e.target.value)}
                >
                  <option value="low">Low Priority</option>
                  <option value="medium">Medium Priority</option>
                  <option value="high">High Priority</option>
                </select>
                <button 
                  onClick={handleAskQuestion}
                  disabled={!questionText.trim()}
                >
                  Send Question
                </button>
              </div>
            </div>
          </section>

          {/* Quick Actions */}
          <section className="quick-actions">
            <h3>Quick Actions</h3>
            <div className="action-buttons">
              <button onClick={() => setShowProgress(true)}>
                Report Progress
              </button>
              <button onClick={handleReportError}>
                Report Error
              </button>
              <button onClick={() => {
                if (client) {
                  client.reportEngagement({
                    attentionLevel: 'high',
                    confusionLevel: 'low',
                    participationScore: 85
                  });
                }
              }}>
                Send Engagement Data
              </button>
            </div>
          </section>

          {/* Progress Report Modal */}
          {showProgress && (
            <div className="modal-overlay">
              <div className="modal">
                <h3>Report Progress</h3>
                <div className="progress-form">
                  <label>
                    Completion Percentage:
                    <input
                      type="range"
                      min="0"
                      max="100"
                      value={progressData.completion}
                      onChange={(e) => setProgressData({
                        ...progressData,
                        completion: parseInt(e.target.value)
                      })}
                    />
                    <span>{progressData.completion}%</span>
                  </label>
                  
                  <label>
                    Time Spent (minutes):
                    <input
                      type="number"
                      value={progressData.timeSpent}
                      onChange={(e) => setProgressData({
                        ...progressData,
                        timeSpent: parseInt(e.target.value) || 0
                      })}
                    />
                  </label>
                  
                  <label>
                    Current Topic:
                    <input
                      type="text"
                      value={progressData.topic}
                      onChange={(e) => setProgressData({
                        ...progressData,
                        topic: e.target.value
                      })}
                      placeholder="What are you working on?"
                    />
                  </label>
                  
                  <div className="modal-buttons">
                    <button onClick={handleReportProgress}>
                      Send Progress
                    </button>
                    <button onClick={() => setShowProgress(false)}>
                      Cancel
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Latest Instructor Response */}
          {latestResponse && (
            <section className="latest-response">
              <h3>Latest Response from Instructor</h3>
              <div className="message-card response">
                <div className="message-header">
                  <strong>{latestResponse.from_user}</strong>
                  <span className="timestamp">
                    {new Date(latestResponse.timestamp).toLocaleTimeString()}
                  </span>
                </div>
                <div className="message-content">
                  {latestResponse.content.text}
                  {latestResponse.content.code_example && (
                    <pre className="code-example">
                      {latestResponse.content.code_example}
                    </pre>
                  )}
                </div>
              </div>
            </section>
          )}

          {/* Pending Request */}
          {latestRequest && (
            <section className="pending-request">
              <h3>Request from Instructor</h3>
              <div className="message-card request">
                <div className="message-header">
                  <strong>{latestRequest.from_user}</strong>
                  <span className="timestamp">
                    {new Date(latestRequest.timestamp).toLocaleTimeString()}
                  </span>
                </div>
                <div className="message-content">
                  {latestRequest.content.text}
                  {latestRequest.content.requirements && (
                    <ul className="requirements">
                      {latestRequest.content.requirements.map((req, index) => (
                        <li key={index}>{req}</li>
                      ))}
                    </ul>
                  )}
                </div>
                <button 
                  className="respond-button"
                  onClick={() => handleRespondToRequest(latestRequest)}
                >
                  Respond
                </button>
              </div>
            </section>
          )}

          {/* Latest Announcement */}
          {latestBroadcast && (
            <section className="latest-announcement">
              <h3>Latest Announcement</h3>
              <div className="message-card announcement">
                <div className="message-header">
                  <strong>📢 {latestBroadcast.from_user}</strong>
                  <span className="timestamp">
                    {new Date(latestBroadcast.timestamp).toLocaleTimeString()}
                  </span>
                </div>
                <div className="message-content">
                  {latestBroadcast.content.text}
                  {latestBroadcast.content.break_duration && (
                    <div className="break-info">
                      Break Duration: {Math.floor(latestBroadcast.content.break_duration / 60)} minutes
                    </div>
                  )}
                </div>
              </div>
            </section>
          )}

          {/* Message History */}
          <section className="message-history">
            <h3>Message History ({messages.length})</h3>
            <div className="messages-list">
              {messages.slice(-10).reverse().map((message, index) => (
                <div key={index} className={`message-item ${message.type}`}>
                  <div className="message-meta">
                    <span className="message-type">{message.type}</span>
                    <span className="message-time">
                      {new Date(message.timestamp).toLocaleTimeString()}
                    </span>
                  </div>
                  <div className="message-preview">
                    {message.content.text || JSON.stringify(message.content).substring(0, 100)}
                  </div>
                </div>
              ))}
            </div>
          </section>
        </div>
      )}

      <style jsx>{`
        .student-dashboard {
          max-width: 800px;
          margin: 0 auto;
          padding: 20px;
          font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        }

        .dashboard-header {
          text-align: center;
          margin-bottom: 30px;
          padding-bottom: 20px;
          border-bottom: 1px solid #eee;
        }

        .connection-status {
          display: flex;
          align-items: center;
          justify-content: center;
          gap: 10px;
          margin: 10px 0;
        }

        .status-indicator {
          font-size: 12px;
        }

        .session-info {
          color: #666;
          font-size: 12px;
        }

        .dashboard-content {
          display: flex;
          flex-direction: column;
          gap: 20px;
        }

        section {
          background: #f9f9f9;
          padding: 20px;
          border-radius: 8px;
          border: 1px solid #e0e0e0;
        }

        .question-form textarea {
          width: 100%;
          padding: 10px;
          border: 1px solid #ddd;
          border-radius: 4px;
          margin-bottom: 10px;
        }

        .question-options {
          display: flex;
          gap: 10px;
          align-items: center;
        }

        .action-buttons {
          display: flex;
          gap: 10px;
          flex-wrap: wrap;
        }

        button {
          padding: 10px 20px;
          background: #007bff;
          color: white;
          border: none;
          border-radius: 4px;
          cursor: pointer;
        }

        button:hover {
          background: #0056b3;
        }

        button:disabled {
          background: #ccc;
          cursor: not-allowed;
        }

        .modal-overlay {
          position: fixed;
          top: 0;
          left: 0;
          right: 0;
          bottom: 0;
          background: rgba(0, 0, 0, 0.5);
          display: flex;
          align-items: center;
          justify-content: center;
          z-index: 1000;
        }

        .modal {
          background: white;
          padding: 20px;
          border-radius: 8px;
          max-width: 400px;
          width: 90%;
        }

        .progress-form label {
          display: block;
          margin-bottom: 15px;
        }

        .progress-form input {
          display: block;
          width: 100%;
          margin-top: 5px;
          padding: 5px;
        }

        .modal-buttons {
          display: flex;
          gap: 10px;
          margin-top: 20px;
        }

        .message-card {
          background: white;
          padding: 15px;
          border-radius: 8px;
          border-left: 4px solid #007bff;
        }

        .message-card.response {
          border-left-color: #28a745;
        }

        .message-card.request {
          border-left-color: #ffc107;
        }

        .message-card.announcement {
          border-left-color: #17a2b8;
        }

        .message-header {
          display: flex;
          justify-content: space-between;
          margin-bottom: 10px;
        }

        .timestamp {
          color: #666;
          font-size: 12px;
        }

        .code-example {
          background: #f8f9fa;
          padding: 10px;
          border-radius: 4px;
          margin-top: 10px;
          overflow-x: auto;
        }

        .requirements {
          margin-top: 10px;
          color: #666;
        }

        .respond-button {
          margin-top: 10px;
          background: #ffc107;
          color: #000;
        }

        .messages-list {
          max-height: 300px;
          overflow-y: auto;
        }

        .message-item {
          padding: 10px;
          margin-bottom: 10px;
          background: white;
          border-radius: 4px;
          border-left: 3px solid #ddd;
        }

        .message-meta {
          display: flex;
          justify-content: space-between;
          font-size: 12px;
          color: #666;
          margin-bottom: 5px;
        }

        .message-type {
          font-weight: bold;
          text-transform: uppercase;
        }

        .error-container {
          text-align: center;
          padding: 40px;
          background: #f8d7da;
          border: 1px solid #f5c6cb;
          border-radius: 8px;
          color: #721c24;
        }
      `}</style>
    </div>
  );
}

export default StudentDashboard;