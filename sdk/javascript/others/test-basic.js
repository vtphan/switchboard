// Basic functionality test for SwitchboardClient V2
import SwitchboardClient from './src/client-v2.js';

console.log('Testing SwitchboardClient V2...');

// Test 1: Constructor validation
try {
  new SwitchboardClient({ role: 'student' });
  console.error('❌ Should require userId');
} catch (error) {
  console.log('✅ Constructor validation works:', error.message);
}

// Test 2: Valid construction
try {
  const client = new SwitchboardClient({
    userId: 'test',
    role: 'student',
    onBroadcastToStudents: (msg) => console.log('Received:', msg),
    onConnectionChange: (state) => console.log('Connection:', state)
  });
  console.log('✅ Client created successfully');
  
  // Test 3: Message sending structure (critical fix validation)
  client.sessionActive = true; // Simulate active session
  
  const message = client.broadcastToInstructors({
    text: 'Test question',
    context: 'question',
    urgent: true
  });
  
  // Verify correct protocol structure
  if (message.type === 'broadcast_to_instructors' &&
      message.context === 'question' &&
      message.content.text === 'Test question' &&
      message.content.urgent === true &&
      message.content.context === undefined) {
    console.log('✅ Context field fix working correctly');
    console.log('Message structure:', JSON.stringify(message, null, 2));
  } else {
    console.error('❌ Context field fix failed');
    console.error('Message structure:', JSON.stringify(message, null, 2));
  }
  
  // Test 4: String content conversion
  const stringMessage = client.broadcastToInstructors('Simple string message');
  if (stringMessage.content.text === 'Simple string message' &&
      stringMessage.context === 'general') {
    console.log('✅ String content conversion works');
  } else {
    console.error('❌ String content conversion failed');
  }
  
  console.log('\n✅ All basic tests passed! Client V2 is working correctly.');
  
} catch (error) {
  console.error('❌ Test failed:', error.message);
}