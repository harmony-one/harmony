#!/usr/bin/env python3

import argparse, binascii
import sys, glob, re, os

parser = argparse.ArgumentParser(description='Tool to convert your BLS key from binary to the accepted hex format.')
parser.add_argument('-i', '--input', type=str, metavar='input', help='path to BLS keyfile')
args = parser.parse_args()

## Finds first bls keyfile in the directory or takes file given by user
if args.input:
    inputName = args.input
    print(inputName)
else:
    keylist = glob.glob('UTC*bls_*')
    inputName = keylist[0]

## Read
try:
    with open(inputName, 'rb') as inputFile:
        binFormat = inputFile.read()
except IOError:
    print('File couldn\'t be opened.')
    sys.exit(1)

## Sanity check and conversion
try:
    binascii.unhexlify(binFormat)
except binascii.Error:
    hexFormat = binascii.hexlify(binFormat)
else:
    print("Keyfile is already in text format, no conversion required!")
    sys.exit(0)

## Name output appropriately based on the public key
matchObj = re.search('UTC.*bls_(.*)', inputName)

if matchObj:
    outputName = matchObj.group(1) + ".key"
else:
    outputName = 'bls.key'

## Write
try:
    with open(outputName, 'w') as outputFile:
        outputFile.write(hexFormat.decode('ascii'))
    print("Keyfile converted to: {}".format(outputName))
except IOError:
    print('File couldn\'t be written.')
    sys.exit(1)
