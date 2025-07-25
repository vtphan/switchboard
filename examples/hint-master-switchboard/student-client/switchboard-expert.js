#!/usr/bin/env node

// AI Expert Switchboard Client for Programming Mentorship Demo
// Connects to Switchboard as a student instead of custom teacher WebSocket

const fs = require('fs');
const path = require('path');
const chalk = require('chalk');
const yargs = require('yargs/yargs');
const { hideBin } = require('yargs/helpers');

class ExpertSwitchboardClient {
  constructor(configPath) {
    this.configPath = configPath;
    this.config = null;
    this.ws = null;
    this.reconnectTimer = null;
    this.isConnected = false;
    this.startTime = Date.now();
    this.messageCount = 0;
    this.reconnectAttempts = 0;
    this.currentSessions = [];
    
    this.loadConfig();
    this.setupShutdown();
  }

  loadConfig() {
    try {
      const configData = fs.readFileSync(this.configPath, 'utf8');
      this.config = JSON.parse(configData);
      
      this.validateConfig();
      
      console.log(chalk.blue(`🤖 ${this.config.expert_profile.name} initialized`));
      console.log(chalk.gray(`User ID: ${this.config.expert_profile.user_id}`));
      console.log(chalk.gray(`Expertise: ${this.config.expert_profile.expertise}`));
      console.log(chalk.gray(`Switchboard URL: ${this.config.switchboard.server_url}`));
      
    } catch (error) {
      console.error(chalk.red('❌ Failed to load config:'), error.message);
      process.exit(1);
    }
  }

  validateConfig() {
    const required = [
      'expert_profile.name',
      'expert_profile.user_id',
      'expert_profile.expertise',
      'switchboard.server_url',
      'gemini_config.api_key',
      'gemini_prompt'
    ];

    for (const field of required) {
      const value = this.getNestedValue(this.config, field);
      if (!value) {
        throw new Error(`Missing required config field: ${field}`);
      }
    }

    if (this.config.gemini_config.api_key === 'your_gemini_api_key_here') {
      throw new Error('Please set your actual Gemini API key in the config file');
    }
  }

  getNestedValue(obj, path) {
    return path.split('.').reduce((current, key) => current && current[key], obj);
  }

  async start() {
    console.log(chalk.blue(`🚀 Starting ${this.config.expert_profile.name}...`));
    
    try {
      // Discover available sessions
      await this.discoverSessions();
      
      if (this.currentSessions.length === 0) {
        console.log(chalk.yellow('⚠️ No available sessions found. Waiting for sessions...'));
        // Poll for sessions every 30 seconds
        this.startSessionPolling();
      } else {
        // Connect to the first available session
        await this.connectToSession(this.currentSessions[0].id);
      }
      
      console.log(chalk.green(`✅ ${this.config.expert_profile.name} ready and waiting for problems!`));
      
      // Keep the process running
      await this.keepAlive();
      
    } catch (error) {
      console.error(chalk.red('❌ Failed to start expert client:'), error);
      process.exit(1);
    }
  }

  async discoverSessions() {
    try {
      const response = await fetch(`${this.config.switchboard.server_url}/api/sessions`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      
      const data = await response.json();
      
      // Filter sessions where this expert is enrolled
      const userId = this.config.expert_profile.user_id;
      this.currentSessions = data.sessions.filter(session => 
        session.student_ids.includes(userId) && 
        session.status === 'active'
      );
      
      console.log(chalk.cyan(`🔍 Found ${this.currentSessions.length} available sessions`));
      
      if (this.currentSessions.length > 0) {
        this.currentSessions.forEach(session => {
          console.log(chalk.gray(`  - ${session.name} (${session.id.substring(0, 8)}...)`));
        });
      }
      
    } catch (error) {
      console.error(chalk.red('❌ Failed to discover sessions:'), error.message);
      throw error;
    }
  }

  async connectToSession(sessionId) {
    try {
      console.log(chalk.yellow(`🔗 Connecting to session: ${sessionId.substring(0, 8)}...`));
      
      const wsUrl = `ws://localhost:8080/ws?user_id=${this.config.expert_profile.user_id}&role=student&session_id=${sessionId}`;
      
      // Use WebSocket from ws package (Node.js environment)
      const WebSocket = require('ws');
      this.ws = new WebSocket(wsUrl);
      
      return new Promise((resolve, reject) => {
        this.ws.on('open', () => {
          this.isConnected = true;
          this.reconnectAttempts = 0;
          
          console.log(chalk.green(`✅ Connected to session: ${sessionId.substring(0, 8)}...`));
          
          // Send connection analytics
          this.sendConnectionAnalytics('connected');
          
          resolve();
        });

        this.ws.on('message', async (data) => {
          try {
            const message = JSON.parse(data.toString());
            await this.handleMessage(message);
          } catch (error) {
            console.error(chalk.red('❌ Failed to handle message:'), error);
          }
        });

        this.ws.on('close', () => {
          this.isConnected = false;
          console.log(chalk.yellow('⚠️ Connection to session lost'));
          this.sendConnectionAnalytics('disconnected');
          this.scheduleReconnect();
        });

        this.ws.on('error', (error) => {
          console.error(chalk.red('❌ WebSocket error:'), error.message);
          if (!this.isConnected) {
            reject(error);
          }
        });

        // Connection timeout
        setTimeout(() => {
          if (!this.isConnected) {
            reject(new Error('Connection timeout'));
          }
        }, 10000);
      });
      
    } catch (error) {
      console.error(chalk.red('❌ Failed to connect to session:'), error);
      throw error;
    }
  }

  async handleMessage(message) {
    this.messageCount++;
    console.log(chalk.blue(`📨 Received message: ${message.type}`));

    switch (message.type) {
      case 'instructor_broadcast':
        if (message.context === 'problem') {
          await this.handleProblemBroadcast(message.content);
        } else if (message.context === 'status') {
          await this.handleStatusUpdate(message.content);
        }
        break;
        
      case 'inbox_response':
        await this.handleInstructorResponse(message);
        break;
        
      case 'system':
        await this.handleSystemMessage(message);
        break;
        
      case 'analytics':
        // Analytics messages are for teacher dashboard, ignore them silently
        // (Note: These shouldn't be routed to students by Switchboard, but we handle them gracefully)
        break;
        
      default:
        console.log(chalk.gray(`Unknown message type: ${message.type}`));
    }
  }

  async handleProblemBroadcast(problemData) {
    console.log(chalk.yellow(`🎯 Processing problem: "${problemData.problem.substring(0, 50)}..."`));
    console.log(chalk.gray(`Time on task: ${problemData.timeOnTask}min, Remaining: ${problemData.remainingTime}min`));
    console.log(chalk.gray(`Frustration level: ${problemData.frustrationLevel}/5`));

    try {
      // Generate hint using Gemini
      const hint = await this.generateHint(problemData);
      
      // Send hint back to instructors via Switchboard
      await this.sendHint(hint, problemData);
      
    } catch (error) {
      console.error(chalk.red('❌ Failed to generate hint:'), error.message);
      
      // Send error notification to instructors
      await this.sendErrorNotification(problemData, error);
    }
  }

  async handleStatusUpdate(statusData) {
    console.log(chalk.cyan(`📢 Status update: ${statusData.message}`));
  }

  async handleInstructorResponse(message) {
    console.log(chalk.green(`💬 Response from ${message.from_user}: ${message.content.text || 'Message received'}`));
  }

  async handleSystemMessage(message) {
    const event = message.content.event;
    
    switch (event) {
      case 'history_complete':
        console.log(chalk.gray('📚 Message history loaded'));
        break;
        
      case 'message_error':
        console.error(chalk.red('❌ Message error:'), message.content.error);
        break;
        
      case 'session_ended':
        console.log(chalk.yellow('🛑 Session ended by instructor'));
        this.isConnected = false;
        if (this.ws) {
          this.ws.close();
        }
        // Try to find another session
        setTimeout(() => this.discoverSessions(), 5000);
        break;
    }
  }

  async generateHint(problemData) {
    console.log(chalk.gray('🧠 Generating hint with Gemini...'));

    // Populate prompt template with actual data
    const prompt = this.config.gemini_prompt
      .replace(/{problem}/g, problemData.problem)
      .replace(/{code}/g, problemData.code)
      .replace(/{timeOnTask}/g, problemData.timeOnTask)
      .replace(/{remainingTime}/g, problemData.remainingTime)
      .replace(/{frustrationLevel}/g, problemData.frustrationLevel);

    console.log(chalk.gray(`📝 Using prompt template: ${this.config.expert_profile.name}`));
    console.log(chalk.gray(`📊 Frustration level: ${problemData.frustrationLevel}/5`));

    // Get Gemini configuration
    const geminiConfig = this.config.gemini_config;
    const apiKey = geminiConfig.api_key;
    const model = geminiConfig.model || 'gemini-1.5-flash';
    const maxTokens = geminiConfig.max_tokens || 150;
    const temperature = geminiConfig.temperature || 0.7;

    console.log(chalk.gray(`🤖 Using Gemini model: ${model} (max_tokens: ${maxTokens}, temp: ${temperature})`));

    // Call Gemini API
    const apiUrl = `https://generativelanguage.googleapis.com/v1beta/models/${model}:generateContent`;
    const response = await fetch(apiUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-goog-api-key': apiKey
      },
      body: JSON.stringify({
        contents: [{
          parts: [{
            text: prompt
          }]
        }],
        generationConfig: {
          maxOutputTokens: maxTokens,
          temperature: temperature
        }
      })
    });

    if (!response.ok) {
      throw new Error(`Gemini API error: ${response.status} ${response.statusText}`);
    }

    const result = await response.json();
    
    if (!result.candidates?.[0]?.content?.parts?.[0]?.text) {
      throw new Error('Invalid Gemini API response format');
    }

    const hint = result.candidates[0].content.parts[0].text.trim();
    console.log(chalk.green(`💡 Generated hint (${hint.length} chars)`));
    
    return hint;
  }

  async sendHint(hint, problemData) {
    if (!this.isConnected) {
      console.log(chalk.yellow('⚠️ Not connected, cannot send hint'));
      return;
    }

    const hintMessage = {
      type: 'instructor_inbox',
      context: 'hint',
      content: {
        hint: hint,
        expert: {
          name: this.config.expert_profile.name,
          user_id: this.config.expert_profile.user_id,
          expertise: this.config.expert_profile.expertise
        },
        problemContext: {
          frustrationLevel: problemData.frustrationLevel,
          timeOnTask: problemData.timeOnTask,
          remainingTime: problemData.remainingTime
        },
        timestamp: new Date().toISOString()
      }
    };

    this.ws.send(JSON.stringify(hintMessage));
    console.log(chalk.green(`✅ Sent hint to instructors (${hint.length} chars)`));
  }

  async sendErrorNotification(problemData, error) {
    if (!this.isConnected) return;

    let errorMessage = "Unknown error occurred";
    
    if (error?.message) {
      if (error.message.includes('401') || error.message.includes('403')) {
        errorMessage = "Invalid or missing Gemini API key";
      } else if (error.message.includes('429')) {
        errorMessage = "Gemini API rate limit exceeded";
      } else if (error.message.includes('500') || error.message.includes('503')) {
        errorMessage = "Gemini API service unavailable";
      } else {
        errorMessage = error.message;
      }
    }
    
    const errorNotification = {
      type: 'instructor_inbox',
      context: 'error',
      content: {
        error: errorMessage,
        expert: {
          name: this.config.expert_profile.name,
          user_id: this.config.expert_profile.user_id
        },
        timestamp: new Date().toISOString()
      }
    };

    this.ws.send(JSON.stringify(errorNotification));
    console.log(chalk.red(`❌ Sent error notification: ${errorMessage}`));
  }

  sendConnectionAnalytics(event) {
    if (!this.isConnected && event !== 'disconnected') return;

    const analyticsMessage = {
      type: 'analytics',
      context: 'connection',
      content: {
        event: event,
        expert: {
          name: this.config.expert_profile.name,
          user_id: this.config.expert_profile.user_id,
          expertise: this.config.expert_profile.expertise
        },
        timestamp: new Date().toISOString(),
        uptime: Date.now() - this.startTime,
        messageCount: this.messageCount
      }
    };

    if (this.ws && this.ws.readyState === 1) {
      this.ws.send(JSON.stringify(analyticsMessage));
    }
  }

  startSessionPolling() {
    // Check for new sessions every 30 seconds
    const pollInterval = setInterval(async () => {
      if (this.isConnected) {
        clearInterval(pollInterval);
        return;
      }
      
      try {
        await this.discoverSessions();
        
        if (this.currentSessions.length > 0) {
          clearInterval(pollInterval);
          await this.connectToSession(this.currentSessions[0].id);
        }
      } catch (error) {
        console.error(chalk.red('❌ Session polling error:'), error.message);
      }
    }, 30000);
  }

  scheduleReconnect() {
    if (this.reconnectTimer || !this.currentSessions.length) return;

    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
    this.reconnectAttempts++;

    console.log(chalk.yellow(`🔄 Reconnecting in ${delay/1000}s (attempt ${this.reconnectAttempts})...`));

    this.reconnectTimer = setTimeout(async () => {
      this.reconnectTimer = null;
      try {
        // First try to rediscover sessions in case the current one ended
        await this.discoverSessions();
        
        if (this.currentSessions.length > 0) {
          await this.connectToSession(this.currentSessions[0].id);
        } else {
          console.log(chalk.yellow('⚠️ No available sessions found. Starting session polling...'));
          this.startSessionPolling();
        }
      } catch (error) {
        console.error(chalk.red('❌ Reconnection failed:'), error.message);
        this.scheduleReconnect();
      }
    }, delay);
  }

  setupShutdown() {
    const handleShutdown = async (signal) => {
      console.log(chalk.yellow(`\n🛑 Received ${signal}, shutting down ${this.config.expert_profile.name}...`));
      await this.stop();
      process.exit(0);
    };

    process.on('SIGINT', handleShutdown);
    process.on('SIGTERM', handleShutdown);
  }

  async stop() {
    console.log(chalk.yellow(`🛑 Stopping ${this.config.expert_profile.name}...`));

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
    }

    if (this.ws && this.isConnected) {
      this.sendConnectionAnalytics('disconnected');
      this.ws.close();
    }

    console.log(chalk.green(`✅ ${this.config.expert_profile.name} stopped`));
  }

  async keepAlive() {
    return new Promise(() => {
      // This promise never resolves, keeping the process alive
    });
  }

  getStatus() {
    return {
      name: this.config.expert_profile.name,
      connected: this.isConnected,
      uptime: Date.now() - this.startTime,
      messageCount: this.messageCount,
      reconnectAttempts: this.reconnectAttempts,
      availableSessions: this.currentSessions.length
    };
  }
}

// CLI Interface
async function main() {
  const argv = yargs(hideBin(process.argv))
    .option('config', {
      alias: 'c',
      description: 'Path to expert configuration file',
      type: 'string',
      demandOption: true
    })
    .help()
    .alias('help', 'h')
    .example('$0 --config experts/technical-expert.json', 'Start technical expert')
    .example('$0 --config experts/emotional-support.json', 'Start emotional support expert')
    .argv;

  // Validate config file exists
  if (!fs.existsSync(argv.config)) {
    console.error(chalk.red(`❌ Config file not found: ${argv.config}`));
    process.exit(1);
  }

  try {
    console.log(chalk.blue('🎓 AI Programming Mentorship - Switchboard Expert Client'));
    console.log(chalk.gray('═'.repeat(60)));
    console.log(chalk.gray(`Config: ${argv.config}`));
    console.log(chalk.gray('═'.repeat(60)));

    const client = new ExpertSwitchboardClient(argv.config);
    await client.start();

  } catch (error) {
    console.error(chalk.red('💥 Fatal error:'), error);
    process.exit(1);
  }
}

// Export for use as module
module.exports = ExpertSwitchboardClient;

// Run CLI if called directly
if (require.main === module) {
  main().catch(error => {
    console.error(chalk.red('💥 Startup error:'), error);
    process.exit(1);
  });
}