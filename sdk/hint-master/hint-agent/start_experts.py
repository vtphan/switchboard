#!/usr/bin/env python3
"""
Multi-Expert Startup Script

This script starts multiple hint agents simultaneously, each with their own configuration.
Useful for running a team of AI experts in parallel.
"""

import asyncio
import sys
import signal
from pathlib import Path
from typing import List, Dict, Any
import json
import subprocess
import time


class ExpertManager:
    """Manages multiple expert agent processes"""
    
    def __init__(self):
        self.processes: List[subprocess.Popen] = []
        self.shutdown_requested = False
        self.setup_shutdown_handlers()
    
    def setup_shutdown_handlers(self):
        """Setup graceful shutdown handlers"""
        def handle_shutdown(signum, frame):
            print(f"\n🛑 Received signal {signum}, shutting down all experts...")
            self.shutdown_requested = True
        
        signal.signal(signal.SIGINT, handle_shutdown)
        signal.signal(signal.SIGTERM, handle_shutdown)
    
    def find_config_files(self, config_dir: Path) -> List[Path]:
        """Find all JSON config files in the configs directory"""
        if not config_dir.exists():
            print(f"❌ Config directory not found: {config_dir}")
            return []
        
        configs = list(config_dir.glob("*.json"))
        print(f"🔍 Found {len(configs)} expert configurations:")
        for config in configs:
            print(f"   - {config.stem}")
        
        return configs
    
    def validate_config(self, config_path: Path) -> bool:
        """Validate a config file has required fields"""
        try:
            with open(config_path) as f:
                data = json.load(f)
            
            required_fields = ['user_id', 'name', 'expertise', 'gemini_api_key']
            for field in required_fields:
                if field not in data:
                    print(f"❌ {config_path.name}: Missing required field '{field}'")
                    return False
            
            if data['gemini_api_key'] == 'your_gemini_api_key_here':
                print(f"⚠️  {config_path.name}: Placeholder API key detected - please update with real key")
                return False
            
            return True
            
        except Exception as e:
            print(f"❌ {config_path.name}: Invalid JSON - {e}")
            return False
    
    def start_expert(self, config_path: Path) -> subprocess.Popen:
        """Start a single expert agent process"""
        agent_script = Path(__file__).parent / "hint_agent.py"
        
        print(f"🚀 Starting expert: {config_path.stem}")
        
        process = subprocess.Popen([
            sys.executable, str(agent_script), str(config_path)
        ], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        
        return process
    
    def start_all_experts(self, config_paths: List[Path]) -> None:
        """Start all expert agents"""
        print(f"🎓 Starting {len(config_paths)} AI experts...")
        print("=" * 60)
        
        for config_path in config_paths:
            if not self.validate_config(config_path):
                print(f"⏭️  Skipping {config_path.name} due to validation errors")
                continue
            
            try:
                process = self.start_expert(config_path)
                self.processes.append(process)
                time.sleep(1)  # Stagger startup to avoid connection conflicts
                
            except Exception as e:
                print(f"❌ Failed to start {config_path.name}: {e}")
        
        print("=" * 60)
        print(f"✅ Started {len(self.processes)} expert agents")
        
        if len(self.processes) == 0:
            print("❌ No experts started successfully")
            return
        
        print("\nRunning experts:")
        for i, process in enumerate(self.processes):
            config_name = config_paths[i].stem if i < len(config_paths) else f"Expert {i+1}"
            print(f"   🤖 {config_name} (PID: {process.pid})")
    
    def monitor_experts(self) -> None:
        """Monitor expert processes and handle failures"""
        print("\n📊 Monitoring experts (Ctrl+C to stop all)...")
        print("=" * 60)
        
        failed_experts = []
        
        while not self.shutdown_requested and len(self.processes) > 0:
            time.sleep(5)  # Check every 5 seconds
            
            # Check for failed processes
            for i, process in enumerate(self.processes):
                if process.poll() is not None:  # Process has terminated
                    return_code = process.returncode
                    expert_name = f"Expert {i+1}"
                    
                    if return_code != 0:
                        print(f"❌ {expert_name} failed with exit code {return_code}")
                        
                        # Capture output for debugging
                        try:
                            output, _ = process.communicate(timeout=1)
                            if output:
                                print(f"   Last output: {output.strip()[-200:]}")  # Last 200 chars
                        except:
                            pass
                    
                    failed_experts.append(i)
            
            # Remove failed processes
            for i in reversed(failed_experts):
                if i < len(self.processes):
                    self.processes.pop(i)
            failed_experts.clear()
            
            if len(self.processes) == 0:
                print("⚠️ All experts have stopped")
                break
    
    def stop_all_experts(self) -> None:
        """Stop all expert processes gracefully"""
        if not self.processes:
            return
        
        print(f"🛑 Stopping {len(self.processes)} expert agents...")
        
        # Send SIGINT to all processes
        for process in self.processes:
            if process.poll() is None:  # Still running
                try:
                    process.send_signal(signal.SIGINT)
                except:
                    pass
        
        # Wait for graceful shutdown
        print("⏳ Waiting for graceful shutdown...")
        time.sleep(3)
        
        # Force terminate any remaining processes
        for process in self.processes:
            if process.poll() is None:
                print(f"🔄 Force terminating PID {process.pid}")
                process.terminate()
        
        # Final cleanup
        time.sleep(1)
        for process in self.processes:
            if process.poll() is None:
                process.kill()
        
        print("✅ All experts stopped")
    
    def run(self, config_dir: str = "configs", specific_configs: List[str] = None) -> None:
        """Main run method"""
        config_dir_path = Path(__file__).parent / config_dir
        
        if specific_configs:
            # Run specific configurations
            config_paths = []
            for config_name in specific_configs:
                config_path = config_dir_path / f"{config_name}.json"
                if config_path.exists():
                    config_paths.append(config_path)
                else:
                    print(f"❌ Config not found: {config_name}.json")
            
        else:
            # Run all configurations
            config_paths = self.find_config_files(config_dir_path)
        
        if not config_paths:
            print("❌ No valid configurations found")
            sys.exit(1)
        
        try:
            self.start_all_experts(config_paths)
            self.monitor_experts()
            
        except KeyboardInterrupt:
            pass
        finally:
            self.stop_all_experts()


def main():
    """Main entry point with CLI argument parsing"""
    import argparse
    
    parser = argparse.ArgumentParser(
        description="Start multiple AI expert hint agents",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python start_experts.py                                      # Start all experts
  python start_experts.py --experts technical-expert caring-instructor  # Start specific experts
  python start_experts.py --experts peer-student emotional-support-coach  # Start peer support experts
  python start_experts.py --config-dir /path/to/configs       # Use custom config directory
        """
    )
    
    parser.add_argument(
        '--experts', 
        nargs='+',
        help='Specific expert configurations to start (without .json extension)'
    )
    
    parser.add_argument(
        '--config-dir',
        default='configs',
        help='Directory containing expert configuration files (default: configs)'
    )
    
    args = parser.parse_args()
    
    print("🎓 AI Programming Mentorship - Expert Manager")
    print("=" * 60)
    
    if args.experts:
        print(f"Starting specific experts: {', '.join(args.experts)}")
    else:
        print("Starting all available experts")
    
    print(f"Config directory: {args.config_dir}")
    print()
    
    manager = ExpertManager()
    manager.run(args.config_dir, args.experts)


if __name__ == "__main__":
    main()