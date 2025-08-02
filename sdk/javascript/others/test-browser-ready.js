/**
 * Browser Readiness Test
 * 
 * Verifies that the browser examples are ready to test
 */

import fetch from 'node-fetch';

async function testBrowserReadiness() {
  console.log('🌐 Testing Browser Example Readiness...\n');
  
  const tests = [
    {
      name: 'Student Example HTML',
      url: 'http://localhost:3000/examples/student/index-v2.html',
      expected: 200
    },
    {
      name: 'Teacher Example HTML', 
      url: 'http://localhost:3000/examples/teacher/index-v2.html',
      expected: 200
    },
    {
      name: 'Student JavaScript',
      url: 'http://localhost:3000/examples/student/student-app-v2.js',
      expected: 200
    },
    {
      name: 'Teacher JavaScript',
      url: 'http://localhost:3000/examples/teacher/teacher-app-v2.js', 
      expected: 200
    },
    {
      name: 'V2 Client Source',
      url: 'http://localhost:3000/src/client-v2.js',
      expected: 200
    },
    {
      name: 'Shared Styles',
      url: 'http://localhost:3000/examples/shared-styles.css',
      expected: 200
    },
    {
      name: 'Switchboard API (CORS)',
      url: 'http://localhost:8080/api/session/start',
      method: 'OPTIONS',
      expected: 200
    }
  ];
  
  let allPassed = true;
  
  for (const test of tests) {
    try {
      const response = await fetch(test.url, {
        method: test.method || 'GET',
        headers: {
          'Origin': 'http://localhost:3000'
        }
      });
      
      if (response.status === test.expected) {
        console.log(`✅ ${test.name}: ${response.status}`);
      } else {
        console.log(`❌ ${test.name}: ${response.status} (expected ${test.expected})`);
        allPassed = false;
      }
    } catch (error) {
      console.log(`❌ ${test.name}: Error - ${error.message}`);
      allPassed = false;
    }
  }
  
  console.log('\n📊 Summary:');
  if (allPassed) {
    console.log('✅ All browser readiness tests passed!');
    console.log('\n🌐 Ready to test in browser:');
    console.log('- Student: http://localhost:3000/examples/student/index-v2.html');
    console.log('- Teacher: http://localhost:3000/examples/teacher/index-v2.html');
    console.log('\n📋 Testing Instructions:');
    console.log('1. Open both URLs in separate browser tabs');
    console.log('2. Teacher: Connect → Start Session');
    console.log('3. Student: Connect → Ask Question'); 
    console.log('4. Teacher: Send Announcement');
    console.log('5. Verify real-time communication works');
    console.log('6. Check protocol compliance in teacher demo panel');
  } else {
    console.log('❌ Some tests failed - check server status');
  }
}

testBrowserReadiness().catch(console.error);