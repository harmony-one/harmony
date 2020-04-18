#!/bin/bash
pkill -9 '^(harmony|soldier|commander|profiler)$' | sed 's/^/Killed process: /'
rm -rf db-127.0.0.1-*
