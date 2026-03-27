# Evidence Collection and Reporting

Every finding in HelixQA is backed by evidence. Screenshots, video recordings, log extracts, stack traces, and performance metrics are captured at each test step and attached to issue tickets and session reports. This guide covers how evidence collection works, how to configure it per platform, and how to interpret the generated reports.

## Evidence Types

HelixQA collects six categories of evidence during test execution:

| Type | Format | Captured When |
|------|--------|--------------|
| Screenshots | PNG (default) or JPG | After every test step |
| Video | MP4 | Full test case execution (start to finish) |
| Audio | WAV or FLAC | When `recording_audio: true` (media playback testing) |
| Logs | Text | Crash/ANR detection, console output |
| Stack traces | Text | On crash or unhandled exception |
| Performance metrics | JSON | At configured intervals during execution |

All evidence is stored in the session output directory under `qa-results/session-<timestamp>/`.

---

## Configuring Evidence Collection

### Via CLI Flags

```bash
# Enable everything (default)
helixqa run --banks banks/ --record=true --validate=true

# Disable video recording (faster, less disk)
helixqa run --banks banks/ --record=false

# Control output location
helixqa run --banks banks/ --output /tmp/qa-evidence
```

### Via YAML Configuration

For fine-grained control, use the autonomous config section:

```yaml
autonomous:
  recording_video: true
  recording_screenshots: true
  recording_video_quality: "medium"    # low, medium, high
  recording_screenshot_format: "png"   # png, jpg
  recording_audio: false
  recording_audio_quality: "high"      # standard, high, ultra
  recording_audio_format: "wav"        # wav, flac
  recording_ffmpeg_path: "/usr/bin/ffmpeg"
```

### Via Environment Variables

```env
HELIX_FFMPEG_PATH=/usr/bin/ffmpeg
HELIX_OUTPUT_DIR=qa-results
```

---

## Screenshot Collection

Screenshots are the primary evidence type. The LLM vision model analyses every screenshot for visual defects, UX issues, accessibility problems, and brand compliance.

### Capture Timing

A screenshot is taken:

1. **Before each test step** (pre-screenshot for comparison)
2. **After each test step** (post-screenshot for validation)
3. **On crash or ANR detection** (diagnostic screenshot)
4. **During curiosity exploration** (every new screen reached)

### Naming Convention

```
test-<NNN>-<slug>-step-<N>-<pre|post>.png
```

Example:

```
test-001-login-flow-step-1-pre.png
test-001-login-flow-step-1-post.png
test-001-login-flow-step-2-pre.png
test-001-login-flow-step-2-post.png
```

### Duplicate Detection

HelixQA uses SSIM (Structural Similarity Index) to detect duplicate screenshots. When consecutive screenshots have SSIM above the configured threshold (default 0.95), only one copy is stored. This reduces disk usage and avoids redundant LLM vision analysis.

```yaml
autonomous:
  vision_ssim_threshold: 0.95
```

### Screenshot Analysis

Each screenshot is sent to the LLM vision model with a structured prompt requesting analysis across six categories:

| Category | What the Model Checks |
|----------|----------------------|
| Visual | Layout alignment, element rendering, clipped text, wrong colours |
| UX | Unresponsive buttons, confusing flows, missing feedback |
| Accessibility | Colour contrast (WCAG), touch target sizes, screen reader labels |
| Brand | Logo presence, correct branding, style consistency |
| Content | Empty screens, placeholder text, missing data |
| Performance | Visible jank, loading spinners, slow transitions |

---

## Video Recording

Video captures the full execution of each test case, providing context that individual screenshots cannot.

### Platform-Specific Recording Methods

#### Android 9 and Below (Mi Box, Older Emulators)

Uses the native `screenrecord` command via ADB:

```bash
adb shell screenrecord --bit-rate 4000000 --time-limit 120 \
    /sdcard/qa_session.mp4
```

After recording, the file is pulled from the device:

```bash
adb pull /sdcard/qa_session.mp4 qa-results/videos/
```

**Limitation**: `screenrecord` has a maximum duration of 180 seconds per recording. For longer tests, HelixQA chains multiple recordings.

#### Android 10+ (Modern Phones)

On Android 10 and later, `screenrecord` from ADB frequently fails with `Encoder failed (err=-38)`. HelixQA uses a rapid screenshot capture approach instead:

1. Captures screenshots at 2-5 FPS using `adb shell screencap`
2. Stores frames as numbered PNGs in a temporary directory
3. Assembles frames into MP4 using ffmpeg:

```bash
ffmpeg -framerate 5 -i frame-%04d.png -c:v libx264 \
    -pix_fmt yuv420p output.mp4
```

Frame sequences are stored in `qa-results/video-sessions-<timestamp>/<device>-frames/` for post-session inspection.

#### Web (Playwright)

Playwright has built-in video recording. HelixQA enables it when `recording_video: true`:

```typescript
const context = await browser.newContext({
    recordVideo: { dir: 'qa-results/videos/' }
});
```

Videos are automatically saved when the browser context closes. No ffmpeg dependency is needed for web recording.

#### Desktop (Tauri, X11)

Desktop recording uses ffmpeg to capture the X11 display:

```bash
ffmpeg -f x11grab -framerate 15 -video_size 1920x1080 \
    -i :0 -c:v libx264 -preset ultrafast \
    -pix_fmt yuv420p output.mp4
```

When running in a headless environment, use Xvfb (X virtual framebuffer) to provide a virtual display:

```bash
Xvfb :99 -screen 0 1920x1080x24 &
export DISPLAY=:99
```

### Video Quality Settings

| Quality | Resolution | Bitrate | FPS | Typical Size |
|---------|-----------|---------|-----|-------------|
| `low` | 720p | 1 Mbps | 10 | ~7 MB/min |
| `medium` | 1080p | 4 Mbps | 15 | ~30 MB/min |
| `high` | 1080p | 8 Mbps | 30 | ~60 MB/min |

---

## Audio Recording

Audio recording is disabled by default. Enable it for testing media playback quality (music players, video players, notification sounds):

```yaml
autonomous:
  recording_audio: true
  recording_audio_quality: "high"
  recording_audio_format: "wav"
  recording_audio_device: "default"
```

### Quality Levels

| Quality | Sample Rate | Bit Depth | Use Case |
|---------|------------|-----------|----------|
| `standard` | 44.1 kHz | 16-bit | Basic audio presence checks |
| `high` | 48 kHz | 24-bit | Media playback quality testing |
| `ultra` | 96 kHz | 32-bit | High-fidelity audio analysis |

### Audio Sources

| Device | Platform | Captures |
|--------|----------|----------|
| `default` | Any | System audio output |
| `adb` | Android | Device audio via ADB |

---

## Log Collection

### Android (Logcat)

Crash and ANR detection pulls relevant lines from logcat:

```bash
adb logcat -d -t 100 *:E
```

The detector filters for:

- `FATAL EXCEPTION` -- application crash
- `ANR in` -- Application Not Responding
- `Process: <package>, PID:` -- process death
- `java.lang.` / `kotlin.` -- unhandled exceptions

### Web (Browser Console)

Playwright captures browser console output. The detector filters for:

- `console.error` messages
- Uncaught exceptions
- Failed network requests (4xx, 5xx)

### Desktop (Process Stderr)

Desktop applications are monitored via their stderr output and process exit codes.

---

## Stack Traces

When a crash is detected, HelixQA extracts and stores the full stack trace. Stack traces appear in:

1. The detection result JSON (`stack_trace` field)
2. The generated issue ticket (reproduction steps section)
3. The session report (findings detail)

---

## Report Formats

### Markdown

Human-readable report with tables, finding summaries, and evidence links. Ideal for reviewing in a text editor or rendering in a documentation system:

```bash
helixqa report --input qa-results/session-123 --format markdown
```

Output: `pipeline-report.md`

### HTML

Standalone HTML page with embedded CSS. Opens in any browser without dependencies. Includes collapsible sections for each test case and inline evidence thumbnails:

```bash
helixqa report --input qa-results/session-123 --format html
```

Output: `pipeline-report.html`

### JSON

Machine-readable format containing the complete session data. Use this for programmatic analysis, dashboard integration, or custom reporting:

```bash
helixqa report --input qa-results/session-123 --format json
```

Output: `pipeline-report.json`

### Report Contents

Every report format includes:

| Section | Description |
|---------|-------------|
| Session summary | Pass number, platform list, duration, coverage ratio |
| Test results | Per-test pass/fail status, step details, evidence paths |
| Findings | Severity-ranked list of detected issues |
| Crash/ANR summary | Count and details of crashes and ANRs per platform |
| Coverage map | Features tested vs. total features discovered |
| Evidence index | File paths to all screenshots, videos, and logs |

---

## Analysing Evidence

### Post-Session Review Workflow

1. **Start with the markdown report** -- read the session summary and scan the findings list for critical and high severity issues
2. **Review failed test screenshots** -- open the `screenshots/` directory and inspect pre/post images for each failed step
3. **Watch video recordings** -- videos provide temporal context. Look for transitions, timing issues, and behaviours that screenshots miss
4. **Read issue tickets** -- each ticket in `docs/issues/` has reproduction steps, expected vs. actual behaviour, and evidence links
5. **Check performance metrics** -- look for memory growth, increasing response times, or CPU spikes

### Programmatic Analysis

Parse the JSON report for automated analysis:

```bash
# Count findings by severity
cat pipeline-report.json | \
    jq '[.findings[] | .severity] | group_by(.) |
    map({severity: .[0], count: length})'

# List all screenshots for failed tests
cat pipeline-report.json | \
    jq '.test_results[] | select(.status == "failed") |
    .evidence.screenshots[]'

# Extract crash stack traces
cat pipeline-report.json | \
    jq '.findings[] | select(.category == "crash") |
    .stack_trace'
```

### Video Frame Analysis

For Android screenshot-to-video captures, the individual frames are preserved in `<device>-frames/`. Inspect specific moments:

```bash
# View frame 42 from the capture sequence
feh qa-results/video-sessions-123/device-frames/frame-0042.png
```

---

## Storage Management

Evidence files accumulate across sessions. Manage disk usage with these strategies:

| Strategy | Command | Effect |
|----------|---------|--------|
| Keep last N sessions | `ls -d qa-results/session-* \| head -n -5 \| xargs rm -rf` | Delete all but 5 newest |
| Delete video only | `find qa-results/ -name "*.mp4" -delete` | Keep screenshots, remove videos |
| Compress old sessions | `tar czf session-old.tar.gz qa-results/session-123/` | Archive and remove original |

## Related Pages

- [Autonomous QA](/guides/autonomous-qa) -- how evidence collection fits into the pipeline
- [Configuration](/reference/config) -- all recording and output configuration fields
- [CLI Reference](/reference/cli) -- `--record`, `--output`, `--report` flags
- [Challenges](/guides/challenges) -- adding evidence to challenge results
