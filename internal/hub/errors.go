package hub

import "errors"

// Hub-specific error types as defined in Phase 3.2 specifications
var (
	ErrHubAlreadyRunning     = errors.New("hub is already running")
	ErrHubNotRunning         = errors.New("hub is not running")
	ErrSenderNotConnected    = errors.New("sender not connected")
	ErrMessageChannelFull    = errors.New("message channel is full")
	ErrRegisterChannelFull   = errors.New("register channel is full")
	ErrUnregisterChannelFull = errors.New("unregister channel is full")
)