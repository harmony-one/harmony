#!/usr/bin/env python3
# This script is to be called from `./test/test_pr.sh` within the harmonyone/test-pr docker image

import json
import sys

input_iter = iter(reversed(list(sys.stdin)))
try:
    while True:
        nxt = next(input_iter)
        if '}' in nxt:
            line = '}'
            while True:
                nxt = next(input_iter)
                line = nxt + line
                if '{' in nxt:
                    for k,v in json.loads(line).items():
                        if v != True:
                            print(f"Test {k} failed")
                            exit(1)
                    print("Passed all tests!")
                    exit(0)
except Exception:
    print(f"Did not finish tests!")
    exit(1)

