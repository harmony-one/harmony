SRC_DIR=$(dirname $0)
protoc -I ${SRC_DIR}/proto/ ${SRC_DIR}/proto/downloader.proto --go_out=plugins=grpc:${SRC_DIR}/proto
