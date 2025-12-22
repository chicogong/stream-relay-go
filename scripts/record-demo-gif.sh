#!/bin/bash

# Script to record a streaming demo GIF for the README
# This provides step-by-step instructions for recording

echo "🎬 Stream Relay Demo GIF Recording Guide"
echo ""
echo "This script will guide you through recording a demo GIF."
echo ""

# Check if tools are installed
echo "📋 Checking required tools..."
echo ""

if ! command -v asciinema &> /dev/null; then
    echo "❌ asciinema not installed"
    echo "   Install: brew install asciinema"
    MISSING_TOOLS=1
else
    echo "✅ asciinema installed"
fi

if ! command -v agg &> /dev/null; then
    echo "❌ agg not installed"
    echo "   Install: brew install agg"
    MISSING_TOOLS=1
else
    echo "✅ agg installed"
fi

echo ""

if [ -n "$MISSING_TOOLS" ]; then
    echo "Please install missing tools and run this script again."
    echo ""
    echo "Quick install:"
    echo "  brew install asciinema agg"
    exit 1
fi

# Check if relay is running
echo "📡 Checking if relay is running..."
if ! curl -s http://localhost:8080/healthz > /dev/null 2>&1; then
    echo "❌ Relay is not running!"
    echo ""
    echo "Please start the relay first:"
    echo "  make dev"
    echo ""
    echo "Or in a separate terminal:"
    echo "  ./bin/relay -config configs/config.yaml"
    exit 1
fi

echo "✅ Relay is running"
echo ""

# Recording instructions
echo "🎥 Recording Instructions:"
echo ""
echo "1. The recording will start in 3 seconds"
echo "2. A sample streaming request will be executed"
echo "3. The recording will stop automatically"
echo "4. The GIF will be generated and saved"
echo ""
echo "Press Ctrl+C to cancel, or press Enter to continue..."
read

echo ""
echo "Starting in 3..."
sleep 1
echo "2..."
sleep 1
echo "1..."
sleep 1
echo ""

# Start recording
echo "🔴 Recording started..."
asciinema rec /tmp/demo.cast -c './scripts/demo-streaming.sh' --overwrite

# Convert to GIF
echo ""
echo "🎨 Converting to GIF..."
agg --font-size 16 --line-height 1.4 --fps 15 \
    /tmp/demo.cast docs/images/streaming-demo.gif

# Check if successful
if [ -f docs/images/streaming-demo.gif ]; then
    SIZE=$(du -h docs/images/streaming-demo.gif | cut -f1)
    echo ""
    echo "✅ Demo GIF created successfully!"
    echo "   Location: docs/images/streaming-demo.gif"
    echo "   Size: $SIZE"
    echo ""
    echo "Next steps:"
    echo "  1. Preview the GIF to make sure it looks good"
    echo "  2. Commit: git add docs/images/streaming-demo.gif"
    echo "  3. Push: git commit -m 'docs: add streaming demo GIF' && git push"
else
    echo ""
    echo "❌ Failed to create GIF"
    echo "   Please check the error messages above"
fi

# Cleanup
rm -f /tmp/demo.cast
