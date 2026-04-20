// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import "digital.vasic.helixqa/pkg/bridge/dbusportal"

// This file re-exports the production D-Bus Caller implementations from
// pkg/bridge/dbusportal. Keeping these aliases means existing code
// (pkg/capture/linux.DBusCaller, NewDBusCaller, DBusCallerFactory, …)
// continues to compile after the dbusportal extraction in OpenClawing4 M17.
//
// New code should import pkg/bridge/dbusportal directly; these aliases exist
// only to avoid churn across downstream consumers.

// DBusCaller re-exports dbusportal.DBusCaller.
type DBusCaller = dbusportal.DBusCaller

// ErrNoSessionBus re-exports dbusportal.ErrNoSessionBus.
var ErrNoSessionBus = dbusportal.ErrNoSessionBus

// NewDBusCaller dials the user-session bus.
var NewDBusCaller = dbusportal.NewDBusCaller

// NewDBusCallerWithConn wraps a pre-existing *dbus.Conn without taking ownership.
var NewDBusCallerWithConn = dbusportal.NewDBusCallerWithConn

// NewDBusCallerOwningConn wraps a connection this DBusCaller will close.
var NewDBusCallerOwningConn = dbusportal.NewDBusCallerOwningConn

// DBusCallerFactory is the production CallerFactory (produces DBusCaller
// instances against the shared session bus).
var DBusCallerFactory = dbusportal.DBusCallerFactory
