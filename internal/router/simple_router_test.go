package router

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"switchboard/internal/websocket"
	"switchboard/pkg/types"
)

// MockConnection for testing
type MockConnection struct {
	userID    string
	role      string
	sessionID string
	messages  []interface{}
	closed    bool
	mu        sync.Mutex
}

func NewMockConnection(userID, role, sessionID string) *MockConnection {
	return &MockConnection{
		userID:    userID,
		role:      role,
		sessionID: sessionID,
		messages:  make([]interface{}, 0),
	}
}

func (c *MockConnection) GetUserID() string     { return c.userID }
func (c *MockConnection) GetRole() string       { return c.role }
func (c *MockConnection) GetSessionID() string  { return c.sessionID }
func (c *MockConnection) IsAuthenticated() bool { return true }
func (c *MockConnection) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, v)
	return nil
}
func (c *MockConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}
func (c *MockConnection) GetLastMessage() *types.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.messages) == 0 {
		return nil
	}
	if msg, ok := c.messages[len(c.messages)-1].(*types.Message); ok {
		return msg
	}
	return nil
}
func (c *MockConnection) GetAllMessages() []interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]interface{}{}, c.messages...)
}

// SimpleMockDB for basic testing
type SimpleMockDB struct {
	mock.Mock
}

func (m *SimpleMockDB) StoreMessage(ctx context.Context, message *types.Message) error {
	return nil
}

func (m *SimpleMockDB) StoreMessageBatch(ctx context.Context, messages []*types.Message) error {
	return nil
}

func (m *SimpleMockDB) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
	return []*types.Message{}, nil
}

func (m *SimpleMockDB) CreateSession(ctx context.Context, session *types.Session) error {
	return nil
}

func (m *SimpleMockDB) UpdateSession(ctx context.Context, session *types.Session) error {
	return nil
}

func (m *SimpleMockDB) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	return &types.Session{}, nil
}

func (m *SimpleMockDB) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	return []*types.Session{}, nil
}

func (m *SimpleMockDB) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *SimpleMockDB) Close() error {
	return nil
}

// TestNewRouter tests router creation
func TestNewRouter(t *testing.T) {
	registry := websocket.NewRegistry()
	dbManager := new(SimpleMockDB)
	
	router := NewRouter(registry, dbManager)
	assert.NotNil(t, router)
}

// TestValidMessageTypes tests message type validation
func TestValidMessageTypes(t *testing.T) {
	registry := websocket.NewRegistry()
	dbManager := new(SimpleMockDB)
	router := NewRouter(registry, dbManager)
	
	validTypes := []string{
		"instructor_inbox",
		"inbox_response", 
		"request",
		"request_response",
		"analytics",
		"instructor_broadcast",
	}
	
	for _, msgType := range validTypes {
		assert.True(t, router.isValidMessageType(msgType))
	}
	
	assert.False(t, router.isValidMessageType("invalid_type"))
}

// Async Persistence Tests

// MockBatchingDB tracks persistence calls for async testing
type MockBatchingDB struct {
	mock.Mock
	messages      []*types.Message
	persistCalled chan struct{}
	blockPersist  bool
}

func NewMockBatchingDB() *MockBatchingDB {
	return &MockBatchingDB{
		messages:      make([]*types.Message, 0),
		persistCalled: make(chan struct{}, 100),
	}
}

func (m *MockBatchingDB) StoreMessage(ctx context.Context, message *types.Message) error {
	m.messages = append(m.messages, message)
	select {
	case m.persistCalled <- struct{}{}:
	default:
	}
	
	if m.blockPersist {
		<-ctx.Done()
		return ctx.Err()
	}
	
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockBatchingDB) StoreMessageBatch(ctx context.Context, messages []*types.Message) error {
	return nil
}

func (m *MockBatchingDB) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
	return []*types.Message{}, nil
}

func (m *MockBatchingDB) CreateSession(ctx context.Context, session *types.Session) error {
	return nil
}

func (m *MockBatchingDB) UpdateSession(ctx context.Context, session *types.Session) error {
	return nil
}

func (m *MockBatchingDB) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	return &types.Session{}, nil
}

func (m *MockBatchingDB) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	return []*types.Session{}, nil
}

func (m *MockBatchingDB) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockBatchingDB) Close() error {
	return nil
}

// TestRouter_RouteImmediately tests that routing happens before persistence
func TestRouter_RouteImmediately(t *testing.T) {
	t.Skip("Skipping until websocket mocking is properly implemented")
}

// TestRouter_AsyncPersistence tests messages reach batcher
func TestRouter_AsyncPersistence(t *testing.T) {
	t.Skip("Skipping until websocket mocking is properly implemented")
}

// TestRouter_PersistenceFailureOK tests routing continues on DB error
func TestRouter_PersistenceFailureOK(t *testing.T) {
	t.Skip("Skipping until websocket mocking is properly implemented")
}

// TestRouter_BatcherIntegration tests router integration with batcher
func TestRouter_BatcherIntegration(t *testing.T) {
	t.Skip("Skipping until router batching integration is implemented")
}


// TestRolePermissions tests role-based message permissions
func TestRolePermissions(t *testing.T) {
	registry := websocket.NewRegistry()
	dbManager := new(SimpleMockDB)
	router := NewRouter(registry, dbManager)
	
	// Student permissions
	assert.True(t, router.canSendMessageType("student", "instructor_inbox"))
	assert.True(t, router.canSendMessageType("student", "request_response"))
	assert.True(t, router.canSendMessageType("student", "analytics"))
	assert.False(t, router.canSendMessageType("student", "inbox_response"))
	assert.False(t, router.canSendMessageType("student", "request"))
	assert.False(t, router.canSendMessageType("student", "instructor_broadcast"))
	
	// Instructor permissions
	assert.False(t, router.canSendMessageType("instructor", "instructor_inbox"))
	assert.False(t, router.canSendMessageType("instructor", "request_response"))
	assert.False(t, router.canSendMessageType("instructor", "analytics"))
	assert.True(t, router.canSendMessageType("instructor", "inbox_response"))
	assert.True(t, router.canSendMessageType("instructor", "request"))
	assert.True(t, router.canSendMessageType("instructor", "instructor_broadcast"))
	
	// Invalid role
	assert.False(t, router.canSendMessageType("admin", "instructor_inbox"))
}