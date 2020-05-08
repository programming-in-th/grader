#!/usr/bin/python
import sys
import filecmp

# ignore first argument
inputFile = sys.argv[1]
outputFile = sys.argv[2]
solFile = sys.argv[3]

if filecmp.cmp(outputFile, solFile) == True:
    print("Correct")
    print("100")
else:
    print("Incorrect")
    print("0")
