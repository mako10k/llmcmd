#!/usr/bin/env python3
import sys
import time

print("Python script started", file=sys.stderr)

# Read from stdin and echo to stdout
for line in sys.stdin:
    line = line.strip()
    if line == "exit":
        break
    print(f"Echo: {line}")
    sys.stdout.flush()

print("Python script finished", file=sys.stderr)
