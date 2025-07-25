#!/usr/bin/env node

/**
 * Teacher Dashboard Example using Switchboard JavaScript SDK
 * 
 * This example shows how to create a teacher application that can:
 * - Create and manage sessions
 * - Send broadcasts and requests to students
 * - Handle student questions and analytics
 * 
 * Usage:
 *   node javascript-teacher-app.js
 */

const { SwitchboardTeacher } = require('../javascript/dist/index.js');
const readline = require('readline');

class TeacherDashboard {
  constructor(teacherId) {
    this.teacherId = teacherId;
    this.teacher = new SwitchboardTeacher(teacherId);
    this.currentSession = null;
    
    // Set up readline interface for interactive commands
    this.rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout
    });
    
    this.setupEventHandlers();
  }
  
  setupEventHandlers() {
    this.teacher.setupEventHandlers({
      onStudentQuestion: this.handleStudentQuestion.bind(this),
      onStudentResponse: this.handleStudentResponse.bind(this),
      onStudentAnalytics: this.handleStudentAnalytics.bind(this),
      onConnection: this.handleConnectionChange.bind(this),
      onError: this.handleError.bind(this)
    });
  }
  
  async handleStudentQuestion(message) {
    console.log(`\n💬 Question from ${message.from_user}:`);
    console.log(`   "${message.content.text}"`);
    
    if (message.content.urgency === 'high') {
      console.log(`   ⚠️  HIGH URGENCY!`);
    }
    
    if (message.content.code_context) {
      console.log(`   Code context: ${message.content.code_context}`);
    }
    
    // Auto-respond with acknowledgment
    await this.teacher.respondToStudent(message.from_user, {
      text: "I received your question and will respond shortly!",
      auto_response: true
    }, 'acknowledgment');
  }
  
  handleStudentResponse(message) {
    console.log(`\n📝 Response from ${message.from_user}:`);
    console.log(`   "${message.content.text || JSON.stringify(message.content)}"`);
  }
  
  handleStudentAnalytics(message) {
    const { content, from_user } = message;
    
    if (message.context === 'progress') {
      console.log(`\n📊 Progress from ${from_user}: ${content.completion_percentage}% complete`);
    } else if (message.context === 'error') {
      console.log(`\n❌ Error reported by ${from_user}: ${content.error_message}`);
    } else if (message.context === 'engagement') {
      console.log(`\n🎯 Engagement from ${from_user}: ${content.attention_level} attention, ${content.confusion_level} confusion`);
    }
  }
  
  handleConnectionChange(connected) {
    if (connected) {
      console.log('🟢 Connected to session');
    } else {
      console.log('🔴 Disconnected from session');
    }
  }
  
  handleError(error) {
    console.error(`❌ Error: ${error.message}`);
  }
  
  async start() {
    console.log('🎓 Switchboard Teacher Dashboard');
    console.log('================================');
    console.log(`Teacher ID: ${this.teacherId}`);
    console.log('');
    
    await this.showMainMenu();
  }
  
  async showMainMenu() {
    if (this.currentSession) {
      await this.showSessionMenu();
    } else {
      console.log('Main Menu:');
      console.log('1. Create new session');
      console.log('2. List active sessions');
      console.log('3. Connect to existing session');
      console.log('4. Exit');
      
      const choice = await this.prompt('Choose an option (1-4): ');
      
      switch (choice) {
        case '1':
          await this.createSession();
          break;
        case '2':
          await this.listSessions();
          break;
        case '3':
          await this.connectToSession();
          break;
        case '4':
          process.exit(0);
          break;
        default:
          console.log('Invalid choice');
          await this.showMainMenu();
      }
    }
  }
  
  async showSessionMenu() {
    console.log(`\nSession: ${this.currentSession.name} (${this.currentSession.student_ids.length} students)`);
    console.log('Session Menu:');
    console.log('1. Make announcement');
    console.log('2. Give instruction');
    console.log('3. Request code from student');
    console.log('4. Broadcast problem (for AI tutors)');
    console.log('5. Schedule break');
    console.log('6. View session status');
    console.log('7. End session');
    console.log('8. Disconnect');
    
    const choice = await this.prompt('Choose an option (1-8): ');
    
    switch (choice) {
      case '1':
        await this.makeAnnouncement();
        break;
      case '2':
        await this.giveInstruction();
        break;
      case '3':
        await this.requestCode();
        break;
      case '4':
        await this.broadcastProblem();
        break;
      case '5':
        await this.scheduleBreak();
        break;
      case '6':
        await this.showSessionStatus();
        break;
      case '7':
        await this.endSession();
        return;
      case '8':
        await this.disconnectSession();
        return;
      default:
        console.log('Invalid choice');
    }
    
    await this.showSessionMenu();
  }
  
  async createSession() {
    console.log('\nCreate New Session');
    const name = await this.prompt('Session name: ');
    const studentIdsInput = await this.prompt('Student IDs (comma-separated): ');
    const studentIds = studentIdsInput.split(',').map(id => id.trim()).filter(id => id);
    
    try {
      const session = await this.teacher.createAndConnect(name, studentIds);
      this.currentSession = session;
      
      console.log(`✅ Created and connected to session: ${session.name}`);
      console.log(`📊 Session ID: ${session.id}`);
      console.log(`👥 Students: ${studentIds.join(', ')}`);
      
      // Send welcome message
      await this.teacher.announce(`Welcome to ${session.name}! I'm your instructor and ready to help.`);
      
    } catch (error) {
      console.error(`❌ Failed to create session: ${error.message}`);
    }
    
    await this.showMainMenu();
  }
  
  async listSessions() {
    try {
      const sessions = await this.teacher.listActiveSessions();
      
      if (sessions.length === 0) {
        console.log('\nNo active sessions found.');
      } else {
        console.log('\nActive Sessions:');
        sessions.forEach((session, index) => {
          console.log(`${index + 1}. ${session.name} (${session.student_ids.length} students)`);
          console.log(`   ID: ${session.id}`);
          console.log(`   Created: ${new Date(session.start_time).toLocaleString()}`);
        });
      }
    } catch (error) {
      console.error(`❌ Failed to list sessions: ${error.message}`);
    }
    
    await this.showMainMenu();
  }
  
  async connectToSession() {
    const sessionId = await this.prompt('Session ID: ');
    
    try {
      await this.teacher.connect(sessionId);
      const session = await this.teacher.getSession(sessionId);
      this.currentSession = session;
      
      console.log(`✅ Connected to session: ${session.name}`);
      
    } catch (error) {
      console.error(`❌ Failed to connect: ${error.message}`);
    }
    
    await this.showMainMenu();
  }
  
  async makeAnnouncement() {
    const text = await this.prompt('Announcement text: ');
    
    try {
      await this.teacher.announce(text);
      console.log('✅ Announcement sent');
    } catch (error) {
      console.error(`❌ Failed to send announcement: ${error.message}`);
    }
  }
  
  async giveInstruction() {
    const instruction = await this.prompt('Instruction text: ');
    
    try {
      await this.teacher.giveInstruction(instruction);
      console.log('✅ Instruction sent');
    } catch (error) {
      console.error(`❌ Failed to send instruction: ${error.message}`);
    }
  }
  
  async requestCode() {
    const studentId = await this.prompt('Student ID: ');
    const prompt = await this.prompt('Request prompt: ');
    const requirements = await this.prompt('Requirements (comma-separated, optional): ');
    
    try {
      await this.teacher.requestCodeFromStudent({
        studentId,
        prompt,
        requirements: requirements ? requirements.split(',').map(r => r.trim()) : []
      });
      console.log(`✅ Code request sent to ${studentId}`);
    } catch (error) {
      console.error(`❌ Failed to send request: ${error.message}`);
    }
  }
  
  async broadcastProblem() {
    const problem = await this.prompt('Problem description: ');
    const code = await this.prompt('Code context (optional): ');
    const frustrationInput = await this.prompt('Frustration level (1-5, default 2): ');
    const frustrationLevel = parseInt(frustrationInput) || 2;
    
    try {
      await this.teacher.broadcastProblem({
        problem,
        code,
        frustrationLevel,
        timeOnTask: 0,
        remainingTime: 30
      });
      console.log('✅ Problem broadcast sent (AI tutors will respond with hints)');
    } catch (error) {
      console.error(`❌ Failed to broadcast problem: ${error.message}`);
    }
  }
  
  async scheduleBreak() {
    const durationInput = await this.prompt('Break duration (minutes): ');
    const duration = parseInt(durationInput) || 10;
    const resumeTime = await this.prompt('Resume time (optional): ');
    
    try {
      await this.teacher.scheduleBreak({
        durationMinutes: duration,
        resumeTime: resumeTime || undefined,
        instructions: 'Save your work and take a break!'
      });
      console.log(`✅ Break scheduled for ${duration} minutes`);
    } catch (error) {
      console.error(`❌ Failed to schedule break: ${error.message}`);
    }
  }
  
  async showSessionStatus() {
    const status = this.teacher.getStatus();
    
    console.log('\nSession Status:');
    console.log(`Connected: ${status.connected}`);
    console.log(`Session ID: ${status.session_id}`);
    console.log(`Uptime: ${status.uptime_seconds} seconds`);
    console.log(`Messages sent: ${status.message_count}`);
    console.log(`Students enrolled: ${this.currentSession.student_ids.length}`);
    console.log(`Students: ${this.currentSession.student_ids.join(', ')}`);
  }
  
  async endSession() {
    const confirm = await this.prompt('Are you sure you want to end this session? (y/n): ');
    
    if (confirm.toLowerCase() === 'y') {
      try {
        await this.teacher.announce('Class is ending. Thank you for participating!');
        await new Promise(resolve => setTimeout(resolve, 2000)); // Give time for message to send
        
        await this.teacher.endCurrentSession();
        this.currentSession = null;
        
        console.log('✅ Session ended successfully');
      } catch (error) {
        console.error(`❌ Failed to end session: ${error.message}`);
      }
    }
  }
  
  async disconnectSession() {
    try {
      await this.teacher.disconnect();
      this.currentSession = null;
      console.log('✅ Disconnected from session');
    } catch (error) {
      console.error(`❌ Failed to disconnect: ${error.message}`);
    }
  }
  
  prompt(question) {
    return new Promise(resolve => {
      this.rl.question(question, resolve);
    });
  }
  
  async stop() {
    if (this.teacher.connected) {
      await this.teacher.disconnect();
    }
    this.rl.close();
  }
}

// Main execution
async function main() {
  const teacherId = process.argv[2] || 'teacher_001';
  const dashboard = new TeacherDashboard(teacherId);
  
  // Handle graceful shutdown
  process.on('SIGINT', async () => {
    console.log('\n\n🛑 Shutting down...');
    await dashboard.stop();
    process.exit(0);
  });
  
  try {
    await dashboard.start();
  } catch (error) {
    console.error('❌ Application error:', error);
    await dashboard.stop();
    process.exit(1);
  }
}

main().catch(console.error);