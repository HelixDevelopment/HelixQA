// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"os"
	"strconv"
)

func osReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func parseUint(s string) uint64 {
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}
