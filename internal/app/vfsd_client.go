package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	imux "github.com/mako10k/llmcmd/internal/mux"
)

// vfsdClient implements tools.VirtualFileSystem backed by vfsd over stdio mux
// It manages a child vfsd process and talks JSON frames with length prefix.
// Minimal operations: OpenFile, CreateTemp, RemoveFile, ListFiles

type vfsdClient struct {
	cmd   *exec.Cmd
	conn  *imux.Conn
	mu    sync.Mutex
	reqID int
	// Track open logical names for ListFiles/RemoveFile
	opened map[string]bool
}

type vfsdRequest struct {
	ID     string                 `json:"id"`
	Op     string                 `json:"op"`
	Params map[string]interface{} `json:"params,omitempty"`
}

type vfsdError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type vfsdResponse struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *vfsdError      `json:"error,omitempty"`
}

// Create vfsd client.
// In attach/proxy mode, if attachFds is non-empty, attach to existing mux FD(s)
// instead of spawning vfsd. Otherwise, spawn vfsd as a child process with -i/-o allowlists.
// attachFds supports either a single full-duplex FD (e.g., "3") or a pair "rfd,wfd".
func newVFSDClient(vfsdPath string, allowRead []string, allowWrite []string, attachFds string) (*vfsdClient, error) {
	// Attach/proxy mode via explicit FD spec
	if attachFds != "" {
		// Parse single or pair
		var rwc io.ReadWriteCloser
		if idx := strings.Index(attachFds, ","); idx > 0 {
			rStr := strings.TrimSpace(attachFds[:idx])
			wStr := strings.TrimSpace(attachFds[idx+1:])
			rfd, err1 := strconv.Atoi(rStr)
			wfd, err2 := strconv.Atoi(wStr)
			if err1 != nil || err2 != nil || rfd < 0 || wfd < 0 {
				return nil, fmt.Errorf("vfsd attach: invalid fd pair '%s'", attachFds)
			}
			// Duplicate both FDs to own lifecycle
			dupR, err := syscall.Dup(rfd)
			if err != nil {
				return nil, fmt.Errorf("vfsd attach: dup rfd %d: %w", rfd, err)
			}
			dupW, err := syscall.Dup(wfd)
			if err != nil {
				_ = syscall.Close(dupR)
				return nil, fmt.Errorf("vfsd attach: dup wfd %d: %w", wfd, err)
			}
			rf := os.NewFile(uintptr(dupR), "vfsd-mux-r")
			wf := os.NewFile(uintptr(dupW), "vfsd-mux-w")
			if rf == nil || wf == nil {
				if rf != nil {
					_ = rf.Close()
				} else {
					_ = syscall.Close(dupR)
				}
				if wf != nil {
					_ = wf.Close()
				} else {
					_ = syscall.Close(dupW)
				}
				return nil, fmt.Errorf("vfsd attach: failed to wrap fd(s)")
			}
			// Wrap separate read/write into a single ReadWriteCloser
			rwc = struct {
				io.Reader
				io.Writer
				io.Closer
			}{Reader: rf, Writer: wf, Closer: closerFunc(func() error { _ = rf.Close(); return wf.Close() })}
		} else {
			// Single full-duplex FD
			fd, err := strconv.Atoi(strings.TrimSpace(attachFds))
			if err != nil || fd < 0 {
				return nil, fmt.Errorf("vfsd attach: invalid fd '%s'", attachFds)
			}
			dupFd, err := syscall.Dup(fd)
			if err != nil {
				return nil, fmt.Errorf("vfsd attach: dup fd %s failed: %w", attachFds, err)
			}
			f := os.NewFile(uintptr(dupFd), "vfsd-mux")
			if f == nil {
				_ = syscall.Close(dupFd)
				return nil, fmt.Errorf("vfsd attach: failed to wrap fd %d", dupFd)
			}
			rwc = f
		}
	conn := imux.New(rwc)
		c := &vfsdClient{cmd: nil, conn: conn, opened: make(map[string]bool)}
		return c, nil
	}

	if vfsdPath == "" {
		vfsdPath = "vfsd"
	}
	args := make([]string, 0, len(allowRead)*2+len(allowWrite)*2)
	for _, p := range allowRead {
		if p != "" {
			args = append(args, "-i", p)
		}
	}
	for _, p := range allowWrite {
		if p != "" {
			args = append(args, "-o", p)
		}
	}
	cmd := exec.Command(vfsdPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("vfsd: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("vfsd: stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr // surface server errors to parent stderr for diagnostics
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("vfsd: start failed: %w", err)
	}

	conn := imux.New(struct {
		io.Reader
		io.Writer
		io.Closer
	}{Reader: stdout, Writer: stdin, Closer: stdin})
	c := &vfsdClient{cmd: cmd, conn: conn, opened: make(map[string]bool)}
	return c, nil
}

// closerFunc adapts a function to an io.Closer
type closerFunc func() error

func (f closerFunc) Close() error { return f() }

func (c *vfsdClient) nextID() string { c.reqID++; return fmt.Sprintf("%d", c.reqID) }

func (c *vfsdClient) do(op string, params map[string]interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	req := vfsdRequest{ID: c.nextID(), Op: op, Params: params}
	data, _ := json.Marshal(req)
	if err := c.conn.WriteFrame(data); err != nil {
		return nil, err
	}
	frame, err := c.conn.ReadFrame()
	if err != nil {
		return nil, err
	}
	var resp vfsdResponse
	if err := json.Unmarshal(frame, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, fmt.Errorf("vfsd %s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("vfsd: operation failed")
	}
	return resp.Result, nil
}

// file handle wrapper implementing io.ReadWriteCloser mapped to vfsd handle id

type vfsdHandle struct {
	client *vfsdClient
	h      int
	// readable/writable enforced by server; we only gate Write/Read by calling server
	closed bool
}

func (h *vfsdHandle) Read(p []byte) (int, error) {
	if h.closed {
		return 0, io.ErrClosedPipe
	}
	max := len(p)
	res, err := h.client.do("read", map[string]interface{}{"h": h.h, "max": max})
	if err != nil {
		return 0, err
	}
	var rr struct {
		EOF  bool   `json:"eof"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(res, &rr); err != nil {
		return 0, err
	}
	if rr.Data == "" && rr.EOF {
		return 0, io.EOF
	}
	raw, err := base64.StdEncoding.DecodeString(rr.Data)
	if err != nil {
		return 0, err
	}
	n := copy(p, raw)
	if n < len(raw) {
		// Should not happen since server limits by max
	}
	if rr.EOF && n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (h *vfsdHandle) Write(p []byte) (int, error) {
	if h.closed {
		return 0, io.ErrClosedPipe
	}
	b64 := base64.StdEncoding.EncodeToString(p)
	_, err := h.client.do("write", map[string]interface{}{"h": h.h, "data": b64})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (h *vfsdHandle) Close() error {
	if h.closed {
		return nil
	}
	_, err := h.client.do("close", map[string]interface{}{"h": h.h})
	h.closed = true
	return err
}

// tools.VirtualFileSystem methods

func (c *vfsdClient) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	mode := "r"
	// Translate os flags to simple modes expected by server
	w := flag&os.O_WRONLY != 0
	rw := flag&os.O_RDWR != 0
	append := flag&os.O_APPEND != 0
	// truncate/create flags are implicit in chosen mode for server; not carried separately
	_ = flag
	_ = perm
	if rw {
		mode = "rw"
	} else if w && append {
		mode = "a"
	} else if w {
		mode = "w"
	} else {
		mode = "r"
	}
	// normalize logical vs real path: server distinguishes by allowlists
	params := map[string]interface{}{"path": name, "mode": mode}
	res, err := c.do("open", params)
	if err != nil {
		return nil, err
	}
	var r struct {
		Handle int `json:"handle"`
	}
	if err := json.Unmarshal(res, &r); err != nil {
		return nil, err
	}
	c.opened[name] = true
	return &vfsdHandle{client: c, h: r.Handle}, nil
}

func (c *vfsdClient) CreateTemp(pattern string) (io.ReadWriteCloser, string, error) {
	// model temp as virtual path unique by pattern prefix
	path := pattern
	if path == "" {
		path = "<tmp>"
	}
	f, err := c.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, "", err
	}
	return f, path, nil
}

func (c *vfsdClient) RemoveFile(name string) error {
	// Server has no explicit unlink; for virtual we can just forget; for real, it's not allowed to remove.
	// Fail-first: deny removal of real paths through client.
	if name != "" && !strings.HasPrefix(name, "<") {
		return fmt.Errorf("remove denied for real path: %s", name)
	}
	delete(c.opened, name)
	return nil
}

func (c *vfsdClient) ListFiles() []string {
	out := make([]string, 0, len(c.opened))
	for k := range c.opened {
		out = append(out, k)
	}
	return out
}

func (c *vfsdClient) Close() error {
	_ = c.conn.Close()
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
	return nil
}
