// Package transport speaks the SLC CLI over a session channel. The MVP
// transport is Telnet; SSH and serial console implement the same contract.
// See GitHub issue #6 (Transport abstraction contract) for the locked design:
// one deep Run/Read interface, session-per-operation with mandatory logout,
// sessions serialized per credential, pager absorbed, prompt auto-discovered.
package transport

import (
	"context"
	"errors"
	"time"
)

// Config parameterizes a Transport.
type Config struct {
	Host           string
	Port           int           // Telnet port; default 23
	Username       string        // default "sysadmin"
	Password       string        // required
	LoginRetries   int           // extra full-login attempts after the first; default 1
	CommandTimeout time.Duration // per-command deadline; default 60s
}

func (c *Config) withDefaults() Config {
	d := *c
	if d.Port == 0 {
		d.Port = 23
	}
	if d.Username == "" {
		d.Username = "sysadmin"
	}
	if d.LoginRetries == 0 {
		d.LoginRetries = 1
	}
	if d.CommandTimeout == 0 {
		d.CommandTimeout = 60 * time.Second
	}
	return d
}

// Transport executes CLI commands. Run is for writes (never auto-retried);
// Read is for idempotent show-commands (auto-retried once on Timeout or
// Disconnected). Both hide dial/login/prompt/paging mechanics entirely and
// return the command output with echo and prompt stripped.
type Transport interface {
	Run(ctx context.Context, command string) (string, error)
	Read(ctx context.Context, command string) (string, error)
}

// Typed errors per the contract.
type AuthFailedError struct{ Err error }

func (e *AuthFailedError) Error() string { return "authentication failed: " + e.Err.Error() }
func (e *AuthFailedError) Unwrap() error { return e.Err }

type CommandRejectedError struct {
	Command string
	Line    string // verbatim device error line
}

func (e *CommandRejectedError) Error() string {
	return "command rejected: " + e.Line
}

type TimeoutError struct {
	Command string
	After   time.Duration
}

func (e *TimeoutError) Error() string {
	return "timed out after " + e.After.String() + " waiting for prompt"
}

type DisconnectedError struct{ Err error }

func (e *DisconnectedError) Error() string { return "disconnected mid-response: " + e.Err.Error() }
func (e *DisconnectedError) Unwrap() error { return e.Err }

func retryable(err error) bool {
	var te *TimeoutError
	var de *DisconnectedError
	return errors.As(err, &te) || errors.As(err, &de)
}
