#!/usr/bin/env node

// Simple Static File Server for Switchboard Teacher Client
// No WebSocket server needed - just serves static files

const express = require('express');
const path = require('path');
const chalk = require('chalk');

class StaticServer {
  constructor(port = 3000) {
    this.port = port;
    this.app = express();
    this.setupExpress();
  }

  setupExpress() {
    // Serve static files from current directory
    this.app.use(express.static(__dirname));
    
    // CORS middleware for development
    this.app.use((req, res, next) => {
      res.header('Access-Control-Allow-Origin', '*');
      res.header('Access-Control-Allow-Headers', 'Origin, X-Requested-With, Content-Type, Accept');
      res.header('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
      next();
    });

    // Health check endpoint
    this.app.get('/health', (req, res) => {
      res.json({
        status: 'healthy',
        timestamp: new Date().toISOString(),
        type: 'static_server',
        switchboard_url: 'http://localhost:8080'
      });
    });

    // Serve the main page
    this.app.get('/', (req, res) => {
      res.sendFile(path.join(__dirname, 'index.html'));
    });

    console.log(chalk.gray('✅ Static file server configured'));
  }

  async start() {
    return new Promise((resolve, reject) => {
      this.server = this.app.listen(this.port, (error) => {
        if (error) {
          console.error(chalk.red('❌ Failed to start static server:'), error);
          reject(error);
        } else {
          console.log(chalk.green(`✅ Static server running on http://localhost:${this.port}`));
          console.log(chalk.blue(`🎯 Teacher dashboard ready`));
          console.log(chalk.gray(`📊 Health check: http://localhost:${this.port}/health`));
          console.log(chalk.yellow(`🔗 Make sure Switchboard is running on http://localhost:8080`));
          resolve();
        }
      });
    });
  }

  async stop() {
    if (this.server) {
      return new Promise((resolve) => {
        this.server.close(() => {
          console.log(chalk.green('✅ Static server stopped'));
          resolve();
        });
      });
    }
  }
}

// CLI Interface
async function main() {
  const port = process.argv[2] ? parseInt(process.argv[2]) : 3000;

  try {
    console.log(chalk.blue('🎓 AI Programming Mentorship - Switchboard Teacher Server'));
    console.log(chalk.gray('═'.repeat(60)));
    console.log(chalk.gray(`Port: ${port}`));
    console.log(chalk.gray('Static files only - WebSocket handled by Switchboard'));
    console.log(chalk.gray('═'.repeat(60)));

    const server = new StaticServer(port);
    await server.start();

    console.log(chalk.green('\n🎉 Teacher server ready!'));
    console.log(chalk.blue('💡 Instructions:'));
    console.log(chalk.white('1. Make sure Switchboard server is running on port 8080'));
    console.log(chalk.white('2. Open your browser to http://localhost:3000'));
    console.log(chalk.white('3. Start AI expert clients with updated configs'));
    console.log(chalk.white('4. Create a session and broadcast problems'));
    console.log(chalk.gray('\nPress Ctrl+C to stop the server'));

    // Handle shutdown
    const handleShutdown = async (signal) => {
      console.log(chalk.yellow(`\n🛑 Received ${signal}, shutting down...`));
      await server.stop();
      process.exit(0);
    };

    process.on('SIGINT', handleShutdown);
    process.on('SIGTERM', handleShutdown);

  } catch (error) {
    console.error(chalk.red('💥 Fatal error:'), error);
    process.exit(1);
  }
}

// Export for use as module
module.exports = StaticServer;

// Run CLI if called directly
if (require.main === module) {
  main().catch(error => {
    console.error(chalk.red('💥 Startup error:'), error);
    process.exit(1);
  });
}