go test ./... -coverprofile=/tmp/coverage.out;
grep -v "harmony-one/harmony/core" /tmp/coverage.out | grep -v "harmony-one/harmony/internal/trie" | grep -v "harmony-one/harmony/internal/db" | grep -v "harmony-one/harmony/log"  > /tmp/coverage1.out
go tool cover -func=/tmp/coverage1.out