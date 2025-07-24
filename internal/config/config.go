package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// ARCHITECTURAL DISCOVERY: Configuration layer serves as system-wide settings coordinator
// Clean separation between configuration management and business logic
type Config struct {
	Database  *DatabaseConfig  `json:"database"`
	HTTP      *HTTPConfig      `json:"http"`
	WebSocket *WebSocketConfig `json:"websocket"`
}

// FUNCTIONAL DISCOVERY: Database configuration supports SQLite optimizations
type DatabaseConfig struct {
	Path    string        `json:"path"`
	Timeout time.Duration `json:"timeout"`
}

// FUNCTIONAL DISCOVERY: HTTP configuration balances performance and reliability
type HTTPConfig struct {
	Port         int           `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	Host         string        `json:"host"`
}

// FUNCTIONAL DISCOVERY: WebSocket configuration optimized for classroom scenarios
type WebSocketConfig struct {
	PingInterval time.Duration `json:"ping_interval"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	BufferSize   int           `json:"buffer_size"`
}

// FUNCTIONAL DISCOVERY: Production-ready defaults based on classroom requirements
// Database on local filesystem, HTTP on standard port, WebSocket with 30s heartbeat
func DefaultConfig() *Config {
	return &Config{
		Database: &DatabaseConfig{
			Path:    "./switchboard.db",
			Timeout: 30 * time.Second,
		},
		HTTP: &HTTPConfig{
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Host:         "0.0.0.0",
		},
		WebSocket: &WebSocketConfig{
			PingInterval: 30 * time.Second,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 10 * time.Second,
			BufferSize:   100,
		},
	}
}

// FUNCTIONAL DISCOVERY: Comprehensive validation prevents invalid system configurations
// Critical for preventing runtime failures in production deployment
func (c *Config) Validate() error {
	if c.Database == nil {
		return fmt.Errorf("database configuration is required")
	}
	
	if c.Database.Path == "" {
		return fmt.Errorf("database path cannot be empty")
	}
	
	if c.Database.Timeout <= 0 {
		return fmt.Errorf("database timeout must be positive")
	}
	
	if c.HTTP == nil {
		return fmt.Errorf("HTTP configuration is required")
	}
	
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("HTTP port must be between 1 and 65535")
	}
	
	if c.HTTP.ReadTimeout <= 0 {
		return fmt.Errorf("HTTP read timeout must be positive")
	}
	
	if c.HTTP.WriteTimeout <= 0 {
		return fmt.Errorf("HTTP write timeout must be positive")
	}
	
	if c.HTTP.Host == "" {
		return fmt.Errorf("HTTP host cannot be empty")
	}
	
	if c.WebSocket == nil {
		return fmt.Errorf("WebSocket configuration is required")
	}
	
	if c.WebSocket.PingInterval <= 0 {
		return fmt.Errorf("WebSocket ping interval must be positive")
	}
	
	if c.WebSocket.ReadTimeout <= 0 {
		return fmt.Errorf("WebSocket read timeout must be positive")
	}
	
	if c.WebSocket.WriteTimeout <= 0 {
		return fmt.Errorf("WebSocket write timeout must be positive")
	}
	
	if c.WebSocket.BufferSize <= 0 {
		return fmt.Errorf("WebSocket buffer size must be positive")
	}
	
	return nil
}

// FUNCTIONAL DISCOVERY: Environment variable configuration enables deployment flexibility
// Supports containerized deployments and configuration management systems
func LoadFromEnv() *Config {
	config := DefaultConfig()
	
	// FUNCTIONAL DISCOVERY: Environment variables override defaults with fallback
	if port := os.Getenv("SWITCHBOARD_HTTP_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.HTTP.Port = p
		}
	}
	
	if host := os.Getenv("SWITCHBOARD_HTTP_HOST"); host != "" {
		config.HTTP.Host = host
	}
	
	if dbPath := os.Getenv("SWITCHBOARD_DATABASE_PATH"); dbPath != "" {
		config.Database.Path = dbPath
	}
	
	if readTimeout := os.Getenv("SWITCHBOARD_HTTP_READ_TIMEOUT"); readTimeout != "" {
		if timeout, err := time.ParseDuration(readTimeout); err == nil {
			config.HTTP.ReadTimeout = timeout
		}
	}
	
	if writeTimeout := os.Getenv("SWITCHBOARD_HTTP_WRITE_TIMEOUT"); writeTimeout != "" {
		if timeout, err := time.ParseDuration(writeTimeout); err == nil {
			config.HTTP.WriteTimeout = timeout
		}
	}
	
	if dbTimeout := os.Getenv("SWITCHBOARD_DATABASE_TIMEOUT"); dbTimeout != "" {
		if timeout, err := time.ParseDuration(dbTimeout); err == nil {
			config.Database.Timeout = timeout
		}
	}
	
	if pingInterval := os.Getenv("SWITCHBOARD_WEBSOCKET_PING_INTERVAL"); pingInterval != "" {
		if interval, err := time.ParseDuration(pingInterval); err == nil {
			config.WebSocket.PingInterval = interval
		}
	}
	
	if wsReadTimeout := os.Getenv("SWITCHBOARD_WEBSOCKET_READ_TIMEOUT"); wsReadTimeout != "" {
		if timeout, err := time.ParseDuration(wsReadTimeout); err == nil {
			config.WebSocket.ReadTimeout = timeout
		}
	}
	
	if wsWriteTimeout := os.Getenv("SWITCHBOARD_WEBSOCKET_WRITE_TIMEOUT"); wsWriteTimeout != "" {
		if timeout, err := time.ParseDuration(wsWriteTimeout); err == nil {
			config.WebSocket.WriteTimeout = timeout
		}
	}
	
	if bufferSize := os.Getenv("SWITCHBOARD_WEBSOCKET_BUFFER_SIZE"); bufferSize != "" {
		if size, err := strconv.Atoi(bufferSize); err == nil {
			config.WebSocket.BufferSize = size
		}
	}
	
	return config
}

// ConfigFile represents the JSON structure for file-based configuration
// FUNCTIONAL DISCOVERY: Separate struct for JSON parsing to handle duration strings
type ConfigFile struct {
	Database  *DatabaseConfigFile  `json:"database"`
	HTTP      *HTTPConfigFile      `json:"http"`
	WebSocket *WebSocketConfigFile `json:"websocket"`
}

type DatabaseConfigFile struct {
	Path    string `json:"path"`
	Timeout string `json:"timeout"`
}

type HTTPConfigFile struct {
	Port         int    `json:"port"`
	ReadTimeout  string `json:"read_timeout"`
	WriteTimeout string `json:"write_timeout"`
	Host         string `json:"host"`
}

type WebSocketConfigFile struct {
	PingInterval string `json:"ping_interval"`
	ReadTimeout  string `json:"read_timeout"`
	WriteTimeout string `json:"write_timeout"`
	BufferSize   int    `json:"buffer_size"`
}

// FUNCTIONAL DISCOVERY: File-based configuration supports complex deployment scenarios
// JSON format chosen for readability and tooling support
func LoadFromFile(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filepath, err)
	}
	
	var configFile ConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", filepath, err)
	}
	
	// Convert to runtime config with duration parsing
	config := DefaultConfig()
	
	if configFile.Database != nil {
		config.Database.Path = configFile.Database.Path
		if configFile.Database.Timeout != "" {
			if timeout, err := time.ParseDuration(configFile.Database.Timeout); err == nil {
				config.Database.Timeout = timeout
			}
		}
	}
	
	if configFile.HTTP != nil {
		if configFile.HTTP.Port > 0 {
			config.HTTP.Port = configFile.HTTP.Port
		}
		if configFile.HTTP.Host != "" {
			config.HTTP.Host = configFile.HTTP.Host
		}
		if configFile.HTTP.ReadTimeout != "" {
			if timeout, err := time.ParseDuration(configFile.HTTP.ReadTimeout); err == nil {
				config.HTTP.ReadTimeout = timeout
			}
		}
		if configFile.HTTP.WriteTimeout != "" {
			if timeout, err := time.ParseDuration(configFile.HTTP.WriteTimeout); err == nil {
				config.HTTP.WriteTimeout = timeout
			}
		}
	}
	
	if configFile.WebSocket != nil {
		if configFile.WebSocket.BufferSize > 0 {
			config.WebSocket.BufferSize = configFile.WebSocket.BufferSize
		}
		if configFile.WebSocket.PingInterval != "" {
			if interval, err := time.ParseDuration(configFile.WebSocket.PingInterval); err == nil {
				config.WebSocket.PingInterval = interval
			}
		}
		if configFile.WebSocket.ReadTimeout != "" {
			if timeout, err := time.ParseDuration(configFile.WebSocket.ReadTimeout); err == nil {
				config.WebSocket.ReadTimeout = timeout
			}
		}
		if configFile.WebSocket.WriteTimeout != "" {
			if timeout, err := time.ParseDuration(configFile.WebSocket.WriteTimeout); err == nil {
				config.WebSocket.WriteTimeout = timeout
			}
		}
	}
	
	// ARCHITECTURAL DISCOVERY: Validate configuration after loading to catch errors early
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", filepath, err)
	}
	
	return config, nil
}

// FUNCTIONAL DISCOVERY: Configuration precedence: file > environment > defaults
// Enables flexible deployment patterns while maintaining sane defaults
func LoadConfigWithPrecedence(filepath string) *Config {
	var config *Config
	
	// Start with defaults
	config = DefaultConfig()
	
	// Override with environment variables
	envConfig := LoadFromEnv()
	if envConfig != nil {
		config = envConfig
	}
	
	// Override with file if provided and exists
	if filepath != "" {
		if fileConfig, err := LoadFromFile(filepath); err == nil {
			config = fileConfig
		}
		// Silently ignore file errors - environment/defaults still work
	}
	
	return config
}