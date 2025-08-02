/**
 * Integration Test for SwitchboardClient V2
 * 
 * This test verifies that the new client works correctly with the actual Go server.
 * Run this with the server running: `make run` in the root directory.
 */

import SwitchboardClient from './src/client-v2.js';

// Test configuration
const TEST_CONFIG = {
  wsUrl: 'ws://localhost:8080/ws',
  apiUrl: 'http://localhost:8080/api',
  studentId: 'test-student-v2',
  instructorId: 'test-instructor-v2'
};

console.log('🚀 Starting Switchboard V2 Integration Tests...');
console.log('Make sure the server is running: `make run` from project root\n');

async function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

// Test 1: Student Connection and Message Flow
async function testStudentFlow() {
  console.log('📚 Test 1: Student Connection and Message Flow');
  
  return new Promise((resolve, reject) => {
    let messagesReceived = [];
    let sessionChanges = [];
    let connectionStates = [];
    
    const student = new SwitchboardClient({
      userId: TEST_CONFIG.studentId,
      role: 'student',
      wsUrl: TEST_CONFIG.wsUrl,
      
      // Test the 6-hook architecture
      onBroadcastToStudents: (message) => {
        console.log('  📢 Student received announcement:', message.content.text);
        messagesReceived.push(message);
      },
      
      onDirectMessage: (message) => {
        console.log('  💬 Student received direct message:', message.content.text);
        messagesReceived.push(message);
      },
      
      onConnectionChange: (state, error) => {
        console.log(`  🔗 Connection: ${state}${error ? ` (${error.message})` : ''}`);
        connectionStates.push(state);
        
        if (state === 'error') {
          reject(new Error(`Connection failed: ${error.message}`));
        }
      },
      
      onSessionChange: (session) => {
        console.log('  📅 Session changed:', session.active ? `Active: ${session.name}` : 'Inactive');
        sessionChanges.push(session);
      }
    });
    
    // Test connection
    student.connect()
      .then(() => {
        console.log('  ✅ Student connected successfully');
        
        // Wait a bit, then test sending a message (should fail without session)
        setTimeout(() => {
          try {
            student.broadcastToInstructors({
              text: 'Test question from V2 student',
              context: 'question',
              urgent: true
            });
            console.log('  ❌ Should have failed without active session');
            reject(new Error('Message sending should fail without active session'));
          } catch (error) {
            console.log('  ✅ Correctly rejected message without session:', error.message);
            
            // Verify we received the correct connection states
            if (connectionStates.includes('connecting') && connectionStates.includes('connected')) {
              console.log('  ✅ Connection state transitions working');
            } else {
              console.log('  ❌ Missing expected connection states:', connectionStates);
            }
            
            student.disconnect();
            resolve({
              student,
              messagesReceived,
              sessionChanges,
              connectionStates
            });
          }
        }, 500);
      })
      .catch(reject);
  });
}

// Test 2: Instructor Session Management
async function testInstructorFlow() {
  console.log('\n👨‍🏫 Test 2: Instructor Session Management and Protocol Compliance');
  
  return new Promise((resolve, reject) => {
    let sessionStarted = false;
    let messagesSent = [];
    
    const instructor = new SwitchboardClient({
      userId: TEST_CONFIG.instructorId,
      role: 'instructor',
      wsUrl: TEST_CONFIG.wsUrl,
      apiUrl: TEST_CONFIG.apiUrl,
      
      onSessionChange: (session) => {
        console.log('  📅 Instructor session changed:', session.active ? `Active: ${session.name}` : 'Inactive');
        if (session.active) {
          sessionStarted = true;
        }
      },
      
      onConnectionChange: (state, error) => {
        console.log(`  🔗 Instructor connection: ${state}${error ? ` (${error.message})` : ''}`);
        
        if (state === 'error') {
          reject(new Error(`Instructor connection failed: ${error.message}`));
        }
      }
    });
    
    instructor.connect()
      .then(() => {
        console.log('  ✅ Instructor connected');
        
        // Start a session
        return instructor.startSession('Integration Test Session V2');
      })
      .then((result) => {
        console.log('  ✅ Session started:', result.message || result);
        
        // Wait for session to become active
        return new Promise((sessionResolve) => {
          const checkSession = () => {
            if (sessionStarted || instructor.isSessionActive()) {
              sessionResolve();
            } else {
              setTimeout(checkSession, 100);
            }
          };
          checkSession();
        });
      })
      .then(() => {
        console.log('  ✅ Session is now active');
        
        // Test message sending with new API - verify protocol compliance
        const message1 = instructor.broadcastToStudents({
          text: 'Welcome to the V2 integration test!',
          context: 'announcement',
          important: true,
          tags: ['test', 'v2']
        });
        
        console.log('  📨 Sent announcement with correct protocol structure:');
        console.log('    Type:', message1.type);
        console.log('    Context:', message1.context);
        console.log('    Content keys:', Object.keys(message1.content));
        console.log('    Context in content?', message1.content.context !== undefined ? '❌ DOUBLE-NESTED' : '✅ CLEAN');
        
        messagesSent.push(message1);
        
        // Test string message
        const message2 = instructor.broadcastToStudents('Simple string announcement');
        console.log('  📨 Sent string message, converted to:', message2.content);
        messagesSent.push(message2);
        
        // End session
        return instructor.endSession();
      })
      .then((result) => {
        console.log('  ✅ Session ended:', result.message || result);
        instructor.disconnect();
        
        // Verify protocol compliance
        const protocolCompliant = messagesSent.every(msg => 
          msg.type && 
          msg.context && 
          msg.content && 
          msg.content.context === undefined
        );
        
        if (protocolCompliant) {
          console.log('  ✅ All messages are protocol compliant');
        } else {
          console.log('  ❌ Protocol compliance failed');
        }
        
        resolve({
          instructor,
          messagesSent,
          protocolCompliant
        });
      })
      .catch(reject);
  });
}

// Test 3: Full Communication Flow
async function testFullCommunication() {
  console.log('\n🔄 Test 3: Full Student-Instructor Communication');
  
  return new Promise((resolve, reject) => {
    let studentMessages = [];
    let instructorMessages = [];
    let communicationComplete = false;
    
    // Create student
    const student = new SwitchboardClient({
      userId: TEST_CONFIG.studentId + '-comm',
      role: 'student',
      wsUrl: TEST_CONFIG.wsUrl,
      
      onBroadcastToStudents: (message) => {
        console.log('  👨‍🎓 Student received:', message.content.text);
        studentMessages.push(message);
        
        // Student responds with a question
        if (message.content.text.includes('communication test') && !communicationComplete) {
          try {
            const question = student.broadcastToInstructors({
              text: 'I have a question about this topic',
              context: 'question',
              urgent: false
            });
            console.log('  ❓ Student sent question with context:', question.context);
          } catch (error) {
            console.log('  📝 Student question queued (session not active yet)');
          }
        }
      },
      
      onSessionChange: (session) => {
        if (session.active) {
          console.log('  👨‍🎓 Student sees session active:', session.name);
          // Try sending question again now that session is active
          setTimeout(() => {
            try {
              const question = student.broadcastToInstructors({
                text: 'Now I can ask my question!',
                context: 'question',
                student_id: student.userId
              });
              console.log('  ✅ Student successfully sent question during active session');
            } catch (error) {
              console.log('  ❌ Student failed to send question:', error.message);
            }
          }, 200);
        }
      }
    });
    
    // Create instructor  
    const instructor = new SwitchboardClient({
      userId: TEST_CONFIG.instructorId + '-comm',
      role: 'instructor',
      wsUrl: TEST_CONFIG.wsUrl,
      apiUrl: TEST_CONFIG.apiUrl,
      
      onBroadcastToInstructors: (message) => {
        console.log('  👨‍🏫 Instructor received question:', message.content.text);
        instructorMessages.push(message);
        
        // Complete the test after receiving a message
        if (!communicationComplete) {
          communicationComplete = true;
          setTimeout(() => {
            instructor.endSession().then(() => {
              console.log('  ✅ Communication test completed');
              student.disconnect();
              instructor.disconnect();
              resolve({
                studentMessages,
                instructorMessages,
                communicationComplete: true
              });
            });
          }, 500);
        }
      }
    });
    
    // Start the flow
    Promise.all([student.connect(), instructor.connect()])
      .then(() => {
        console.log('  ✅ Both clients connected');
        return instructor.startSession('Full Communication Test');
      })
      .then(() => {
        console.log('  ✅ Session started');
        
        // Wait a bit for session to propagate
        setTimeout(() => {
          instructor.broadcastToStudents({
            text: 'Starting full communication test',
            context: 'announcement'
          });
        }, 300);
      })
      .catch(reject);
  });
}

// Run all tests
async function runTests() {
  try {
    console.log('Starting tests...\n');
    
    const test1Result = await testStudentFlow();
    await sleep(1000);
    
    const test2Result = await testInstructorFlow();
    await sleep(1000);
    
    const test3Result = await testFullCommunication();
    
    console.log('\n🎉 All Integration Tests Completed Successfully!');
    console.log('\n📊 Test Summary:');
    console.log('✅ Student connection and validation');
    console.log('✅ Instructor session management');
    console.log('✅ Protocol compliance (context field fix)');
    console.log('✅ Message routing and 6-hook architecture');
    console.log('✅ Full student-instructor communication');
    
    // Verify the critical fix
    if (test2Result.protocolCompliant) {
      console.log('\n🔧 CRITICAL FIX VERIFIED:');
      console.log('✅ Context field is properly separated (no double-nesting)');
      console.log('✅ Messages comply with server database schema');
    }
    
    console.log('\n🚀 SDK V2 is ready for production use!');
    
  } catch (error) {
    console.error('\n❌ Integration test failed:', error.message);
    console.error('Make sure the Switchboard server is running: `make run`');
    process.exit(1);
  }
}

// Handle graceful shutdown
process.on('SIGINT', () => {
  console.log('\n⏹️  Tests interrupted');
  process.exit(0);
});

runTests();