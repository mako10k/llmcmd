package tools

import (
    "bufio"
    "io"
    "os"
    imux "github.com/mako10k/llmcmd/internal/mux"
)

// localDuplex creates an in-memory full-duplex ReadWriteCloser pair.
// Side A: Reader reads what B writes; Writer writes to B's reader.
// Side B: Reader reads what A writes; Writer writes to A's reader.
type rwc struct{
    io.Reader
    io.Writer
    io.Closer
}

func localDuplex() (a io.ReadWriteCloser, b io.ReadWriteCloser) {
    ar, aw := io.Pipe() // A writes -> ar read by B
    br, bw := io.Pipe() // B writes -> br read by A
    a = rwc{Reader: br, Writer: aw, Closer: closerFunc(func() error { _ = br.Close(); return aw.Close() })}
    b = rwc{Reader: ar, Writer: bw, Closer: closerFunc(func() error { _ = ar.Close(); return bw.Close() })}
    return
}

// channel ids for stdio
const (
    chStdin  byte = 0
    chStdout byte = 1
    chStderr byte = 2
)

// closerFunc adapts a function to an io.Closer (local copy to avoid cross-package dep)
type closerFunc func() error
func (f closerFunc) Close() error { return f() }

// StdioMux bridges OS stdio <-> engine fd0/1/2 via length-prefixed mux frames.
// Engine side is exposed as: stdinR (reader), stdoutW (writer), stderrW (writer).
type StdioMux struct {
    // engine-visible endpoints
    stdinR *io.PipeReader
    stdinW *io.PipeWriter
    stdoutR *io.PipeReader
    stdoutW *io.PipeWriter
    stderrR *io.PipeReader
    stderrW *io.PipeWriter

    // mux endpoints
    eng *imux.Conn // engine side mux
    osd *imux.Conn // OS side mux

    // done channels to stop goroutines
    done chan struct{}
}

// NewStdioMux builds the mux and starts bridging goroutines.
// bufSize controls chunk size for pipe copies.
func NewStdioMux(bufSize int) *StdioMux {
    // Create full-duplex in-memory connection
    a, b := localDuplex()
    sm := &StdioMux{eng: imux.New(a), osd: imux.New(b), done: make(chan struct{})}

    // Engine side endpoints (pipes)
    sm.stdinR, sm.stdinW = io.Pipe()
    sm.stdoutR, sm.stdoutW = io.Pipe()
    sm.stderrR, sm.stderrW = io.Pipe()

    // OS-side goroutines
    // 1) Pump OS stdin -> mux frames (chStdin)
    go func(w *imux.Conn, bufSz int){
        buf := make([]byte, bufSz)
        r := bufio.NewReader(os.Stdin)
        for {
            n, err := r.Read(buf)
            if n > 0 {
                payload := append([]byte{chStdin}, buf[:n]...)
                if e := w.WriteFrame(payload); e != nil { return }
            }
            if err != nil { return }
        }
    }(sm.osd, bufSize)

    // 2) Demux frames coming from engine to OS stdout/stderr
    go func(r *imux.Conn){
        for {
            frame, err := r.ReadFrame()
            if err != nil { return }
            if len(frame) == 0 { continue }
            ch := frame[0]
            data := frame[1:]
            switch ch {
            case chStdout:
                if len(data) > 0 { if _, e := os.Stdout.Write(data); e != nil { return } }
            case chStderr:
                if len(data) > 0 { if _, e := os.Stderr.Write(data); e != nil { return } }
            case chStdin:
                // Ignore unexpected stdin from engine on OS side
            default:
                // Unknown channel, drop
            }
        }
    }(sm.osd)

    // Engine-side goroutines
    // 3) Demux frames from OS side to engine stdin pipe
    go func(r *imux.Conn, dst *io.PipeWriter){
        defer dst.Close()
        for {
            frame, err := r.ReadFrame()
            if err != nil { return }
            if len(frame) == 0 { continue }
            if frame[0] != chStdin { continue }
            if len(frame) > 1 {
                if _, e := dst.Write(frame[1:]); e != nil { return }
            }
        }
    }(sm.eng, sm.stdinW)

    // 4) Pump engine stdout/stderr writers into mux frames
    go func(src *io.PipeReader, w *imux.Conn, ch byte, bufSz int){
        defer src.Close()
        buf := make([]byte, bufSz)
        for {
            n, err := src.Read(buf)
            if n > 0 {
                payload := append([]byte{ch}, buf[:n]...)
                if e := w.WriteFrame(payload); e != nil { return }
            }
            if err != nil { return }
        }
    }(sm.stdoutR, sm.eng, chStdout, bufSize)

    go func(src *io.PipeReader, w *imux.Conn, ch byte, bufSz int){
        defer src.Close()
        buf := make([]byte, bufSz)
        for {
            n, err := src.Read(buf)
            if n > 0 {
                payload := append([]byte{ch}, buf[:n]...)
                if e := w.WriteFrame(payload); e != nil { return }
            }
            if err != nil { return }
        }
    }(sm.stderrR, sm.eng, chStderr, bufSize)

    return sm
}

// Engine-facing endpoints
func (s *StdioMux) StdinReader() io.Reader  { return s.stdinR }
func (s *StdioMux) StdoutWriter() io.Writer { return s.stdoutW }
func (s *StdioMux) StderrWriter() io.Writer { return s.stderrW }

// CloseStdinToEngine closes the writer feeding engine stdin (signals EOF to fd0 readers).
func (s *StdioMux) CloseStdinToEngine() { _ = s.stdinW.Close() }

// Close terminates mux endpoints and internal pipes.
func (s *StdioMux) Close() error {
    _ = s.eng.Close()
    _ = s.osd.Close()
    _ = s.stdinR.Close()
    _ = s.stdoutW.Close()
    _ = s.stderrW.Close()
    return nil
}
