#!/bin/bash

# Define paths
SCRIPT_DIR=$(dirname $0)
PROJECT_ROOT=$(realpath "$SCRIPT_DIR/..")
HARMONY_DIR="$PROJECT_ROOT"
DOCKER_IMAGE="harmony-one"  # This is the name we'll use for the Docker image
CONTAINER_PATH="/harmony"     # The container's path to mount the harmony folder
OUTPUT_DIR="$HARMONY_DIR/bin/linux_static_bin"  # The folder where we want to store the generated binaries

echo "SCRIPT_DIR: $SCRIPT_DIR"
echo "PROJECT_ROOT: $PROJECT_ROOT"
echo "HARMONY_DIR: $HARMONY_DIR"
echo "OUTPUT_DIR: $OUTPUT_DIR"

# Create the output directory if it doesn't exist
mkdir -p $OUTPUT_DIR
rm $OUTPUT_DIR/harmony
rm $OUTPUT_DIR/bootnode

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "Docker is not running. Please start Docker and try again."
    exit 1
fi

# Build the Docker image (if it's not already built)
if ! docker image inspect $DOCKER_IMAGE > /dev/null 2>&1; then
    echo "Docker image '$DOCKER_IMAGE' not found. Building the Docker image from the Dockerfile..."
    echo "build docker file: $PROJECT_ROOT/scripts/macos_docker"
    cd $PROJECT_ROOT/scripts/macos_docker #$PROJECT_ROOT
    docker build -t $DOCKER_IMAGE -f $PROJECT_ROOT/scripts/macos_docker/Dockerfile $PROJECT_ROOT/scripts/macos_docker
else
    echo "Docker image '$DOCKER_IMAGE' already exists. Skipping build."
fi

# Run the Docker container
echo "Running Docker container and get the container ID..."
CONTAINER_ID=$(docker run -d $DOCKER_IMAGE tail -f /dev/null)

# Copy all files from project directory to Docker container
echo "Copying all files from project directory to Docker container..."
docker cp $HARMONY_DIR/. $CONTAINER_ID:/root/go/src/github.com/harmony-one/tmp

# Run the Docker container, mount the harmony folder and output folder, and build the project
echo "Building Linux static binary in Docker container..."
docker exec $CONTAINER_ID bash -c "
    set -e  # Exit on error
    cd "/root/go/src/github.com/harmony-one"
    echo 'Current Directory: ' && pwd
    ls -l
    rm -rf harmony
    mv tmp harmony
    cd harmony/bin
    rm -rf *
    cd ..
    echo 'Current Directory: ' && pwd
    ls -l
    git config --global --add safe.directory /root/go/src/github.com/harmony-one/harmony
    make linux_static
"

echo 'Copying generated harmony and bootnode binaries to output directory...'
docker cp $CONTAINER_ID:/root/go/src/github.com/harmony-one/harmony/bin/harmony $OUTPUT_DIR 
docker cp $CONTAINER_ID:/root/go/src/github.com/harmony-one/harmony/bin/bootnode $OUTPUT_DIR 

# Check if the binaries were copied to the output directory
if [ "$(ls -A $OUTPUT_DIR)" ]; then
    echo "Build completed. The binaries should be in the '$OUTPUT_DIR' directory."
else
    echo "Build failed: No binaries were found in the output directory."
    exit 1
fi
