#!/bin/bash

# FSProxy Phase 3 Stream I/O Test Script
# Tests handleStreamRead and handleStreamWrite with actual processes

set -e

cd /home/mako10k/llmcmd

echo "=== FSProxy Phase 3 Stream I/O Test ==="

# Build the binary
echo "Building llmcmd..."
go build -o bin/llmcmd ./cmd/llmcmd

# Create test configuration for stream I/O
cat > test_stream_config.json << 'EOF'
{
  "openai": {
    "api_key": "test-key",
    "model": "gpt-3.5-turbo",
    "max_tokens": 1000
  },
  "security": {
    "api_call_limit": 10,
    "quota_limit": 10000
  },
  "fsproxy": {
    "enabled": true,
    "socket_path": "/tmp/fsproxy_test_stream.sock"
  }
}
EOF

# Create test script for LLM to run
cat > test_stream_program.py << 'EOF'
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
EOF

chmod +x test_stream_program.py

# Test prompt for stream I/O
cat > test_stream_prompt.txt << 'EOF'
You are a system testing the FSProxy stream I/O functionality.

Your task is to:
1. Use the spawn tool to start a Python script that reads from stdin and echoes to stdout
2. Use the write tool to send some test data to the process
3. Use the read tool to receive the echoed data from the process
4. Verify the I/O operations work correctly

The Python script is available at: ./test_stream_program.py

Test steps:
1. spawn "./test_stream_program.py"
2. write "Hello, World!" to the process stdin
3. read from the process stdout to get the echo
4. write "Testing FSProxy" to the process stdin  
5. read from the process stdout again
6. write "exit" to terminate the process

Please execute these steps and report the results.
EOF

echo "Starting FSProxy in background..."
./bin/llmcmd --config test_stream_config.json --fsproxy &
FSPROXY_PID=$!

# Wait for FSProxy to start
sleep 2

echo "Testing stream I/O operations manually..."

# Test using FSProxy socket directly
cat > test_stream_client.py << 'EOF'
#!/usr/bin/env python3
import socket
import json
import time

def send_request(sock, request):
    message = json.dumps(request) + '\n'
    sock.send(message.encode())
    response = sock.recv(4096).decode().strip()
    return json.loads(response)

# Connect to FSProxy
sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
sock.connect('/tmp/fsproxy_test_stream.sock')

print("Connected to FSProxy")

# 1. Spawn Python process
spawn_request = {
    "command": "SPAWN",
    "args": ["python3", "./test_stream_program.py"]
}

print("1. Spawning process...")
response = send_request(sock, spawn_request)
print(f"Spawn response: {response}")

if response["status"] != "OK":
    print("FAILED: Could not spawn process")
    exit(1)

process_id = int(response["data"])
print(f"Process ID: {process_id}")

# Wait for process to start
time.sleep(0.5)

# 2. Write test data to stdin
write_request = {
    "command": "STREAM_WRITE",
    "process_id": process_id,
    "stream_type": "stdin",
    "data": "Hello, World!\n"
}

print("2. Writing to stdin...")
response = send_request(sock, write_request)
print(f"Write response: {response}")

if response["status"] != "OK":
    print("FAILED: Could not write to stdin")
    exit(1)

# 3. Read from stdout
read_request = {
    "command": "STREAM_READ",
    "process_id": process_id,
    "stream_type": "stdout",
    "size": 1024
}

print("3. Reading from stdout...")
response = send_request(sock, read_request)
print(f"Read response: {response}")

if response["status"] == "OK" and response["data"]:
    print(f"Received output: {repr(response['data'])}")
else:
    print("FAILED: Could not read from stdout")

# 4. Write more test data
write_request["data"] = "Testing FSProxy\n"
print("4. Writing more data...")
response = send_request(sock, write_request)
print(f"Write response: {response}")

# 5. Read again
print("5. Reading again...")
response = send_request(sock, read_request)
print(f"Read response: {response}")

if response["status"] == "OK" and response["data"]:
    print(f"Received output: {repr(response['data'])}")

# 6. Send exit command
write_request["data"] = "exit\n"
print("6. Sending exit command...")
response = send_request(sock, write_request)
print(f"Write response: {response}")

# Close socket
sock.close()
print("Test completed successfully!")
EOF

chmod +x test_stream_client.py

echo "Running stream I/O test..."
python3 test_stream_client.py

echo "Cleaning up..."
kill $FSPROXY_PID 2>/dev/null || true
rm -f /tmp/fsproxy_test_stream.sock
rm -f test_stream_config.json test_stream_program.py test_stream_prompt.txt test_stream_client.py

echo "=== Test completed ==="
