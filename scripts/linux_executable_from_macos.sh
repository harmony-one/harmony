#!/bin/bash
set -e  # Exit on first error

# Define paths
SCRIPT_DIR=$(dirname "$0")
PROJECT_ROOT=$(realpath "$SCRIPT_DIR/..")
HARMONY_DIR="$PROJECT_ROOT"
DOCKER_IMAGE="harmony-one"  # Name of the Docker image
OUTPUT_DIR="$HARMONY_DIR/bin/linux_static_bin"  # Folder for generated binaries

echo "=== Paths ==="
echo "SCRIPT_DIR: $SCRIPT_DIR"
echo "PROJECT_ROOT: $PROJECT_ROOT"
echo "HARMONY_DIR: $HARMONY_DIR"
echo "OUTPUT_DIR: $OUTPUT_DIR"

# Ensure output directory exists and clean old binaries
mkdir -p "$OUTPUT_DIR"
rm -f "$OUTPUT_DIR/harmony" "$OUTPUT_DIR/bootnode"

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "Docker is not running. Please start Docker and try again."
    exit 1
fi

# Build Docker image if not found
if ! docker image inspect "$DOCKER_IMAGE" > /dev/null 2>&1; then
    echo "Building Docker image '$DOCKER_IMAGE'..."
    pushd "$PROJECT_ROOT/scripts/macos_docker"
    docker build --platform linux/amd64 -t "$DOCKER_IMAGE" -f Dockerfile .
    popd
else
    echo "Docker image '$DOCKER_IMAGE' already exists. Skipping build."
fi

# Run the container in the background
echo "Running Docker container..."
CONTAINER_ID=$(docker run --platform linux/amd64 -d "$DOCKER_IMAGE" tail -f /dev/null)
trap 'docker stop "$CONTAINER_ID" > /dev/null; docker rm "$CONTAINER_ID" > /dev/null' EXIT  # Ensure cleanup on exit

# Copy project files into the container
echo "Copying project files to Docker container..."
docker cp "$HARMONY_DIR/." "$CONTAINER_ID:/root/go/src/github.com/harmony-one/tmp"

# Build binaries inside the container
echo "Building Linux static binary..."
docker exec "$CONTAINER_ID" bash -c "
    set -e
    cd /root/go/src/github.com/harmony-one
    echo 'Current Directory: ' && pwd
    rm -rf harmony
    mv tmp harmony
    cd harmony/bin && rm -rf *  # Clear previous binaries
    cd ..
    git config --global --add safe.directory /root/go/src/github.com/harmony-one/harmony
    make linux_static
"

# Copy built binaries back to host machine
echo "Copying binaries to output directory..."
docker cp "$CONTAINER_ID:/root/go/src/github.com/harmony-one/harmony/bin/harmony" "$OUTPUT_DIR"
docker cp "$CONTAINER_ID:/root/go/src/github.com/harmony-one/harmony/bin/bootnode" "$OUTPUT_DIR"

# Validate build output
if [ "$(ls -A "$OUTPUT_DIR")" ]; then
    echo "✅ Build completed successfully. Binaries are in '$OUTPUT_DIR'."
else
    echo "❌ Build failed: No binaries found in '$OUTPUT_DIR'."
    exit 1
fi