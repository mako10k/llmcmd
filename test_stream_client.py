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
