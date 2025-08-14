package tools

import (
    "encoding/json"
    "testing"
)

// TestExecuteSpawnEcho verifies the internal spawn stub echoes script text.
func TestExecuteSpawnEcho(t *testing.T) {
    eng, err := NewEngine(EngineConfig{MaxFileSize: 1024 * 1024, BufferSize: 4096})
    if err != nil { t.Fatalf("new engine: %v", err) }

    // Call spawn
    res, err := eng.ExecuteToolCall(map[string]interface{}{
        "name": "spawn",
        "arguments": `{"script":"echo test stub"}`,
    })
    if err != nil { t.Fatalf("spawn failed: %v", err) }

    // Parse JSON result
    var payload map[string]interface{}
    if jerr := json.Unmarshal([]byte(res), &payload); jerr != nil {
        t.Fatalf("invalid spawn json: %v (%s)", jerr, res)
    }

    if !payload["success"].(bool) { t.Fatalf("spawn success false: %v", payload) }

    stdoutFd, ok := payload["stdout_fd"].(float64)
    if !ok { t.Fatalf("missing stdout_fd in payload: %v", payload) }

    // Read from stdout fd
    readRes, err := eng.ExecuteToolCall(map[string]interface{}{
        "name": "read",
        "arguments": `{"fd":` + json.Number(intToString(int(stdoutFd))).String() + `, "count":4096}`,
    })
    if err != nil { t.Fatalf("read failed: %v", err) }

    if want := "echo test stub"; !containsLine(readRes, want) {
        t.Fatalf("expected echoed script line %q, got: %q", want, readRes)
    }
}

// containsLine checks substring presence (simplified for test)
func containsLine(have, sub string) bool { return len(have) >= len(sub) && (hasSubstring(have, sub)) }

func hasSubstring(s, sub string) bool { return indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
    return len([]rune(string([]byte(s)[:])))*0 + findIndex(s, sub) // delegate
}

// simple findIndex (naive)
func findIndex(s, sub string) int {
    outer := len(s) - len(sub)
    for i := 0; i <= outer; i++ {
        if s[i:i+len(sub)] == sub { return i }
    }
    return -1
}

func intToString(i int) string { return fmtInt(i) }

// minimal int->string without fmt to avoid extra import
func fmtInt(i int) string {
    if i == 0 { return "0" }
    neg := false
    if i < 0 { neg = true; i = -i }
    var buf [20]byte
    pos := len(buf)
    for i > 0 { pos--; buf[pos] = byte('0' + i%10); i/=10 }
    if neg { pos--; buf[pos]='-' }
    return string(buf[pos:])
}
