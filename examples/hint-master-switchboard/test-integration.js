#!/usr/bin/env node

// Integration Test for Switchboard AI Programming Mentorship
// Tests the basic flow: session creation, connection, and message routing

const chalk = require('chalk');

async function testIntegration() {
  console.log(chalk.blue('🧪 Testing Switchboard AI Programming Mentorship Integration'));
  console.log(chalk.gray('═'.repeat(60)));

  let testsPassed = 0;
  let testsTotal = 0;

  const test = (name, result) => {
    testsTotal++;
    if (result) {
      testsPassed++;
      console.log(chalk.green(`✅ ${name}`));
    } else {
      console.log(chalk.red(`❌ ${name}`));
    }
  };

  try {
    // Test 1: Switchboard server is running
    console.log(chalk.yellow('Testing Switchboard connectivity...'));
    const healthResponse = await fetch('http://localhost:8080/health');
    test('Switchboard server is running', healthResponse.ok);

    if (!healthResponse.ok) {
      console.log(chalk.red('\n❌ Switchboard server is not running. Please start it first:'));
      console.log(chalk.gray('   cd /path/to/switchboard && make run'));
      process.exit(1);
    }

    // Test 2: Can create a session
    console.log(chalk.yellow('Testing session creation...'));
    const sessionData = {
      name: 'Test AI Mentorship Session',
      instructor_id: 'test_teacher',
      student_ids: [
        'technical_expert',
        'emotional_support_coach', 
        'debugging_guru',
        'learning_coach',
        'architecture_expert'
      ]
    };

    const createResponse = await fetch('http://localhost:8080/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(sessionData)
    });

    const session = createResponse.ok ? await createResponse.json() : null;
    test('Can create session with AI experts', createResponse.ok && session?.session?.id);

    if (!session?.session?.id) {
      console.log(chalk.red('❌ Failed to create session'));
      process.exit(1);
    }

    const sessionId = session.session.id;
    console.log(chalk.gray(`   Session ID: ${sessionId.substring(0, 8)}...`));

    // Test 3: Can retrieve session
    console.log(chalk.yellow('Testing session retrieval...'));
    const getResponse = await fetch(`http://localhost:8080/api/sessions/${sessionId}`);
    const retrievedSession = getResponse.ok ? await getResponse.json() : null;
    test('Can retrieve session details', getResponse.ok && retrievedSession?.session?.student_ids?.length === 5);

    // Test 4: Session contains all expert IDs
    const expertIds = retrievedSession?.session?.student_ids || [];
    const expectedExperts = ['technical_expert', 'emotional_support_coach', 'debugging_guru', 'learning_coach', 'architecture_expert'];
    const hasAllExperts = expectedExperts.every(id => expertIds.includes(id));
    test('Session contains all 5 AI expert IDs', hasAllExperts);

    // Test 5: Can list sessions
    console.log(chalk.yellow('Testing session listing...'));
    const listResponse = await fetch('http://localhost:8080/api/sessions');
    const sessionsList = listResponse.ok ? await listResponse.json() : null;
    const hasOurSession = sessionsList?.sessions?.some(s => s.id === sessionId);
    test('Can list sessions and find created session', listResponse.ok && hasOurSession);

    // Test 6: Config files exist and are valid
    console.log(chalk.yellow('Testing expert configurations...'));
    const fs = require('fs');
    const expertConfigs = [
      'student-client/experts/technical-expert.json',
      'student-client/experts/emotional-support.json', 
      'student-client/experts/debugging-guru.json',
      'student-client/experts/learning-coach.json',
      'student-client/experts/architecture-expert.json'
    ];

    let configsValid = true;
    for (const configPath of expertConfigs) {
      try {
        const configData = fs.readFileSync(configPath, 'utf8');
        const config = JSON.parse(configData);
        
        const hasRequired = config.expert_profile?.user_id && 
                           config.switchboard?.server_url && 
                           config.gemini_config?.api_key;
        
        if (!hasRequired) {
          configsValid = false;
          console.log(chalk.red(`   ❌ Invalid config: ${configPath}`));
        }
      } catch (error) {
        configsValid = false;
        console.log(chalk.red(`   ❌ Cannot read config: ${configPath}`));
      }
    }
    test('All expert config files are valid', configsValid);

    // Test 7: Teacher client files exist
    console.log(chalk.yellow('Testing teacher client files...'));
    const teacherFiles = [
      'teacher-client/index.html',
      'teacher-client/app.js',
      'teacher-client/switchboard-client.js',
      'teacher-client/server.js',
      'teacher-client/style.css'
    ];

    const teacherFilesExist = teacherFiles.every(file => {
      try {
        fs.accessSync(file);
        return true;
      } catch {
        return false;
      }
    });
    test('All teacher client files exist', teacherFilesExist);

    // Cleanup: End the test session
    console.log(chalk.yellow('Cleaning up test session...'));
    const deleteResponse = await fetch(`http://localhost:8080/api/sessions/${sessionId}`, {
      method: 'DELETE'
    });
    test('Can end session', deleteResponse.ok);

    // Final results
    console.log(chalk.gray('═'.repeat(60)));
    if (testsPassed === testsTotal) {
      console.log(chalk.green(`🎉 All ${testsTotal} integration tests passed!`));
      console.log(chalk.blue('\n🚀 Ready to use:'));
      console.log(chalk.white('1. npm run teacher        # Start teacher dashboard'));
      console.log(chalk.white('2. npm run start-all-experts  # Start AI experts'));
      console.log(chalk.white('3. Open http://localhost:3000  # Use the system'));
    } else {
      console.log(chalk.red(`❌ ${testsPassed}/${testsTotal} tests passed`));
      process.exit(1);
    }

  } catch (error) {
    console.error(chalk.red('❌ Integration test failed:'), error.message);
    process.exit(1);
  }
}

if (require.main === module) {
  testIntegration();
}