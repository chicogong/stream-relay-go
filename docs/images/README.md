# Documentation Images

This directory contains screenshots and visual assets for the project documentation.

## Required Images

### 1. `grafana-dashboard.png`
**Description**: Full screenshot of the Grafana monitoring dashboard showing all 8 panels with live data

**Specifications:**
- Format: PNG
- Resolution: 1920x1080 or higher
- Content: Complete dashboard view with populated metrics
- File size: < 500KB (optimized)

**How to create:**
1. Run `./scripts/generate-demo.sh` to populate metrics
2. Open http://localhost:3000 in your browser
3. Login with admin/admin
4. Navigate to "Stream Relay Monitoring" dashboard
5. Press 'f' for full-screen mode
6. Take screenshot
7. Optimize: `pngquant grafana-dashboard.png --force -o grafana-dashboard.png`

### 2. `streaming-demo.gif`
**Description**: Animated GIF showing real-time token streaming in action

**Specifications:**
- Format: GIF
- Duration: 10-20 seconds
- FPS: 10-15
- Resolution: 1200px width
- File size: < 5MB

**How to create:**
```bash
# Option 1: Using asciinema
asciinema rec demo.cast
# Execute: curl -N http://localhost:8080/v1/chat/completions ...
# Press Ctrl+D when done
agg demo.cast docs/images/streaming-demo.gif

# Option 2: Using terminalizer
terminalizer record streaming
# Execute streaming request
terminalizer render streaming -o docs/images/streaming-demo.gif
```

## Optional Images

### `architecture.png`
High-level architecture diagram (can be created with draw.io or mermaid)

### `features-*.png`
Individual feature screenshots if needed

## Image Optimization

Always optimize images before committing:

```bash
# PNG
pngquant image.png --force -o image.png
optipng -o7 image.png

# GIF
gifsicle -O3 --colors 256 input.gif -o output.gif
```

## Updating Images

When updating screenshots:
1. Keep the same filename
2. Ensure similar framing/composition
3. Update if UI changes significantly
4. Archive old versions if major changes

## Tools

Recommended tools for creating demo materials:
- **Screenshots**: macOS Screenshot (Cmd+Shift+4), Flameshot, Spectacle
- **GIFs**: asciinema + agg, terminalizer, LICEcap
- **Optimization**: pngquant, optipng, gifsicle
- **Editing**: GIMP, Photopea (web), Preview (macOS)
