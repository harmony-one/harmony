#!/usr/bin/env python3

import argparse
import sys
import binascii

parser = argparse.ArgumentParser(description="Tool to convert your BLS key from binary to the accepted hex format.")
parser.add_argument('file')
parser.add_argument('-o', '--output', type=str, metavar="output", help="name your keyfile")
args = parser.parse_args()

try:
    with open(args.file, 'rb') as inputFile:
        binFormat = inputFile.read()
except IOError:
    print("File couldn't be opened.")
    sys.exit(1)

hexFormat = binascii.hexlify(binFormat)

try:
    if args.output is not None:
        outputName = args.output
    else:
        outputName = 'PUB.key'
    with open(outputName, 'wt') as outputFile:
        outputFile.write(hexFormat.decode('ascii'))
except IOError:
    print("File couldn't be written.")
    sys.exit(1)
