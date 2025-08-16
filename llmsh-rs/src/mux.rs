use anyhow::Result;
use std::io::{BufReader, Read, Write};
use std::collections::HashMap;
use std::env;
use std::os::unix::io::{RawFd, FromRawFd};
use std::os::fd::IntoRawFd;
use nix::fcntl::{fcntl, FcntlArg, FdFlag};
use nix::unistd::{dup, read as nread, close};
use libc;
use nix::sys::socket::{socketpair, AddressFamily, SockType, SockFlag};
use serde_json::json;
use base64::{engine::general_purpose, Engine as _};

// (Legacy VfsClient removed: asynchronous MUX now handles upstream frames directly)
#[derive(Debug)]
pub enum VfsErrorCode { EArg, ENoEnt, EPerm, EIO, EClosed, EUnsupported, Other(String) }
fn map_code(c:&str)->VfsErrorCode { match c {"E_ARG"=>VfsErrorCode::EArg,"E_NOENT"=>VfsErrorCode::ENoEnt,"E_PERM"=>VfsErrorCode::EPerm,"E_IO"=>VfsErrorCode::EIO,"E_CLOSED"=>VfsErrorCode::EClosed,"E_UNSUPPORTED"=>VfsErrorCode::EUnsupported, other=>VfsErrorCode::Other(other.to_string())} }

pub struct ChildMux { fd_write: RawFd, reader: BufReader<std::fs::File>, seq: u64 }
impl ChildMux {
    fn new(fd: RawFd) -> Result<Self> { let dup_r = dup(fd)?; let read_file = unsafe { std::fs::File::from_raw_fd(dup_r) }; Ok(ChildMux { fd_write: fd, reader: BufReader::new(read_file), seq:0 }) }
    fn next_id(&mut self)->String { self.seq+=1; self.seq.to_string() }
    fn send(&mut self, op:&str, params: serde_json::Value) -> Result<(bool, serde_json::Value)> {
        let id=self.next_id();
        let obj=json!({"id":id,"op":op,"params":params});
        let data=serde_json::to_vec(&obj)?;
        let len=(data.len() as u32).to_be_bytes();
        write_all_fd(self.fd_write, &len)?; write_all_fd(self.fd_write, &data)?;
        let mut len_buf=[0u8;4]; self.reader.read_exact(&mut len_buf)?; let resp_len=u32::from_be_bytes(len_buf) as usize; let mut buf=vec![0u8;resp_len]; self.reader.read_exact(&mut buf)?; let v:serde_json::Value=serde_json::from_slice(&buf)?; let ok=v.get("ok").and_then(|b|b.as_bool()).unwrap_or(false); if !ok { let code=v.get("error").and_then(|e|e.get("code").and_then(|c|c.as_str())).unwrap_or("?").to_string(); return Ok((false, json!({"code": code}))); } Ok((true, v.get("result").cloned().unwrap_or(json!({})))) }
    pub fn read_chunk(&mut self,h:u32,max:u32)->Result<Result<(Vec<u8>,bool),VfsErrorCode>>{
        if h==0 {
            let max = std::cmp::min(max as usize, 4096usize);
            let mut buf=vec![0u8; max];
            match std::io::stdin().read(&mut buf) {
                Ok(n)=>{ buf.truncate(n); let eof = n==0; return Ok(Ok((buf, eof))); },
                Err(_)=>{ return Ok(Err(VfsErrorCode::EIO)); }
            }
        }
        let (ok,res)=self.send("read", json!({"h":h,"max":max}))?; if ok { let b64=res.get("data").and_then(|d|d.as_str()).unwrap_or(""); let mut data=Vec::new(); if !b64.is_empty(){ data=general_purpose::STANDARD.decode(b64).unwrap_or_default(); } let eof=res.get("eof").and_then(|e|e.as_bool()).unwrap_or(false); Ok(Ok((data,eof))) } else { Ok(Err(map_code(res.get("code").and_then(|c|c.as_str()).unwrap_or("?")))) }
    }
    pub fn write_chunk(&mut self,h:u32,data:&[u8])->Result<Result<usize,VfsErrorCode>>{
        if h==1 || h==2 {
            // Write locally to process stdout/stderr
            let wres = if h==1 { std::io::stdout().write(data) } else { std::io::stderr().write(data) };
            return match wres { Ok(n)=>Ok(Ok(n)), Err(_)=>Ok(Err(VfsErrorCode::EIO)) };
        }
        if h==0 { return Ok(Err(VfsErrorCode::EPerm)); }
        // Implement partial write semantics: upstream may write fewer bytes than requested.
        // Loop until all data written or an error occurs.
        let mut total_written = 0usize;
        while total_written < data.len() {
            let remaining = &data[total_written..];
            let b64 = general_purpose::STANDARD.encode(remaining);
            let (ok,res)= self.send("write", json!({"h":h,"data": b64}))?;
            if !ok {
                return Ok(Err(map_code(res.get("code").and_then(|c|c.as_str()).unwrap_or("?"))));
            }
            let w = res.get("written").and_then(|n| n.as_u64()).unwrap_or(0) as usize;
            if w==0 { // Prevent infinite loop; treat as IO error
                return Ok(Err(VfsErrorCode::EIO));
            }
            total_written += w;
            if w > remaining.len() { break; } // protocol violation guard
        }
        Ok(Ok(total_written))
    }
    pub fn close(&mut self,h:u32)->Result<Result<(),VfsErrorCode>>{
        if h<=2 { return Ok(Ok(())); }
        let (ok,_)=self.send("close", json!({"h":h}))?; if ok { Ok(Ok(())) } else { Ok(Err(VfsErrorCode::EClosed)) }
    }
    pub fn open(&mut self, path:&str, mode:&str) -> Result<Result<u32,VfsErrorCode>> {
        let (ok,res)= self.send("open", json!({"path":path, "mode":mode}))?;
        if ok { Ok(Ok(res.get("handle").and_then(|h|h.as_u64()).unwrap_or(0) as u32)) } else { Ok(Err(map_code(res.get("code").and_then(|c|c.as_str()).unwrap_or("?")))) }
    }
}

static mut CHILD_MUX: Option<ChildMux> = None;
pub fn init_child_mux(fd: RawFd) { unsafe { CHILD_MUX = Some(ChildMux::new(fd).expect("child mux init")); } }
pub fn child_mux() -> &'static mut ChildMux { unsafe { CHILD_MUX.as_mut().expect("child mux not initialized") } }

pub fn run_mux(child_parent_fds: Vec<RawFd>, allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> Result<()> {
    struct MuxChild { fd: RawFd, alive: bool, buf: Vec<u8> }
    struct Pending { child_fd: RawFd, child_id: serde_json::Value }
    // Spawn upstream (inline variant of VfsClient::spawn giving us raw fds for async handling)
    fn spawn_upstream(vfsd_path:&str, allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> Result<(RawFd, RawFd)> {
        let (cli_fd_owned, parent_fd_owned) = socketpair(AddressFamily::Unix, SockType::Stream, None, SockFlag::empty())?;
        let cli_fd = cli_fd_owned.into_raw_fd();
        let parent_fd = parent_fd_owned.into_raw_fd();
        let mut cmd = std::process::Command::new(vfsd_path);
        cmd.arg("--client-vfs-fd").arg(cli_fd.to_string());
        if let Some(up_fd) = pass_fd { cmd.arg("--vfs-fd").arg(up_fd.to_string()); }
        else { for p in allow_read { cmd.arg("-i").arg(p); } for p in allow_write { cmd.arg("-o").arg(p); } }
        let status = cmd.stderr(std::process::Stdio::inherit()).spawn();
        let _ = close(cli_fd);
        if let Some(up_fd)=pass_fd { let _ = close(up_fd); }
        let _child = status?; // keep child process alive
        // Duplicate for separate read/write handles (blocking)
        let read_dup = nix::unistd::dup(parent_fd)?;
        // Mark CLOEXEC
        let _ = fcntl(parent_fd, FcntlArg::F_SETFD(FdFlag::FD_CLOEXEC));
        let _ = fcntl(read_dup, FcntlArg::F_SETFD(FdFlag::FD_CLOEXEC));
        Ok((parent_fd, read_dup))
    }

    let mut children: Vec<MuxChild> = child_parent_fds.into_iter().map(|fd| MuxChild { fd, alive:true, buf:Vec::new() }).collect();
    let vfsd_path = env::var("LLMSH_VFSD_BIN").unwrap_or_else(|_| "vfsd/target/debug/vfsd".to_string());
    let (up_write_fd, up_read_fd) = spawn_upstream(&vfsd_path, allow_read, allow_write, pass_fd)?;
    let mux_debug = env::var("LLMSH_DEBUG_MUX").is_ok();
    let mut upstream_buf: Vec<u8> = Vec::new();
    let mut pending: HashMap<String, Pending> = HashMap::new();
    let mut req_seq: u64 = 0;
    if mux_debug { eprintln!("[mux] start: children={} fds={:?} upstream=(w:{},r:{})", children.len(), children.iter().map(|c| c.fd).collect::<Vec<_>>(), up_write_fd, up_read_fd); }
    while children.iter().any(|c| c.alive) {
        // Build poll set: child fds + upstream read fd
        let mut pfds: Vec<libc::pollfd> = children.iter().filter(|c| c.alive)
            .map(|c| libc::pollfd { fd: c.fd, events: (libc::POLLIN | libc::POLLHUP | libc::POLLERR) as i16, revents: 0 }).collect();
        // Upstream read
        pfds.push(libc::pollfd { fd: up_read_fd, events: (libc::POLLIN | libc::POLLHUP | libc::POLLERR) as i16, revents: 0 });
        let rc = unsafe { libc::poll(pfds.as_mut_ptr(), pfds.len() as libc::nfds_t, 250) }; // 250ms tick
        if rc < 0 { eprintln!("mux poll error: {}", std::io::Error::last_os_error()); break; }
        if rc == 0 { continue; }
        // Process events
        for pfd in &pfds {
            if pfd.revents == 0 { continue; }
            // Upstream readable / error
            if pfd.fd == up_read_fd {
                if (pfd.revents & (libc::POLLHUP | libc::POLLERR)) != 0 { eprintln!("mux upstream hangup/error"); break; }
                if (pfd.revents & libc::POLLIN) != 0 {
                    loop {
                        let mut tmp=[0u8;4096]; match nread(up_read_fd,&mut tmp) {
                            Ok(0)=>{ // upstream closed
                                if mux_debug { eprintln!("[mux] upstream EOF"); }
                                break;
                            },
                            Ok(n)=>{ upstream_buf.extend_from_slice(&tmp[..n]); if n < tmp.len() { break; } },
                            Err(e)=>{ if e==nix::errno::Errno::EAGAIN { break; } eprintln!("mux upstream read error: {e}"); break; }
                        }
                    }
                    // Parse upstream frames
                    loop {
                        if upstream_buf.len()<4 { break; }
                        let len = u32::from_be_bytes([upstream_buf[0],upstream_buf[1],upstream_buf[2],upstream_buf[3]]) as usize;
                        if upstream_buf.len() < 4+len { break; }
                        let frame = upstream_buf[4..4+len].to_vec(); upstream_buf.drain(0..4+len);
                        let v: serde_json::Value = match serde_json::from_slice(&frame) { Ok(v)=>v, Err(_)=> { eprintln!("mux: bad json from upstream"); continue; } };
                        let mux_id = v.get("id").and_then(|i| i.as_str()).unwrap_or("").to_string();
                        if let Some(p) = pending.remove(&mux_id) {
                            let mut resp_obj = v.clone();
                            // restore child id
                            if let serde_json::Value::String(_) = p.child_id { resp_obj["id"] = p.child_id; } else { resp_obj["id"] = p.child_id; }
                            let resp_bytes = serde_json::to_vec(&resp_obj).unwrap_or_else(|_| b"{}".to_vec());
                            let len_bytes = (resp_bytes.len() as u32).to_be_bytes();
                            if let Err(e)=write_all_fd(p.child_fd,&len_bytes) { eprintln!("mux child write len error: {e}"); }
                            else if let Err(e)=write_all_fd(p.child_fd,&resp_bytes) { eprintln!("mux child write frame error: {e}"); }
                            else if mux_debug { eprintln!("[mux] upstream->child delivered mux_id={} child_fd={}", mux_id, p.child_fd); }
                        } else {
                            eprintln!("mux: unknown mux_id response '{}', dropping", mux_id);
                        }
                    }
                }
                continue;
            }
            // Child fd events
            if let Some(child) = children.iter_mut().find(|c| c.fd == pfd.fd && c.alive) {
                if mux_debug { eprintln!("[mux] event fd={} revents=0x{:x}", pfd.fd, pfd.revents); }
                if (pfd.revents & (libc::POLLHUP | libc::POLLERR)) != 0 { let mut tmp=[0u8;1024]; let _=nread(child.fd,&mut tmp); child.alive=false; continue; }
                if (pfd.revents & libc::POLLIN) != 0 {
                    loop { let mut tmp=[0u8;4096]; match nread(child.fd,&mut tmp) { Ok(0)=>{ child.alive=false; break; }, Ok(n)=>{ child.buf.extend_from_slice(&tmp[..n]); if n < tmp.len() { break; } }, Err(e)=>{ if e==nix::errno::Errno::EAGAIN { break; } child.alive=false; break; } } }
                }
                // Extract complete frames from child
                loop {
                    if child.buf.len()<4 { break; }
                    let len = u32::from_be_bytes([child.buf[0],child.buf[1],child.buf[2],child.buf[3]]) as usize;
                    if child.buf.len() < 4+len { break; }
                    let frame = child.buf[4..4+len].to_vec(); child.buf.drain(0..4+len);
                    let mut v: serde_json::Value = match serde_json::from_slice(&frame) { Ok(v)=>v, Err(_)=> { eprintln!("mux: bad json from child"); continue; } };
                    let child_id = v.get("id").cloned().unwrap_or(json!("?"));
                    // MUX-level reserved handle interception (defensive):
                    if let Some(op) = v.get("op").and_then(|o| o.as_str()) {
                        match op {
                            "read" => {
                                let h = v.get("params").and_then(|p| p.get("h")).and_then(|h| h.as_u64()).unwrap_or(u64::MAX) as u32;
                                if h == 0 {
                                    let max = v.get("params").and_then(|p| p.get("max")).and_then(|m| m.as_u64()).unwrap_or(4096) as usize;
                                    let max = std::cmp::min(max, 4096usize);
                                    let mut buf = vec![0u8; max];
                                    let n = match std::io::stdin().read(&mut buf) { Ok(n)=>n, Err(_)=>{ buf.clear(); 0 } };
                                    buf.truncate(n);
                                    let b64 = if buf.is_empty() { String::new() } else { general_purpose::STANDARD.encode(&buf) };
                                    let resp_obj = json!({
                                        "id": child_id,
                                        "ok": true,
                                        "result": { "eof": n==0, "data": b64, "reserved": true }
                                    });
                                    let resp_bytes = serde_json::to_vec(&resp_obj).unwrap_or_else(|_| b"{}".to_vec());
                                    let len_bytes = (resp_bytes.len() as u32).to_be_bytes();
                                    let _ = write_all_fd(child.fd, &len_bytes);
                                    let _ = write_all_fd(child.fd, &resp_bytes);
                                    if mux_debug { eprintln!("[mux] served read on fd0 locally to child_fd={}", child.fd); }
                                    continue; // do not forward upstream
                                }
                            }
                            "write" => {
                                let h = v.get("params").and_then(|p| p.get("h")).and_then(|h| h.as_u64()).unwrap_or(u64::MAX) as u32;
                                if h == 1 || h == 2 {
                                    let data_b64 = v.get("params").and_then(|p| p.get("data")).and_then(|s| s.as_str()).unwrap_or("");
                                    let decoded = general_purpose::STANDARD.decode(data_b64).unwrap_or_default();
                                    let w = if h==1 { std::io::stdout().write(&decoded).unwrap_or(0) } else { std::io::stderr().write(&decoded).unwrap_or(0) };
                                    let resp_obj = json!({ "id": child_id, "ok": true, "result": { "written": w as u64, "reserved": true } });
                                    let resp_bytes = serde_json::to_vec(&resp_obj).unwrap_or_else(|_| b"{}".to_vec());
                                    let len_bytes = (resp_bytes.len() as u32).to_be_bytes();
                                    let _ = write_all_fd(child.fd, &len_bytes);
                                    let _ = write_all_fd(child.fd, &resp_bytes);
                                    if mux_debug { eprintln!("[mux] served write on fd{} locally to child_fd={}", h, child.fd); }
                                    continue;
                                }
                                if h == 0 {
                                    // writing to fd0 is not allowed; return E_PERM
                                    let resp_obj = json!({ "id": child_id, "ok": false, "error": { "code": "E_PERM", "message": "not writable" } });
                                    let resp_bytes = serde_json::to_vec(&resp_obj).unwrap_or_else(|_| b"{}".to_vec());
                                    let len_bytes = (resp_bytes.len() as u32).to_be_bytes();
                                    let _ = write_all_fd(child.fd, &len_bytes);
                                    let _ = write_all_fd(child.fd, &resp_bytes);
                                    if mux_debug { eprintln!("[mux] denied write on fd0 locally to child_fd={}", child.fd); }
                                    continue;
                                }
                            }
                            "close" => {
                                let h = v.get("params").and_then(|p| p.get("h")).and_then(|h| h.as_u64()).unwrap_or(u64::MAX) as u32;
                                if h <= 2 {
                                    let resp_obj = json!({ "id": child_id, "ok": true, "result": { "closed": true, "reserved": true } });
                                    let resp_bytes = serde_json::to_vec(&resp_obj).unwrap_or_else(|_| b"{}".to_vec());
                                    let len_bytes = (resp_bytes.len() as u32).to_be_bytes();
                                    let _ = write_all_fd(child.fd, &len_bytes);
                                    let _ = write_all_fd(child.fd, &resp_bytes);
                                    if mux_debug { eprintln!("[mux] acknowledged close on reserved fd locally to child_fd={}", child.fd); }
                                    continue;
                                }
                            }
                            _ => {}
                        }
                    }
                    req_seq += 1; let mux_id = req_seq.to_string();
                    v["id"] = json!(mux_id.clone()); // replace id for upstream
                    // Record pending mapping
                    pending.insert(mux_id.clone(), Pending { child_fd: child.fd, child_id });
                    let out_bytes = serde_json::to_vec(&v).unwrap_or_else(|_| b"{}".to_vec());
                    let len_bytes = (out_bytes.len() as u32).to_be_bytes();
                    if let Err(e)=write_all_fd(up_write_fd,&len_bytes) { eprintln!("mux upstream write len error: {e}"); break; }
                    if let Err(e)=write_all_fd(up_write_fd,&out_bytes) { eprintln!("mux upstream write frame error: {e}"); break; }
                    if mux_debug { eprintln!("[mux] child_fd={} -> upstream mux_id={} (pending={})", child.fd, req_seq, pending.len()); }
                }
            }
        }
        if mux_debug { let alive_cnt = children.iter().filter(|c| c.alive).count(); eprintln!("[mux] loop end alive={} pending={}", alive_cnt, pending.len()); }
    }
    // Cleanup
    for c in children { let _ = close(c.fd); }
    let _ = close(up_write_fd); let _ = close(up_read_fd);
    if mux_debug { eprintln!("[mux] exit loop pending_left={}", pending.len()); }
    Ok(())
}

// --- Low-level helpers ---
fn write_all_fd(fd: RawFd, mut buf: &[u8]) -> std::io::Result<()> {
    while !buf.is_empty() {
        let rc = unsafe { libc::write(fd, buf.as_ptr() as *const _, buf.len()) };
        if rc < 0 {
            let err = std::io::Error::last_os_error();
            if err.kind() == std::io::ErrorKind::Interrupted { continue; }
            // Non-blocking/EAGAIN should not occur (fds are blocking for child ends); treat as error.
            return Err(err);
        }
        let written = rc as usize;
        if written == 0 { return Err(std::io::Error::new(std::io::ErrorKind::WriteZero, "write returned 0")); }
        buf = &buf[written..];
    }
    Ok(())
}
