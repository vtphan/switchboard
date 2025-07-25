#!/usr/bin/env node

// Debug test to verify hint flow
const TeacherSwitchboardClient = require('./teacher-client/switchboard-client.js');

async function testHintFlow() {
  console.log('🧪 Testing hint flow...');
  
  const client = new TeacherSwitchboardClient();
  
  // Set up event handlers
  client.onHint((hint) => {
    console.log('✅ HINT RECEIVED:', hint);
  });
  
  client.onExpertConnectionChange((message) => {
    console.log('✅ EXPERT CONNECTION:', message);
  });
  
  try {
    // Create session
    console.log('Creating session...');
    const session = await client.createSession('Debug Test Session');
    console.log('Session created:', session.id);
    
    // Connect to session
    console.log('Connecting to session...');
    await client.connectToSession(session.id);
    console.log('Connected to session');
    
    // Wait a bit for experts to connect
    console.log('Waiting for expert connections...');
    await new Promise(resolve => setTimeout(resolve, 5000));
    
    // Broadcast a problem
    console.log('Broadcasting test problem...');
    const problemData = {
      problem: 'I have a simple JavaScript function that should add two numbers but it\'s not working correctly.',
      code: 'function add(a, b) { return a + b; }',
      timeOnTask: 10,
      remainingTime: 20,
      frustrationLevel: 3
    };
    
    client.broadcastProblem(problemData);
    console.log('Problem broadcasted, waiting for hints...');
    
    // Wait for hints
    await new Promise(resolve => setTimeout(resolve, 10000));
    
    // Clean up
    console.log('Ending session...');
    await client.endSession();
    await client.disconnect();
    
    console.log('✅ Test completed');
    
  } catch (error) {
    console.error('❌ Test failed:', error);
  }
  
  process.exit(0);
}

testHintFlow();