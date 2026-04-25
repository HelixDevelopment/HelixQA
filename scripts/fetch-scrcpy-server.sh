#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# fetch-scrcpy-server.sh — operator helper that downloads a pinned
# scrcpy-server.jar release from the upstream Genymobile/scrcpy GitHub
# project + verifies its SHA-256.
#
# HelixQA does NOT redistribute scrcpy-server.jar (licensing / size /
# upgrade hygiene). This script is the canonical way for operators to
# pull the pinned version referenced in
# docs/openclawing/OpenClawing4-Handover.md § "scrcpy-server JAR pin".
#
# Usage:
#   scripts/fetch-scrcpy-server.sh                # writes to ./scrcpy-server.jar
#   scripts/fetch-scrcpy-server.sh /some/path.jar # writes to explicit path
#
# Exit codes: 0 on success, 1 on any failure (no runtime privilege
# escalation — HelixQA never uses "sudo").

set -euo pipefail

# Pin — bump here when upgrading.
SCRCPY_VERSION="3.0.2"
SCRCPY_URL="https://github.com/Genymobile/scrcpy/releases/download/v${SCRCPY_VERSION}/scrcpy-server-v${SCRCPY_VERSION}"
# SHA-256 from the upstream release page.
SCRCPY_SHA256="PLACEHOLDER_UPDATE_ME_WHEN_BUMPING_VERSION"

OUT="${1:-./scrcpy-server.jar}"

if ! command -v curl >/dev/null 2>&1; then
    echo "fetch-scrcpy-server: curl is required (no wget fallback)" >&2
    exit 1
fi
if ! command -v sha256sum >/dev/null 2>&1; then
    echo "fetch-scrcpy-server: sha256sum is required" >&2
    exit 1
fi

TMPDIR=$(mktemp -d -t helixqa-scrcpy-XXXXXX)
trap 'rm -rf "$TMPDIR"' EXIT

TMP_JAR="$TMPDIR/scrcpy-server.jar"

echo "fetch-scrcpy-server: downloading scrcpy-server v${SCRCPY_VERSION}"
curl --fail --silent --show-error --location --output "$TMP_JAR" "$SCRCPY_URL"

# If the SHA is still the placeholder, refuse to trust the download.
if [[ "$SCRCPY_SHA256" == "PLACEHOLDER_UPDATE_ME_WHEN_BUMPING_VERSION" ]]; then
    echo "fetch-scrcpy-server: SHA-256 pin is still the placeholder." >&2
    echo "    Update SCRCPY_SHA256 in scripts/fetch-scrcpy-server.sh to the" >&2
    echo "    value from https://github.com/Genymobile/scrcpy/releases/tag/v${SCRCPY_VERSION}" >&2
    echo "    before running this script in production." >&2
    exit 1
fi

ACTUAL_SHA=$(sha256sum "$TMP_JAR" | awk '{print $1}')
if [[ "$ACTUAL_SHA" != "$SCRCPY_SHA256" ]]; then
    echo "fetch-scrcpy-server: SHA-256 mismatch" >&2
    echo "    expected: $SCRCPY_SHA256" >&2
    echo "    actual:   $ACTUAL_SHA" >&2
    exit 1
fi

# Atomic move into place — caller sees either the old file or the new one,
# never a truncated partial.
mv "$TMP_JAR" "$OUT"
echo "fetch-scrcpy-server: wrote $OUT (v$SCRCPY_VERSION, sha256 ok)"
