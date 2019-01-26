#!/bin/sh
exec git ls-files '*.go' | grep -v \
	-e '^vendor/' \
	-e '\.pb\.go$' \
	-e '/mock_stream\.go' \
	-e '/host_mock\.go' \
	-e '/gen_[^/]*\.go'
