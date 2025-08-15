use std::io::{Read, Write, Seek, SeekFrom};
use std::env;
use std::sync::Arc;
use std::thread;
use std::os::unix::io::RawFd;
use std::fs::{File, OpenOptions};
use serde::{Deserialize, Serialize};
use serde_json::json;
use anyhow::{Result, bail};
use std::collections::HashMap;
use base64::{engine::general_purpose, Engine as _};
use nix::fcntl::{fcntl, FcntlArg};
use std::os::unix::io::{AsRawFd, FromRawFd};
use std::os::unix::net::UnixStream;
use tempfile::tempfile;

#[derive(Deserialize)]
struct Request {
    id: String,
    op: String,
    #[serde(default)]
    params: serde_json::Value,
    #[allow(dead_code)]
    #[serde(default)]
    version: Option<u32>,
}

#[derive(Serialize)]
struct Response {
    id: String,
    ok: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    result: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<ErrorBody>,
}

#[derive(Serialize)]
struct ErrorBody {
    code: String,
    message: String,
}

fn read_frame(stdin: &mut impl Read) -> Result<Option<Vec<u8>>> {
    let mut len_buf = [0u8;4];
    let mut read_total = 0;
    while read_total < 4 {
        let n = stdin.read(&mut len_buf[read_total..])?;
        if n == 0 { // EOF
            if read_total == 0 { return Ok(None); }
            bail!("unexpected EOF while reading length header");
        }
        read_total += n;
    }
    let len = u32::from_be_bytes(len_buf) as usize;
    let mut buf = vec![0u8; len];
    let mut off = 0;
    while off < len {
        let n = stdin.read(&mut buf[off..])?;
        if n == 0 { bail!("unexpected EOF in frame body"); }
        off += n;
    }
    Ok(Some(buf))
}

// Bridge mode: client_fd <-> upstream_fd (both full-duplex sockets)
fn run_bridge(client_fd: RawFd, upstream_fd: RawFd) -> Result<()> {
    // SAFETY: caller guarantees these are valid socket fds
    let mut client = unsafe { UnixStream::from_raw_fd(client_fd) };
    let mut upstream = unsafe { UnixStream::from_raw_fd(upstream_fd) };
    client.set_nonblocking(false)?; upstream.set_nonblocking(false)?;
    let mut client_tx = client.try_clone()?; // for writing back to client
    let mut upstream_tx = upstream.try_clone()?; // for writing upstream
    let t_cu = thread::spawn(move || -> Result<()> { // client -> upstream
        let mut buf=[0u8;8192];
        loop { let n = client.read(&mut buf)?; if n==0 { break; } upstream_tx.write_all(&buf[..n])?; upstream_tx.flush()?; }
        Ok(())
    });
    let t_uc = thread::spawn(move || -> Result<()> { // upstream -> client
        let mut buf=[0u8;8192];
        loop { let n = upstream.read(&mut buf)?; if n==0 { break; } client_tx.write_all(&buf[..n])?; client_tx.flush()?; }
        Ok(())
    });
    let _ = t_cu.join().unwrap_or(Ok(()));
    let _ = t_uc.join().unwrap_or(Ok(()));
    Ok(())
}

fn write_frame(stdout: &mut impl Write, data: &[u8]) -> Result<()> {
    let len = data.len() as u32;
    stdout.write_all(&len.to_be_bytes())?;
    stdout.write_all(data)?;
    stdout.flush()?;
    Ok(())
}

const MAX_READ: usize = 4096;

enum HandleKind {
    Read(File),
    Write(File),
}

struct VirtualEntry {
    file: File, // original O_TMPFILE-like backing file
}

struct State {
    next: u32,
    handles: HashMap<u32, HandleKind>,
    allow_read: Vec<String>,
    allow_write: Vec<String>,
    virtual_files: HashMap<String, VirtualEntry>,
}

impl State {
    fn new(allow_read: Vec<String>, allow_write: Vec<String>) -> Self {
        State { next: 1, handles: HashMap::new(), allow_read, allow_write, virtual_files: HashMap::new() }
    }
    fn alloc(&mut self, kind: HandleKind) -> u32 {
        let h = self.next; self.next += 1; self.handles.insert(h, kind); h
    }
}

fn path_allowed(list: &[String], path: &str) -> bool {
    list.iter().any(|p| p == path)
}

fn handle(state: &mut State, req: Request) -> Response {
    match req.op.as_str() {
        "ping" => ok(req.id, json!({"pong": true})),
        "init" => ok(req.id, json!({"status":"ready"})), // kept for compatibility; no-op
        "open_read" => {
            let path = match req.params.get("path").and_then(|v| v.as_str()) { Some(p)=>p, None=> return err(req.id, "E_ARG", "missing path") };
            if path_allowed(&state.allow_read, path) || path_allowed(&state.allow_write, path) {
                match File::open(path) {
                    Ok(f) => {
                        let h = state.alloc(HandleKind::Read(f));
                        ok(req.id, json!({"handle": h}))
                    }
                    Err(e) if e.kind() == std::io::ErrorKind::NotFound => err(req.id, "E_NOENT", "not found"),
                    Err(_) => err(req.id, "E_IO", "open failed"),
                }
            } else {
                // virtual path must already exist; no implicit creation
                let ve = match state.virtual_files.get(path) {
                    Some(v) => v,
                    None => return err(req.id, "E_NOENT", "virtual not found"),
                };
                let dup_fd = match fcntl(ve.file.as_raw_fd(), FcntlArg::F_DUPFD(0)) { Ok(fd)=>fd, Err(_)=> return err(req.id, "E_IO", "dup failed") };
                let mut dup_file = unsafe { File::from_raw_fd(dup_fd) };
                let _ = dup_file.seek(SeekFrom::Start(0));
                let h = state.alloc(HandleKind::Read(dup_file));
                ok(req.id, json!({"handle": h, "virtual": true}))
            }
        }
        "open_write" => {
            let path = match req.params.get("path").and_then(|v| v.as_str()) { Some(p)=>p, None=> return err(req.id, "E_ARG", "missing path") };
            let append = req.params.get("append").and_then(|v| v.as_bool()).unwrap_or(false);
            if path_allowed(&state.allow_write, path) {
                let mut opts = OpenOptions::new(); opts.write(true).create(true);
                if append { opts.append(true); } else { opts.truncate(true); }
                match opts.open(path) {
                    Ok(f) => { let h = state.alloc(HandleKind::Write(f)); ok(req.id, json!({"handle": h})) }
                    Err(_) => err(req.id, "E_IO", "open failed"),
                }
            } else {
                // virtual path: create or reuse backing, then dup
                let ve = state.virtual_files.entry(path.to_string()).or_insert_with(|| {
                    let f = tempfile().expect("tempfile");
                    VirtualEntry { file: f }
                });
                if !append {
                    // truncate by reopening a new tempfile (simulate new O_TRUNC semantics)
                    let f = tempfile().expect("tempfile");
                    ve.file = f;
                }
                let dup_fd = fcntl(ve.file.as_raw_fd(), FcntlArg::F_DUPFD(0)).expect("dupfd");
                let dup_file = unsafe { File::from_raw_fd(dup_fd) };
                let h = state.alloc(HandleKind::Write(dup_file));
                ok(req.id, json!({"handle": h, "virtual": true}))
            }
        }
        "read" => {
            let h = match req.params.get("h").and_then(|v| v.as_u64()) { Some(v)=> v as u32, None=> return err(req.id, "E_ARG", "missing h") };
            let max_req = req.params.get("max").and_then(|v| v.as_u64()).map(|v| v as usize).unwrap_or(MAX_READ);
            let max = std::cmp::min(max_req, MAX_READ);
            if max == 0 { return err(req.id, "E_ARG", "max must be > 0"); }
            let readable = match state.handles.get_mut(&h) { Some(e)=>e, None=> return err(req.id, "E_CLOSED", "invalid handle") };
            let f = match readable {
                HandleKind::Read(f) => f,
                HandleKind::Write(_) => return err(req.id, "E_PERM", "not readable"),
            };
            let mut buf = vec![0u8; max];
            match f.read(&mut buf) {
                Ok(n) => {
                    buf.truncate(n);
                    let b64 = general_purpose::STANDARD.encode(&buf);
                    let eof = n == 0; // zero read => EOF
                    ok(req.id, json!({"eof": eof, "data": b64}))
                }
                Err(_) => err(req.id, "E_IO", "read failed"),
            }
        }
        "write" => {
            let h = match req.params.get("h").and_then(|v| v.as_u64()) { Some(v)=> v as u32, None=> return err(req.id, "E_ARG", "missing h") };
            let data_b64 = match req.params.get("data").and_then(|v| v.as_str()) { Some(s)=>s, None=> return err(req.id, "E_ARG", "missing data") };
            let entry = match state.handles.get_mut(&h) { Some(e)=>e, None=> return err(req.id, "E_CLOSED", "invalid handle") };
            match entry {
                HandleKind::Write(f) => {
                    let decoded = match general_purpose::STANDARD.decode(data_b64) { Ok(d)=>d, Err(_)=> return err(req.id, "E_ARG", "bad base64") };
                    match f.write(&decoded) {
                        Ok(n) => ok(req.id, json!({"written": n})),
                        Err(_) => err(req.id, "E_IO", "write failed"),
                    }
                }
                HandleKind::Read(_) => err(req.id, "E_PERM", "not writable"),
            }
        }
        "close" => {
            let h = match req.params.get("h").and_then(|v| v.as_u64()) { Some(v)=> v as u32, None=> return err(req.id, "E_ARG", "missing h") };
            match state.handles.remove(&h) {
                Some(_) => ok(req.id, json!({"closed": true})),
                None => err(req.id, "E_CLOSED", "invalid handle"),
            }
        }
        other => unsupported_with(req.id, other)
    }
}

fn main() -> Result<()> {
    // Modes:
    // 1) Origin: vfsd --client-vfs-fd <fd> [-i path ... -o path ...]
    // 2) Bridge: vfsd --client-vfs-fd <fd> --vfs-fd <upstream-fd>
    // Legacy stdin/stdout fallback has been removed (fail-fast if omitted).
    let mut args = env::args().skip(1);
    let mut client_fd: Option<i32> = None;
    let mut upstream_fd: Option<i32> = None;
    let mut ins: Vec<String> = Vec::new();
    let mut outs: Vec<String> = Vec::new();
    while let Some(a) = args.next() {
        match a.as_str() {
            "--client-vfs-fd" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("--client-vfs-fd requires value"))?; let fd: i32 = v.parse().map_err(|_| anyhow::anyhow!("invalid fd"))?; client_fd = Some(fd); },
            "--vfs-fd" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("--vfs-fd requires value"))?; let fd: i32 = v.parse().map_err(|_| anyhow::anyhow!("invalid fd"))?; upstream_fd = Some(fd); },
            "-i" | "--input" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("missing value after -i"))?; ins.push(v); },
            "-o" | "--output" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("missing value after -o"))?; outs.push(v); },
            "-h" | "--help" => { eprintln!("usage: vfsd --client-vfs-fd <fd> [--vfs-fd <upstream-fd>] [-i path ... -o path ...]\n       --client-vfs-fd is mandatory. Bridge mode disallows -i/-o."); return Ok(()); },
            other => { bail!("unknown arg: {}", other); }
        }
    }
    if upstream_fd.is_some() && (!ins.is_empty() || !outs.is_empty()) { bail!("--vfs-fd cannot be combined with -i/-o (origin allow lists)"); }
    let cfd = client_fd.ok_or_else(|| anyhow::anyhow!("--client-vfs-fd is required (legacy stdin/stdout mode removed)"))?;
    if let Some(up_fd) = upstream_fd { return run_bridge(cfd, up_fd); }
    // origin mode over provided socket fd
    let mut state = State::new(ins, outs);
    let mut ctrl = unsafe { File::from_raw_fd(cfd) };
    loop {
        let opt = read_frame(&mut ctrl)?; if opt.is_none() { break; }
        let frame = opt.unwrap();
        let req: Request = match serde_json::from_slice(&frame) {
            Ok(r) => r,
            Err(_) => {
                let resp = Response { id: "?".to_string(), ok: false, result: None, error: Some(ErrorBody{ code:"E_ARG".to_string(), message:"invalid json".to_string()}) };
                let data = serde_json::to_vec(&resp)?; write_frame(&mut ctrl, &data)?; continue;
            }
        };
        let resp = handle(&mut state, req);
        let data = serde_json::to_vec(&resp)?; write_frame(&mut ctrl, &data)?;
    }
    Ok(())
}

// Helper constructors
fn ok(id: String, result: serde_json::Value) -> Response { Response { id, ok: true, result: Some(result), error: None } }
fn err(id: String, code: &str, msg: &str) -> Response { Response { id, ok: false, result: None, error: Some(ErrorBody{ code: code.to_string(), message: msg.to_string() }) } }
fn unsupported_with(id: String, op: &str) -> Response { err(id, "E_UNSUPPORTED", op) }
