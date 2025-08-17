package builtin

import (
    "bytes"
    "strings"
    "testing"
)

func TestSedBasicFirstOnly(t *testing.T) {
    in := strings.NewReader("foo bar foo\nfoo\n")
    var out bytes.Buffer
    if err := Sed([]string{"s/foo/FOO/"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := out.String()
    want := "FOO bar foo\nFOO\n"
    if got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedGlobal(t *testing.T) {
    in := strings.NewReader("foo bar foo\nFOO foo\n")
    var out bytes.Buffer
    if err := Sed([]string{"s/foo/x/g"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := out.String()
    want := "x bar x\nFOO x\n"
    if got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedIgnoreCase(t *testing.T) {
    in := strings.NewReader("Error error ERROR\n")
    var out bytes.Buffer
    if err := Sed([]string{"s/error/ok/i"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := out.String()
    want := "ok error ERROR\n"
    if got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedIgnoreCaseGlobal(t *testing.T) {
    in := strings.NewReader("Error error ERROR\n")
    var out bytes.Buffer
    if err := Sed([]string{"s/error/ok/ig"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := out.String()
    want := "ok ok ok\n"
    if got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedErrors(t *testing.T) {
    var out bytes.Buffer
    if err := Sed([]string{}, strings.NewReader(""), &out); err == nil {
        t.Fatalf("expected error for missing expr")
    }
    if err := Sed([]string{"x/foo/bar/"}, strings.NewReader(""), &out); err == nil {
        t.Fatalf("expected error for unsupported command")
    }
    if err := Sed([]string{"s/[foo/bar/"}, strings.NewReader(""), &out); err == nil {
        t.Fatalf("expected error for invalid regex")
    }
}
