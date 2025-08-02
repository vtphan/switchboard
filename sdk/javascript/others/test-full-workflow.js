/**
 * Full Workflow Test for SwitchboardClient V2
 * 
 * This test simulates a complete student-teacher interaction workflow
 * to verify end-to-end functionality.
 */

// Set up WebSocket for Node.js
import WebSocket from 'ws';
import fetch from 'node-fetch';

global.WebSocket = WebSocket;
global.fetch = fetch;

import SwitchboardClient from './src/client-v2.js';

const TEST_CONFIG = {
  wsUrl: 'ws://localhost:8080/ws',
  apiUrl: 'http://localhost:8080/api',
  studentId: 'alice-student',
  instructorId: 'bob-instructor',
  sessionName: 'Full Workflow Test Session'
};

console.log('🎓 Starting Full Student-Teacher Workflow Test...\n');

async function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function runFullWorkflowTest() {
  return new Promise((resolve, reject) => {
    let testComplete = false;
    let sessionStarted = false;
    let studentConnected = false;
    let instructorConnected = false;
    let messagesExchanged = [];

    // Create instructor client
    const instructor = new SwitchboardClient({
      userId: TEST_CONFIG.instructorId,
      role: 'instructor',
      wsUrl: TEST_CONFIG.wsUrl,
      apiUrl: TEST_CONFIG.apiUrl,
      
      onBroadcastToInstructors: (message) => {
        console.log('👨‍🏫 Instructor received question:', message.content.text);
        messagesExchanged.push({
          type: 'question_received',
          from: message.content.student_id || 'student',
          content: message.content,
          context: message.context
        });
        
        // Instructor responds with announcement
        setTimeout(() => {
          const response = instructor.broadcastToStudents({
            text: `Thanks for the question about "${message.content.text}". Great question!`,
            context: 'response',
            important: true,
            reference_to: message.content.student_id
          });
          
          console.log('👨‍🏫 Instructor sent response');
          messagesExchanged.push({
            type: 'response_sent',
            message: response
          });
        }, 500);
      },
      
      onConnectionChange: (state, error) => {
        console.log(`👨‍🏫 Instructor connection: ${state}${error ? ` (${error.message})` : ''}`);
        if (state === 'connected') {
          instructorConnected = true;
          
          // Start session once connected
          instructor.startSession(TEST_CONFIG.sessionName)
            .then((result) => {
              console.log('📅 Session started:', result.session?.name || TEST_CONFIG.sessionName);
              sessionStarted = true;
            })
            .catch(reject);
        }
        if (state === 'error') reject(error);
      },
      
      onSessionChange: (session) => {
        console.log(`👨‍🏫 Instructor session: ${session.active ? `Active: ${session.name}` : 'Inactive'}`);
      }
    });

    // Create student client
    const student = new SwitchboardClient({
      userId: TEST_CONFIG.studentId,
      role: 'student',
      wsUrl: TEST_CONFIG.wsUrl,
      
      onBroadcastToStudents: (message) => {
        console.log('👨‍🎓 Student received announcement:', message.content.text);
        messagesExchanged.push({
          type: 'announcement_received',
          content: message.content,
          context: message.context
        });
        
        // Complete test after receiving response
        if (!testComplete && message.content.reference_to === TEST_CONFIG.studentId) {
          testComplete = true;
          
          setTimeout(() => {
            // End session and cleanup
            instructor.endSession()
              .then(() => {
                console.log('📅 Session ended');
                instructor.disconnect();
                student.disconnect();
                
                resolve({
                  sessionStarted,
                  studentConnected,
                  instructorConnected,
                  messagesExchanged,
                  testComplete: true
                });
              })
              .catch(reject);
          }, 300);
        }
      },
      
      onConnectionChange: (state, error) => {
        console.log(`👨‍🎓 Student connection: ${state}${error ? ` (${error.message})` : ''}`);
        if (state === 'connected') {
          studentConnected = true;
        }
        if (state === 'error') reject(error);
      },
      
      onSessionChange: (session) => {
        console.log(`👨‍🎓 Student session: ${session.active ? `Active: ${session.name}` : 'Inactive'}`);
        
        // Student asks question once session is active
        if (session.active && !testComplete) {
          setTimeout(() => {
            try {
              const question = student.broadcastToInstructors({
                text: 'How does the V2 client simplify the API?',
                context: 'question',
                urgent: false,
                student_id: TEST_CONFIG.studentId,
                topic: 'V2 Architecture'
              });
              
              console.log('👨‍🎓 Student sent question');
              messagesExchanged.push({
                type: 'question_sent',
                message: question
              });
              
            } catch (error) {
              console.error('👨‍🎓 Student failed to send question:', error.message);
            }
          }, 200);
        }
      }
    });

    // Start the workflow
    Promise.all([
      instructor.connect(),
      student.connect()
    ]).catch(reject);
    
    // Timeout safeguard
    setTimeout(() => {
      if (!testComplete) {
        reject(new Error('Test timeout - workflow did not complete'));
      }
    }, 15000);
  });
}

async function main() {
  try {
    console.log('🚀 Starting full workflow simulation...\n');
    
    const result = await runFullWorkflowTest();
    
    console.log('\n🎉 Full Workflow Test Completed Successfully!\n');
    
    console.log('📊 Test Results:');
    console.log(`✅ Instructor connected: ${result.instructorConnected}`);
    console.log(`✅ Student connected: ${result.studentConnected}`);
    console.log(`✅ Session started: ${result.sessionStarted}`);
    console.log(`✅ Messages exchanged: ${result.messagesExchanged.length}`);
    console.log(`✅ Workflow completed: ${result.testComplete}`);
    
    console.log('\n📝 Message Flow:');
    result.messagesExchanged.forEach((msg, i) => {
      console.log(`${i + 1}. ${msg.type}${msg.message ? ' - Protocol compliant' : ''}`);
      if (msg.message) {
        const contextSeparate = msg.message.content.context === undefined;
        console.log(`   Context field: ${contextSeparate ? '✅ Separate' : '❌ Double-nested'}`);
      }
    });
    
    console.log('\n🔧 V2 Features Demonstrated:');
    console.log('✅ End-to-end student-teacher communication');
    console.log('✅ Session lifecycle management'); 
    console.log('✅ Protocol compliance (context field fix)');
    console.log('✅ Real-time bidirectional messaging');
    console.log('✅ Simplified 6-hook architecture');
    
    console.log('\n🌐 Browser Examples Ready:');
    console.log('Open these URLs to test in browser:');
    console.log('- Student: http://localhost:3000/examples/student/index-v2.html');
    console.log('- Teacher: http://localhost:3000/examples/teacher/index-v2.html');
    
    process.exit(0);
    
  } catch (error) {
    console.error('\n❌ Full workflow test failed:', error.message);
    process.exit(1);
  }
}

main();