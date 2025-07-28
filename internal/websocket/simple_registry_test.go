package websocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewRegistry tests registry creation
func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.globalConnections)
	assert.NotNil(t, registry.sessionInstructors)
	assert.NotNil(t, registry.sessionStudents)
}

// TestGetStats tests statistics gathering
func TestGetStats(t *testing.T) {
	registry := NewRegistry()
	stats := registry.GetStats()
	
	assert.Contains(t, stats, "total_connections")
	assert.Contains(t, stats, "active_sessions")
	assert.Equal(t, 0, stats["total_connections"])
	assert.Equal(t, 0, stats["active_sessions"])
}

// TestGetLobbyConnections tests lobby connections retrieval
func TestGetLobbyConnections(t *testing.T) {
	registry := NewRegistry()
	connections := registry.GetLobbyConnections()
	assert.Empty(t, connections)
}

// TestGetAllConnections tests all connections retrieval  
func TestGetAllConnections(t *testing.T) {
	registry := NewRegistry()
	connections := registry.GetAllConnections()
	assert.Empty(t, connections)
}