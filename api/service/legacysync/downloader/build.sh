
# if command fails, try this
# docker buildx create --use
docker buildx build -t frozen621/harmony-proto:latest --platform linux/amd64,linux/arm64 -f Proto.Dockerfile --progress=plain .