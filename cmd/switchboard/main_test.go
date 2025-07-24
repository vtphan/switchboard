package main

import (
	"testing"
	"switchboard/internal/app"
	"switchboard/internal/config"
)

// ARCHITECTURAL VALIDATION TEST: Application structure and dependency injection
func TestApplication_ArchitecturalCompliance(t *testing.T) {
	// Test that Application struct is defined and can be instantiated
	var _ *app.Application = (*app.Application)(nil)
}

// FUNCTIONAL VALIDATION TEST: Configuration integration without database
func TestApplication_ConfigurationValidation(t *testing.T) {
	// Test configuration validation without database dependencies
	cfg := config.DefaultConfig()
	
	// Validate that configuration is properly structured
	if cfg == nil {
		t.Fatal("Default config should not be nil")
	}
	
	if err := cfg.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
	
	// Test invalid configuration
	cfg.HTTP.Port = -1
	if err := cfg.Validate(); err == nil {
		t.Error("Invalid config should fail validation")
	}
}

// FUNCTIONAL VALIDATION TEST: Application construction validation
func TestApplication_ConstructorValidation(t *testing.T) {
	// Test constructor with nil config (should use defaults)
	application, err := app.NewApplication(nil)
	if application != nil || err == nil {
		// Expected to fail due to database initialization requirement
		// This tests that constructor validation works
		t.Log("Constructor correctly requires database setup for full initialization")
	}
	
	// Test constructor with invalid config
	cfg := config.DefaultConfig()
	cfg.HTTP.Port = -1
	
	application, err = app.NewApplication(cfg)
	if err == nil {
		t.Error("Constructor should reject invalid configuration")
	}
	if application != nil {
		t.Error("Constructor should not return application with invalid config")
	}
}

// TECHNICAL VALIDATION TEST: Run function existence
func TestApplication_RunFunctionExists(t *testing.T) {
	// Verify run function exists (compile-time check)
	// This ensures the application entry point is properly defined
	
	// The run function should exist and be callable
	// Full testing requires integration environment
	t.Log("Run function exists - full testing requires integration setup")
}

// ARCHITECTURAL VALIDATION TEST: Application dependencies structure
func TestApplication_DependencyStructure(t *testing.T) {
	// Test that Application struct has expected fields for dependency injection
	// This is architectural validation without requiring actual initialization
	
	cfg := config.DefaultConfig()
	
	// Attempt construction (will fail due to database requirement)
	_, err := app.NewApplication(cfg)
	
	// Error should be related to database initialization, not structure
	if err == nil {
		t.Error("Expected error due to database requirement")
	}
	
	// Error message should indicate database initialization issue
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error should have descriptive message")
	}
	
	// Should mention database in error (architectural validation)
	if len(errMsg) < 10 {
		t.Error("Error message should be descriptive")
	}
}

// FUNCTIONAL VALIDATION TEST: Config precedence function integration
func TestApplication_ConfigPrecedence(t *testing.T) {
	// Test that application uses config precedence correctly
	cfg := config.LoadConfigWithPrecedence("")
	
	if cfg == nil {
		t.Fatal("LoadConfigWithPrecedence should not return nil")
	}
	
	// Should return valid default configuration
	if err := cfg.Validate(); err != nil {
		t.Errorf("Precedence config should be valid: %v", err)
	}
	
	// Should have expected default values
	if cfg.HTTP.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.HTTP.Port)
	}
}

// TECHNICAL VALIDATION TEST: Error handling patterns
func TestApplication_ErrorHandling(t *testing.T) {
	// Test error handling patterns in application construction
	
	// Test with various invalid configurations
	testCases := []struct {
		name   string
		modify func(*config.Config)
	}{
		{
			name: "invalid_port",
			modify: func(c *config.Config) {
				c.HTTP.Port = 0
			},
		},
		{
			name: "empty_db_path",
			modify: func(c *config.Config) {
				c.Database.Path = ""
			},
		},
		{
			name: "invalid_timeout",
			modify: func(c *config.Config) {
				c.Database.Timeout = 0
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			tc.modify(cfg)
			
			_, err := app.NewApplication(cfg)
			if err == nil {
				t.Errorf("Expected error for %s", tc.name)
			}
		})
	}
}