#!/usr/bin/env bash
# Go-flavored bluff patterns. Sourced by bluff-scanner.sh.
# Each pattern emits "<relative path>:<line>:BLUFF-G-NNN:<context>"
#
# Skip-marker convention (any of the three forms suppresses a hit when on
# the same line, the immediately preceding line, or anywhere inside the
# function body for body-level patterns like BLUFF-G-003):
#   //  SKIP-OK: #<ticket>           -- repository convention (preferred)
#   //  ANTI-BLUFF-EXEMPT: <ticket>  -- legacy/synonym (forward-compat)
#   //  bluff-scan: no-assert-ok     -- legacy/synonym (forward-compat)

scan_go() {
  local relpath="$1" fpath="$2"

  # BLUFF-G-001: t.Skip() / t.Skipf() without an exempt marker.
  awk -v rel="${relpath}" '
    {
      # Track exempt markers: marker on a line means the next line is exempt
      # (and the current line is exempt if marker is on the same line).
      cur_exempt = (index($0, "SKIP-OK:") > 0 || index($0, "ANTI-BLUFF-EXEMPT:") > 0 || index($0, "bluff-scan:") > 0)
      this_line_exempt = (NR in exempt_next) || cur_exempt
      if (cur_exempt) exempt_next[NR+1] = 1
      if ($0 ~ /[[:space:]]t\.Skip[f]?\(/ || $0 ~ /^t\.Skip[f]?\(/) {
        if (!this_line_exempt) {
          print rel ":" NR ":BLUFF-G-001:t.Skip without exempt comment"
        }
      }
    }
  ' "$fpath"

  # BLUFF-G-005: t.Run("", func(t *testing.T) { }) — empty named subtest.
  awk -v rel="${relpath}" '
    /t\.Run\("",[[:space:]]*func\(t \*testing\.T\)[[:space:]]*\{[[:space:]]*\}\)/ {
      print rel ":" NR ":BLUFF-G-005:empty t.Run subtest"
    }
  ' "$fpath"

  # BLUFF-G-006: empty TestXxx body on a single line.
  awk -v rel="${relpath}" '
    /^func Test[A-Z][A-Za-z0-9_]*\(t \*testing\.T\)[[:space:]]*\{[[:space:]]*\}[[:space:]]*$/ {
      print rel ":" NR ":BLUFF-G-006:empty test body"
    }
  ' "$fpath"

  # BLUFF-G-007: assert.True(t, true) / assert.NotNil(t, x) as a literal line.
  awk -v rel="${relpath}" '
    /^[[:space:]]*assert\.True\(t,[[:space:]]*true\)/ {
      # skip if exempt marker is on this line or the previous line
      if (index($0, "SKIP-OK:") == 0 && index($0, "ANTI-BLUFF-EXEMPT:") == 0 &&
          index($0, "bluff-scan:") == 0 &&
          (prev !~ /SKIP-OK:|ANTI-BLUFF-EXEMPT:|bluff-scan:/)) {
        print rel ":" NR ":BLUFF-G-007:assert.True(t, true) trivial"
      }
    }
    { prev = $0 }
  ' "$fpath"

  # BLUFF-G-003: test function whose body has only t.Log calls (no t.Fatal/Error/Errorf,
  # no assert., no require., no testing.B/F-style assertions).
  awk -v rel="${relpath}" '
    function flush(start_line) {
      if (start_line > 0 && asserts == 0 && logs > 0 && body_exempt == 0) {
        print rel ":" start_line ":BLUFF-G-003:no-assert test (only t.Log)"
      }
    }
    /^func Test[A-Z][A-Za-z0-9_]*\(t \*testing\.T\)[[:space:]]*\{/ {
      flush(start_line)
      start_line = NR; brace = 1; asserts = 0; logs = 0; body_exempt = 0
      next
    }
    start_line > 0 {
      n = gsub(/\{/, "&"); brace += n
      n = gsub(/\}/, "&"); brace -= n
      if ($0 ~ /t\.(Fatal|Fatalf|Error|Errorf|Skip|Skipf|FailNow|Fail|Cleanup)\(|assert\.|require\./)  asserts++
      if ($0 ~ /t\.Log[f]?\(/) logs++
      if ($0 ~ /SKIP-OK:|ANTI-BLUFF-EXEMPT:|bluff-scan:/) body_exempt = 1
      if (brace == 0) { flush(start_line); start_line = 0 }
    }
    END { flush(start_line) }
  ' "$fpath"
}
