// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// RemoteWorkerInfo exposes inspection fields of a Worker resolved
// to a remote host. Returns ok=false if the Worker is local.
type RemoteWorkerInfo interface {
	Host() string
}

// Unwrap returns a RemoteWorkerInfo if w is a remote worker.
func Unwrap(w contracts.Worker) (RemoteWorkerInfo, bool) {
	if rw, ok := w.(*remoteWorker); ok {
		return rw, true
	}
	return nil, false
}

// Host implements RemoteWorkerInfo.
func (r *remoteWorker) Host() string { return r.host }
