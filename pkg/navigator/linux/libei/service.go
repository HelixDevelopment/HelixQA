// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package libei

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"digital.vasic.helixqa/pkg/bridge/dbusportal"
)

// ServiceConfig drives NewDefaultService — the one-liner operator-facing
// entry point that runs the full RemoteDesktop portal handshake and returns
// a Service holding the resulting EIS file descriptor.
type ServiceConfig struct {
	// Types selects which device kinds to request. Zero -> Keyboard | Pointer.
	Types DeviceType

	// ParentWindow is passed to portal Start. Empty is fine for headless QA.
	ParentWindow string

	// Persist asks the portal to remember this selection.
	Persist PersistMode

	// RestoreToken, if set, tries to restore a prior selection without
	// re-prompting. Requires Persist > 0 on the session that yielded the token.
	RestoreToken string
}

// Service bundles a completed RemoteDesktop session: portal handshake done,
// ConnectToEIS called, the Unix-socket FD held as an *os.File. An EI
// wire-protocol client (lands in a future commit as ei_client.go) consumes
// EISFile() to actually emit input events.
type Service struct {
	portal  *Portal
	eisFile *os.File
	granted DeviceType
	restore string

	closeOnce sync.Once
	closeErr  error
}

// NewDefaultService is the production entry point — dials the user-session
// D-Bus via dbusportal.DBusCallerFactory, then runs:
//
//	CreateSession → SelectDevices → Start → ConnectToEIS
//
// Returns a Service ready for EISFile() access. Callers Close the service
// when done; that closes the FD and the Caller.
func NewDefaultService(ctx context.Context, cfg ServiceConfig) (*Service, error) {
	return NewServiceWithFactory(ctx, dbusportal.DBusCallerFactory, cfg)
}

// NewServiceWithFactory lets callers inject their own CallerFactory —
// typical uses: tests injecting a fake; integration suites driving a
// dbus-launch'd private bus.
func NewServiceWithFactory(ctx context.Context, factory dbusportal.CallerFactory, cfg ServiceConfig) (*Service, error) {
	if factory == nil {
		return nil, errors.New("libei: NewServiceWithFactory: nil factory")
	}
	caller, err := factory()
	if err != nil {
		return nil, fmt.Errorf("libei: CallerFactory: %w", err)
	}
	portal := NewPortal(caller)
	return runHandshake(ctx, portal, cfg)
}

// NewServiceWithPortal runs the handshake using a pre-built Portal. Useful
// when the caller wants to share a Caller across multiple sessions or drive
// CreateSession/SelectDevices/Start manually for special cases.
func NewServiceWithPortal(ctx context.Context, portal *Portal, cfg ServiceConfig) (*Service, error) {
	if portal == nil {
		return nil, errors.New("libei: NewServiceWithPortal: nil portal")
	}
	return runHandshake(ctx, portal, cfg)
}

func runHandshake(ctx context.Context, portal *Portal, cfg ServiceConfig) (*Service, error) {
	sessPath, err := portal.CreateSession(ctx)
	if err != nil {
		_ = portal.Close()
		return nil, err
	}
	if err := portal.SelectDevices(ctx, sessPath, SelectDevicesOptions{
		Types:        cfg.Types,
		Persist:      cfg.Persist,
		RestoreToken: cfg.RestoreToken,
	}); err != nil {
		_ = portal.Close()
		return nil, err
	}
	startRes, err := portal.Start(ctx, sessPath, cfg.ParentWindow)
	if err != nil {
		_ = portal.Close()
		return nil, err
	}
	fd, err := portal.ConnectToEIS(ctx, sessPath)
	if err != nil {
		_ = portal.Close()
		return nil, err
	}
	return &Service{
		portal:  portal,
		eisFile: fd,
		granted: startRes.ChosenDevices,
		restore: startRes.RestoreToken,
	}, nil
}

// EISFile returns the *os.File wrapping the Unix-socket FD ConnectToEIS
// handed back. The EI wire-protocol client reads / writes this FD to emit
// input events. Service retains ownership — consumers MUST NOT close the
// file themselves; call Service.Close() instead.
func (s *Service) EISFile() *os.File {
	if s == nil {
		return nil
	}
	return s.eisFile
}

// GrantedDevices returns the bitmask the portal actually granted. Callers
// MUST NOT write events for devices the portal did not grant — that is a
// protocol violation that libei will reject.
func (s *Service) GrantedDevices() DeviceType {
	if s == nil {
		return 0
	}
	return s.granted
}

// RestoreToken returns the non-empty token the portal issued when Persist
// was set; empty otherwise. Save this + pass it as cfg.RestoreToken on the
// next session to skip the consent prompt.
func (s *Service) RestoreToken() string {
	if s == nil {
		return ""
	}
	return s.restore
}

// Close closes the EIS file and the underlying Caller. Idempotent.
func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		if s.eisFile != nil {
			if err := s.eisFile.Close(); err != nil {
				s.closeErr = fmt.Errorf("libei: close EIS file: %w", err)
			}
			s.eisFile = nil
		}
		if s.portal != nil {
			if err := s.portal.Close(); err != nil && s.closeErr == nil {
				s.closeErr = fmt.Errorf("libei: close portal: %w", err)
			}
		}
	})
	return s.closeErr
}
