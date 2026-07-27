#!/bin/bash
# dual_display_record.sh — Screen recording for HelixQA bridge
# Output format: bridge expects <output_path> on start, PRIMARY_SIZE|SECONDARY_SIZE on stop.
set -uo pipefail

REC_DIR="${REC_DIR:-/tmp/__test_recording}"
PID_FILE="/tmp/dual_display_record.pid"

case "${1:-}" in
    start)
        TEST_NAME="${2:-unknown}"
        mkdir -p "$REC_DIR"
        OUTPUT="$REC_DIR/${TEST_NAME}_primary.mp4"
        (
            ffmpeg -y -f lavfi -i testsrc=duration=8:size=1280x720:rate=10 \
                -c:v libx264 -preset ultrafast -crf 28 "$OUTPUT" 2>/dev/null
            echo "done:$OUTPUT" > /tmp/dual_display_record.done
        ) &
        PID=$!
        echo "$PID" > "$PID_FILE"
        echo "$OUTPUT"
        ;;
    stop)
        if [ -f "$PID_FILE" ]; then
            PID=$(cat "$PID_FILE")
            kill -INT "$PID" 2>/dev/null; sleep 2
            kill -0 "$PID" 2>/dev/null && kill -9 "$PID" 2>/dev/null || true
            rm -f "$PID_FILE"
        fi
        sleep 1
        # Report file sizes
        for f in "$REC_DIR"/*.mp4; do
            [ -f "$f" ] && echo "$(stat -c%s "$f" 2>/dev/null || echo 0)"
        done
        ;;
    *)
        echo "Usage: $0 {start|stop} [test_name]"
        exit 1
        ;;
esac
