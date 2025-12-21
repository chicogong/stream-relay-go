# Creating Demo Materials

This guide explains how to create screenshots and recordings for documentation.

## 📸 Screenshots Needed

### 1. Grafana Dashboard (`docs/images/grafana-dashboard.png`)

**Steps:**
1. Start the relay and monitoring stack:
   ```bash
   # Terminal 1: Start relay
   make dev

   # Terminal 2: Start Grafana
   cd deployments/grafana
   docker-compose up -d
   ```

2. Generate some traffic:
   ```bash
   ./test_relay.sh
   ```

3. Open Grafana at http://localhost:3000
   - Login: admin/admin
   - Navigate to the "Stream Relay Monitoring" dashboard
   - Wait for panels to populate with data (15-30 seconds)

4. Take screenshot:
   - Use full-screen mode (press 'f' in Grafana)
   - Capture the entire dashboard
   - Recommended resolution: 1920x1080 or higher
   - Save as `docs/images/grafana-dashboard.png`

**What to show:**
- All 8 panels with live data
- Success rate showing 100%
- Request rate showing activity
- Response time charts with actual latencies
- Heatmap showing distribution

### 2. Streaming Demo GIF (`docs/images/streaming-demo.gif`)

**Option A: Using asciinema + agg**

```bash
# Install tools
brew install asciinema agg  # macOS
# or
apt install asciinema  # Linux

# Record session
asciinema rec demo.cast

# In the recording:
# 1. Show the curl command
# 2. Execute streaming request
# 3. Show tokens arriving in real-time
# 4. Press Ctrl+D to stop recording

# Convert to GIF
agg demo.cast docs/images/streaming-demo.gif
```

**Option B: Using terminalizer**

```bash
# Install
npm install -g terminalizer

# Record
terminalizer record streaming-demo

# In the recording:
curl -N http://localhost:8080/v1/chat/completions \
  -H 'Authorization: Bearer sk-relay-test-key-123' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "Qwen/Qwen2.5-7B-Instruct",
    "messages": [{"role": "user", "content": "Write a haiku about streaming"}],
    "stream": true,
    "max_tokens": 50
  }'

# Render to GIF
terminalizer render streaming-demo -o docs/images/streaming-demo.gif
```

**Option C: Screen recording + conversion**

```bash
# Record your terminal with QuickTime/OBS
# Then convert using ffmpeg:
ffmpeg -i recording.mov -vf "fps=10,scale=1200:-1:flags=lanczos" \
  -c:v gif docs/images/streaming-demo.gif
```

### 3. Architecture Diagram (Optional)

If you want to create a proper architecture diagram:

```bash
# Use draw.io or mermaid
# Save as docs/images/architecture.png
```

## 📹 Video Tutorial (Optional)

For a more comprehensive demo:

**Recording checklist:**
1. ✅ Show project structure
2. ✅ Demonstrate configuration
3. ✅ Start the relay
4. ✅ Send streaming request
5. ✅ Show Grafana dashboard
6. ✅ Explain metrics
7. ✅ Demonstrate rate limiting
8. ✅ Show error handling

**Tools:**
- OBS Studio (free, cross-platform)
- QuickTime (macOS)
- SimpleScreenRecorder (Linux)

**Upload to:**
- YouTube
- Asciinema (terminal-only)

## 📐 Image Specifications

### Screenshots
- **Format**: PNG
- **Resolution**: 1920x1080 minimum
- **Compression**: Optimize with `pngquant` or `optipng`
- **File size**: Keep under 500KB

```bash
# Optimize PNG
pngquant docs/images/grafana-dashboard.png --force --output docs/images/grafana-dashboard.png
```

### GIFs
- **Format**: GIF
- **Duration**: 10-30 seconds
- **FPS**: 10-15 (smooth but small size)
- **Resolution**: 1200px width
- **File size**: Keep under 5MB

```bash
# Optimize GIF
gifsicle -O3 --colors 256 docs/images/streaming-demo.gif -o docs/images/streaming-demo-optimized.gif
```

## 🎨 Tips for Great Screenshots

1. **Clean terminal**: Clear scrollback before recording
2. **Readable font**: Use a clear monospace font (JetBrains Mono, Fira Code)
3. **Color scheme**: Use a theme with good contrast (Dracula, Solarized Dark)
4. **Terminal size**: 120x30 or similar standard size
5. **No personal info**: Remove API keys, IP addresses, etc.
6. **Focus**: Highlight what matters, crop unnecessary parts

## 📝 Adding to Documentation

After creating the images:

1. Place in `docs/images/` directory
2. Update README.md references
3. Commit images with descriptive messages:
   ```bash
   git add docs/images/
   git commit -m "docs: add Grafana dashboard screenshot and streaming demo"
   ```

## 🔄 Updating Screenshots

Screenshots should be updated when:
- Dashboard layout changes
- New features are added
- UI significantly changes
- Better quality images available

Keep old versions in `docs/images/archive/` for reference.
