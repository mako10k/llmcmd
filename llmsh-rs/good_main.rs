use anyhow::Result;
use std::env;
use std::path::{Path, PathBuf};
use std::os::unix::fs::PermissionsExt; // for mode()
use std::os::unix::io::RawFd;
use std::os::fd::IntoRawFd; // needed for into_raw_fd on OwnedFd
use std::ffi::CString;
use std::io::Write;
use std::process::Command;
use nix::unistd::{fork, ForkResult, pipe, dup2, close, execvp};
use nix::fcntl::{fcntl, FcntlArg, FdFlag, OFlag};
use nix::sys::socket::{socketpair, AddressFamily, SockType, SockFlag};
use nix::libc;
// extracted multiplexer & VFS client implementation moved to mux.rs
mod mux;
use mux::{child_mux, init_child_mux, VfsErrorCode};
use regex::Regex;

// -------- Builtins / Parsing ---------
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum BuiltinKind { Cat, Head, Tail, Wc, Grep, Tr, Sed, Help, Llmsh }

#[derive(Debug, Default, Clone)]
struct RedirSpec { in_file: Option<String>, out_file: Option<(String,bool)> } // (path, append)

#[derive(Debug, Clone)]
enum ExecUnit { Builtin { kind: BuiltinKind, args: Vec<String>, redir: RedirSpec }, External { argv: Vec<String>, redir: RedirSpec } }

fn parse_segment(seg: &str) -> Option<ExecUnit> {
    let toks: Vec<String> = seg.split_whitespace().map(|s| s.to_string()).collect();
    if toks.is_empty() { return None; }
    let mut redir = RedirSpec::default();
    let mut out: Vec<String> = Vec::new();
    let mut i=0;
    while i < toks.len() {
        let t=&toks[i];
        let need_next = |idx:usize| idx+1 < toks.len();
        match t.as_str() {
            "<" if need_next(i) => { if redir.in_file.is_none() { redir.in_file = Some(toks[i+1].clone()); } i+=2; },
            ">" if need_next(i) => { redir.out_file = Some((toks[i+1].clone(), false)); i+=2; },
            ">>" if need_next(i) => { redir.out_file = Some((toks[i+1].clone(), true)); i+=2; },
            tok => { out.push(tok.to_string()); i+=1; }
        }
    }
    if out.is_empty() { return None; }
    let cmd = &out[0];
    let args = out[1..].to_vec();
    if ["cat","head","tail","wc","grep","tr","sed","help","llmsh"].contains(&cmd.as_str()) {
        if !args.is_empty() && cmd == "cat" { redir.in_file = None; }
        let kind = match cmd.as_str() { "cat"=>BuiltinKind::Cat, "head"=>BuiltinKind::Head, "tail"=>BuiltinKind::Tail, "wc"=>BuiltinKind::Wc, "grep"=>BuiltinKind::Grep, "tr"=>BuiltinKind::Tr, "sed"=>BuiltinKind::Sed, "help"=>BuiltinKind::Help, "llmsh"=>BuiltinKind::Llmsh, _=>BuiltinKind::Cat };
        return Some(ExecUnit::Builtin { kind, args, redir });
    }
    Some(ExecUnit::External { argv: out, redir })
}

fn parse_pipeline(input:&str)->Vec<ExecUnit>{ input.split('|').filter_map(|s| parse_segment(s.trim())).collect() }

fn spawn_pipeline(units:&[ExecUnit], allow_read:&[String], allow_write:&[String], pass_fds:Option<(i32,i32)>) -> Result<i32> {
    if units.is_empty() { return Ok(0); }
    // Helper: set FD_CLOEXEC; optionally set O_NONBLOCK (used for MUX sockets so poll loop is safe)
    fn set_cloexec(fd: RawFd) { let _ = fcntl(fd, FcntlArg::F_SETFD(FdFlag::FD_CLOEXEC)); }
    fn set_nonblock(fd: RawFd) { if let Ok(flags) = fcntl(fd, FcntlArg::F_GETFL) { let mut oflags = OFlag::from_bits_truncate(flags); oflags.insert(OFlag::O_NONBLOCK); let _ = fcntl(fd, FcntlArg::F_SETFL(oflags)); } }
    fn clear_nonblock(fd: RawFd) { if let Ok(flags) = fcntl(fd, FcntlArg::F_GETFL) { let mut oflags = OFlag::from_bits_truncate(flags); oflags.remove(OFlag::O_NONBLOCK); let _ = fcntl(fd, FcntlArg::F_SETFL(oflags)); } }
    fn clear_cloexec(fd: RawFd) { if let Ok(flags) = fcntl(fd, FcntlArg::F_GETFD) { let mut f = FdFlag::from_bits_truncate(flags); f.remove(FdFlag::FD_CLOEXEC); let _ = fcntl(fd, FcntlArg::F_SETFD(f)); } }
    let mut fds: Vec<(RawFd, RawFd)> = Vec::new();
    for _ in 0..units.len().saturating_sub(1) {
        let (r,w)=pipe()?;
        let rfd = r.into_raw_fd();
        let wfd = w.into_raw_fd();
        // Mark pipeline intermediate FDs CLOEXEC to avoid leaking into exec children (dup2 will create stdio without CLOEXEC later)
        set_cloexec(rfd); set_cloexec(wfd);
        fds.push((rfd,wfd));
    }
    // New design: always operate in MUX mode with a single lazily spawned upstream vfsd.
    let mux_mode = true;
    let mut mux_child_fds: Vec<Option<(RawFd, RawFd)>> = Vec::new(); // (child_end, parent_end)
    if mux_mode {
        for _ in 0..units.len() {
            let (s1,s2) = socketpair(AddressFamily::Unix, SockType::Stream, None, SockFlag::SOCK_CLOEXEC)?; // CLOEXEC
            let s1_fd = s1.into_raw_fd();
            let s2_fd = s2.into_raw_fd();
            // Set non-blocking for poll-driven MUX loop
            set_nonblock(s1_fd); set_nonblock(s2_fd);
            // (CLOEXEC already via flag) – still explicitly mark for safety in case of older kernels
            set_cloexec(s1_fd); set_cloexec(s2_fd);
            mux_child_fds.push(Some((s1_fd,s2_fd)));
        }
    }
    // Parent MUX storage
    // legacy variable removed (parent_mux_fds) after switching to libc::poll implementation
    for (i, unit) in units.iter().enumerate() {
        match unsafe { fork()? } {
            ForkResult::Child => {
                if i > 0 { let (pr,_pw)=fds[i-1]; dup2(pr,0)?; }
                if i < units.len()-1 { let (_pr,pw)=fds[i]; dup2(pw,1)?; }
                // Ensure stdio not CLOEXEC for exec'd external commands
                clear_cloexec(0); clear_cloexec(1); clear_cloexec(2);
                for (r,w) in &fds { let _=close(*r); let _=close(*w); }
                if mux_mode { if let Some((child_fd,parent_fd)) = mux_child_fds[i] { let _=close(parent_fd); clear_nonblock(child_fd); init_child_mux(child_fd); } }
                match unit {
                    ExecUnit::Builtin { kind, args, redir } => {
                        let code = match kind {
                BuiltinKind::Cat => run_builtin_cat(args, redir, allow_read, allow_write, pass_fds),
                BuiltinKind::Head => run_builtin_head(args, redir, allow_read, allow_write, pass_fds),
                BuiltinKind::Tail => run_builtin_tail(args, redir, allow_read, allow_write, pass_fds),
                BuiltinKind::Wc => run_builtin_wc(args, redir, allow_read, allow_write, pass_fds),
                BuiltinKind::Grep => run_builtin_grep(args, redir, allow_read, allow_write, pass_fds),
                BuiltinKind::Tr => run_builtin_tr(args, redir, allow_read, allow_write, pass_fds),
                BuiltinKind::Sed => run_builtin_sed(args, redir, allow_read, allow_write, pass_fds),
                            BuiltinKind::Help => run_builtin_help(args),
                BuiltinKind::Llmsh => run_builtin_llmsh(args, redir, allow_read, allow_write, pass_fds),
                        }; 
                        std::process::exit(code); 
                    }
                    ExecUnit::External { argv, redir } => {
                        // Apply simple allowlist-based redirections (local FS) BEFORE exec.
                        // Security: only permit paths explicitly listed in allow_read/allow_write.
                        // NOTE: Future enhancement could route through VFS daemon for stricter control.
                        let check_allowed = |p: &str, allow: &[String]| -> bool { allow.iter().any(|a| a == p) };
                        if let Some(in_path) = &redir.in_file {
                            if !check_allowed(in_path, allow_read) {
                                eprintln!("redir input not allowed: {in_path}");
                                std::process::exit(13);
                            }
                            match std::fs::File::open(in_path) { Ok(f)=> { let fd = f.into_raw_fd(); dup2(fd, 0)?; }, Err(e)=> { eprintln!("failed to open input {in_path}: {e}"); std::process::exit(14); } }
                        }
                        if let Some((out_path, append)) = &redir.out_file {
                            if !check_allowed(out_path, allow_write) {
                                eprintln!("redir output not allowed: {out_path}");
                                std::process::exit(15);
                            }
                            let mut opts = std::fs::OpenOptions::new();
                            opts.create(true).write(true);
                            if *append { opts.append(true); } else { opts.truncate(true); }
                            match opts.open(out_path) { Ok(f)=> { let fd = f.into_raw_fd(); dup2(fd, 1)?; }, Err(e)=> { eprintln!("failed to open output {out_path}: {e}"); std::process::exit(16); } }
                        }
                        let cstrs: Vec<CString> = argv.iter().map(|s| CString::new(s.as_str()).unwrap()).collect();
                        if cstrs.is_empty() { std::process::exit(127); }
                        let argv_refs: Vec<&CString> = cstrs.iter().collect();
                        let prog = argv_refs[0];
                        execvp(prog, &argv_refs)?; unreachable!();
                    }
                }
            }
            ForkResult::Parent { .. } => {}
        }
    }
    for (r,w) in &fds { let _=close(*r); let _=close(*w); }
    if mux_mode {
        let mut parent_fds: Vec<RawFd> = Vec::new();
        for pair in mux_child_fds.into_iter() { if let Some((child_fd,parent_fd))=pair { let _=close(child_fd); parent_fds.push(parent_fd); } }
        mux::run_mux(parent_fds, allow_read, allow_write, pass_fds)?;
    }
    let mut status_code = 0;
    for _ in 0..units.len() { let mut status: i32 = 0; unsafe { libc::wait(&mut status); if libc::WIFEXITED(status){ status_code = libc::WEXITSTATUS(status);} } }
    Ok(status_code)
}

fn main() -> Result<()> {
    let mut args = env::args().skip(1);
    let mut script: Option<String> = None;
    let mut allow_read: Vec<String> = Vec::new();
    let mut allow_write: Vec<String> = Vec::new();
    let mut pass_fds: Option<(i32,i32)> = None;
    let mut vfsd_arg: Option<String> = None; // --vfsd explicit binary path
    while let Some(a) = args.next() {
        match a.as_str() {
            "-c" => { script = Some(args.next().ok_or_else(|| anyhow::anyhow!("missing script after -c"))?); }
            "-i" | "--input" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("missing value after -i"))?; allow_read.push(v); }
            "-o" | "--output" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("missing value after -o"))?; allow_write.push(v); }
            "--vfs-fds" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("missing value after --vfs-fds"))?; let parts: Vec<&str> = v.split(',').collect(); if parts.len()!=2 { anyhow::bail!("--vfs-fds requires '<rfd>,<wfd>'"); } let rfd: i32 = parts[0].parse().map_err(|_| anyhow::anyhow!("invalid rfd"))?; let wfd: i32 = parts[1].parse().map_err(|_| anyhow::anyhow!("invalid wfd"))?; pass_fds = Some((rfd,wfd)); }
            "--vfsd" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("missing value after --vfsd"))?; vfsd_arg = Some(v); }
            other => { eprintln!("unknown arg: {other}"); }
        }
    }
    if pass_fds.is_some() && (!allow_read.is_empty() || !allow_write.is_empty()) {
        eprintln!("error: --vfs-fds cannot be combined with -i/-o options");
        std::process::exit(2);
    }

    // Resolve vfsd binary path with precedence:
    // (1) --vfsd argument
    // (2) Environment variable LLMSH_VFSD_BIN
    // (3) Same directory as current executable (llmsh) containing a file named "vfsd"
    // (4) PATH search
    // (5) Error out
    fn is_executable(p: &Path) -> bool { p.is_file() && std::fs::metadata(p).map(|m| m.permissions().mode() & 0o111 != 0).unwrap_or(false) }
    fn resolve_vfsd(arg: &Option<String>) -> Result<String> {
        if let Some(a) = arg { let p = PathBuf::from(a); if is_executable(&p) { return Ok(p.to_string_lossy().to_string()); } else { anyhow::bail!("--vfsd path not executable or not found: {a}"); } }
        if let Ok(envv) = env::var("LLMSH_VFSD_BIN") { let p = PathBuf::from(&envv); if is_executable(&p) { return Ok(p.to_string_lossy().to_string()); } }
        if let Ok(exe) = env::current_exe() { if let Some(dir) = exe.parent() { let cand = dir.join("vfsd"); if is_executable(&cand) { return Ok(cand.to_string_lossy().to_string()); } } }
        if let Some(path_var) = env::var_os("PATH") { for comp in env::split_paths(&path_var) { let cand = comp.join("vfsd"); if is_executable(&cand) { return Ok(cand.to_string_lossy().to_string()); } } }
        anyhow::bail!("vfsd binary not found: specify --vfsd or set LLMSH_VFSD_BIN or place 'vfsd' alongside llmsh or in PATH")
    }
    let resolved_vfsd = match resolve_vfsd(&vfsd_arg) { Ok(p)=>p, Err(e)=> { eprintln!("vfsd resolve error: {e}"); std::process::exit(9); } };
    // Export resolved path so existing spawn code (which still reads env var) uses it.
    env::set_var("LLMSH_VFSD_BIN", &resolved_vfsd);
    let s = script.unwrap_or_else(|| "".to_string());
    let units = parse_pipeline(&s);
    let code = spawn_pipeline(&units, &allow_read, &allow_write, pass_fds)?;
    std::process::exit(code);
}

fn run_builtin_cat(args: &Vec<String>, redir: &RedirSpec, _allow_read:&[String], _allow_write:&[String], _pass_fds:Option<(i32,i32)>) -> i32 {
    let inputs: Vec<String> = if !args.is_empty() { args.clone() } else if let Some(f)= &redir.in_file { vec![f.clone()] } else { Vec::new() };
    if inputs.is_empty() { return 0; }
    let client = child_mux();
    let mut out_handle: Option<u32> = None;
    if let Some((path, append)) = &redir.out_file { let mode = if *append { "a" } else { "w" }; match client.open(path, mode) { Ok(Ok(h))=> out_handle=Some(h), Ok(Err(code))=> { eprintln!("open {path} {mode} error: {:?}", code); return map_exit(&code); }, Err(e)=> { eprintln!("protocol open {path} {mode} error: {e}"); return 6; } } }
    let mut stdout_handle = std::io::stdout();
    for p in inputs.iter() {
    let h = match client.open(p, "r") { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open {p} r error: {:?}", code); return map_exit(&code); }, Err(e)=> { eprintln!("protocol open {p} r error: {e}"); return 6; } };
        loop {
            match client.read_chunk(h, 4096) {
                Ok(Ok((data, eof))) => {
                    if data.is_empty() && eof { break; }
                    if let Some(oh)=out_handle {
                        match client.write_chunk(oh, &data) { Ok(Ok(_))=>{}, Ok(Err(code))=>{ eprintln!("write error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol write error: {e}"); return 6; } }
                    } else if let Err(e)= stdout_handle.write_all(&data) { eprintln!("stdout write error: {e}"); return 7; }
                    if eof { break; }
                }
                Ok(Err(code)) => { eprintln!("read error: {:?}", code); return map_exit(&code); }
                Err(e) => { eprintln!("protocol read error: {e}"); return 6; }
            }
        }
        let _ = client.close(h);
    }
    if let Some(oh)=out_handle { let _ = client.close(oh); }
    if out_handle.is_none() { let _ = stdout_handle.flush(); }
    0
}

// ---- head / tail helpers ----
fn parse_head_tail_args(_kind: BuiltinKind, args:&[String]) -> (usize, Vec<String>) {
    let mut n:usize = 10; // default lines
    let mut files:Vec<String>=Vec::new();
    let mut i=0;
    while i < args.len() {
        if args[i] == "-n" && i+1 < args.len() { if let Ok(v)=args[i+1].parse::<usize>() { n=v; } i+=2; continue; }
        files.push(args[i].clone()); i+=1;
    }
    (n, files)
}

fn run_builtin_head(args:&Vec<String>, redir:&RedirSpec, _allow_read:&[String], _allow_write:&[String], _pass_fds:Option<(i32,i32)>) -> i32 {
    let (limit, mut files) = parse_head_tail_args(BuiltinKind::Head, args);
    if files.is_empty() {
        if let Some(f)=&redir.in_file { files.push(f.clone()); } else { return 0; }
    }
    let client = child_mux();
    let mut stdout_handle = std::io::stdout();
    let mut remaining = limit;
    for p in files.iter() {
        if remaining == 0 { break; }
    let h = match client.open(p, "r") { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open {p} r error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open {p} r error: {e}"); return 6; } };
        let mut buf_acc:Vec<u8>=Vec::new();
        loop {
            match client.read_chunk(h, 4096) {
                Ok(Ok((data,eof))) => {
                    if data.is_empty() && eof { break; }
                    buf_acc.extend_from_slice(&data);
                    // process lines
                    let mut start=0; let mut i=0;
                    while i < buf_acc.len() && remaining>0 {
                        if buf_acc[i]==b'\n' { let line=&buf_acc[start..=i]; if let Err(e)=stdout_handle.write_all(line) { eprintln!("stdout err: {e}"); return 7; } remaining-=1; start=i+1; }
                        i+=1;
                    }
                    if start>0 { buf_acc.drain(0..start); }
                    if remaining==0 || eof { break; }
                }
                Ok(Err(code)) => { eprintln!("read error: {:?}", code); return map_exit(&code); }
                Err(e)=> { eprintln!("protocol read error: {e}"); return 6; }
            }
        }
        let _ = client.close(h);
    }
    let _ = stdout_handle.flush();
    0
}

fn run_builtin_tail(args:&Vec<String>, redir:&RedirSpec, _allow_read:&[String], _allow_write:&[String], _pass_fds:Option<(i32,i32)>) -> i32 {
    let (limit, mut files) = parse_head_tail_args(BuiltinKind::Tail, args);
    if files.is_empty() { if let Some(f)=&redir.in_file { files.push(f.clone()); } else { return 0; } }
    let client = child_mux();
    let mut stdout_handle = std::io::stdout();
    for (fi,p) in files.iter().enumerate() {
        let h = match client.open(p, "r") { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open {p} r error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open {p} r error: {e}"); return 6; } };
        // naive: read whole file (WARNING large file). Could optimize later.
        let mut all:Vec<u8>=Vec::new();
        loop { match client.read_chunk(h, 8192) { Ok(Ok((data,eof)))=>{ if !data.is_empty(){ all.extend_from_slice(&data); } if eof { break; } }, Ok(Err(code))=>{ eprintln!("read error {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol read error: {e}"); return 6; } } }
        let _ = client.close(h);
        // collect newline indices to preserve exact newline presence
        let mut line_starts:Vec<usize>=Vec::new();
        let mut newline_pos:Vec<usize>=Vec::new();
        if !all.is_empty() { line_starts.push(0); }
        for (i,b) in all.iter().enumerate() {
            if *b==b'\n' { newline_pos.push(i); if i+1 < all.len() { line_starts.push(i+1); } }
        }
        let total_lines = line_starts.len();
        let start_line = if total_lines>limit { total_lines - limit } else { 0 };
        if fi>0 && start_line < total_lines { /* placeholder for multi-file separator */ }
        for li in start_line..total_lines {
            let start = line_starts[li];
            // did this line terminate with a newline? if li < newline_pos.len() AND newline_pos[li] >= start
            let had_nl = li < newline_pos.len();
            let end_exclusive = if had_nl { newline_pos[li] + 1 } else { all.len() }; // include newline if present
            if start > all.len() || end_exclusive > all.len() || start > end_exclusive { break; }
            if let Err(e)=stdout_handle.write_all(&all[start..end_exclusive]) { eprintln!("stdout err: {e}"); return 7; }
            if !had_nl { let _=stdout_handle.write_all(b"\n"); }
        }
    }
    let _ = stdout_handle.flush();
    0
}

fn map_exit(code:&VfsErrorCode) -> i32 { match code { VfsErrorCode::ENoEnt=>1, VfsErrorCode::EPerm|VfsErrorCode::EArg=>2, VfsErrorCode::EIO=>3, VfsErrorCode::EClosed=>4, VfsErrorCode::EUnsupported=>5, VfsErrorCode::Other(_)=>6 } }
// ---- wc ----
fn run_builtin_wc(args:&Vec<String>, redir:&RedirSpec, _allow_read:&[String], _allow_write:&[String], _pass_fds:Option<(i32,i32)>) -> i32 {
    // Options (subset): support -l (lines), -w (words), -c (bytes), default all three like POSIX.
    let mut show_l=true; let mut show_w=true; let mut show_b=true;
    let mut files:Vec<String>=Vec::new();
    for a in args.iter() {
        if a.starts_with('-') && a.len()>1 { for ch in a.chars().skip(1) { match ch { 'l'=>{show_w=false;show_b=false;}, 'w'=>{show_l=false;show_b=false;}, 'c'=>{show_l=false;show_w=false;}, _=>{} } } } else { files.push(a.clone()); }
    }
    if files.is_empty() { if let Some(f)=&redir.in_file { files.push(f.clone()); } }
    if files.is_empty() { return 0; }
    let client = child_mux();
    let mut stdout_handle = std::io::stdout();
    let mut total_l=0usize; let mut total_w=0usize; let mut total_b=0usize;
    let multi = files.len()>1;
    for p in files.iter() {
        let h = match client.open(p, "r") { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open {p} r error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open {p} r error: {e}"); return 6; } };
        let mut l=0usize; let mut w=0usize; let mut b=0usize; let mut in_word=false;
        loop { match client.read_chunk(h, 8192) { Ok(Ok((data,eof)))=>{ if data.is_empty() && eof { break; } b+=data.len(); for &c in &data { if c==b'\n' { l+=1; } if c.is_ascii_whitespace() { if in_word { in_word=false; } } else { if !in_word { w+=1; in_word=true; } } } if eof { break; } }, Ok(Err(code))=>{ eprintln!("read error {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol read error: {e}"); return 6; } } }
        let _ = client.close(h);
        total_l+=l; total_w+=w; total_b+=b;
        let _ = write!(stdout_handle, "{:>8}{:>8}{:>8} {}\n", if show_l { l } else {0}, if show_w { w } else {0}, if show_b { b } else {0}, p);
    }
    if multi { let _ = write!(stdout_handle, "{:>8}{:>8}{:>8} total\n", if show_l { total_l } else {0}, if show_w { total_w } else {0}, if show_b { total_b } else {0}); }
    let _ = stdout_handle.flush();
    0
}

// ---- grep (expanded flags: -i -n -E|-r -v -c -x -w -H/-h -q) ----
fn run_builtin_grep(args:&Vec<String>, redir:&RedirSpec, _allow_read:&[String], _allow_write:&[String], _pass_fds:Option<(i32,i32)>) -> i32 {
    if args.is_empty() { eprintln!("grep: missing pattern"); return 2; }
    // Options
    let mut idx=0;
    let mut ignore_case=false; // -i
    let mut show_line=false;   // -n
    let mut use_regex=false;   // -E / -r
    let mut invert=false;      // -v
    let mut count_only=false;  // -c
    let mut whole_line=false;  // -x
    let mut word_match=false;  // -w
    let mut force_with_filename:Option<bool>=None; // -H(true)/-h(false)
    let mut quiet=false;       // -q
    while idx < args.len() && args[idx].starts_with('-') && args[idx] != "-" {
        for ch in args[idx].chars().skip(1) {
            match ch {
                'i' => ignore_case=true,
                'n' => show_line=true,
                'E' | 'r' => use_regex=true,
                'v' => invert=true,
                'c' => count_only=true,
                'x' => whole_line=true,
                'w' => word_match=true,
                'H' => force_with_filename=Some(true),
                'h' => force_with_filename=Some(false),
                'q' => quiet=true,
                _ => {}
            }
        }
        idx+=1;
    }
    if idx>=args.len() { eprintln!("grep: missing pattern"); return 2; }
    let mut pattern_raw=args[idx].clone(); idx+=1;
    let mut files:Vec<String> = if idx < args.len() { args[idx..].to_vec() } else { Vec::new() };
    if files.is_empty() { if let Some(f)=&redir.in_file { files.push(f.clone()); } }
    if files.is_empty() { return 0; }

    // Build matcher
    let mut regex_opt:Option<Regex>=None;
    let mut needle=String::new();
    if use_regex {
        // Apply -w and -x by anchoring/wrapping with word boundaries
        let mut pat = pattern_raw.clone();
        if word_match { pat = format!("\\b{}\\b", pat); }
        if whole_line { pat = format!("^{}$", pat); }
        if ignore_case { pat = format!("(?i){}", pat); }
        match Regex::new(&pat) { Ok(r)=>regex_opt=Some(r), Err(e)=>{ eprintln!("grep: invalid regex: {e}"); return 2; } }
    } else {
        // Literal
        if word_match {
            // For literal -w, construct regex with escaped literal
            let esc = regex::escape(&pattern_raw);
            let mut pat = format!("\\b{}\\b", esc);
            if whole_line { pat = format!("^{}$", pat); }
            if ignore_case { pat = format!("(?i){}", pat); }
            match Regex::new(&pat) { Ok(r)=>{ regex_opt=Some(r); use_regex=true; }, Err(e)=>{ eprintln!("grep: internal regex build failed: {e}"); return 2; } }
        } else if whole_line {
            // For literal -x, compare equality later; no regex needed
            needle = if ignore_case { pattern_raw.to_lowercase() } else { pattern_raw.clone() };
        } else {
            needle = if ignore_case { pattern_raw.to_lowercase() } else { pattern_raw.clone() };
        }
    }

    let client = child_mux();
    let mut stdout_handle = std::io::stdout();
    let multi_default = files.len()>1;
    let with_filename = force_with_filename.unwrap_or(multi_default);
    let mut any_selected=false;
    for p in files.iter() {
        let h = match client.open(p, "r") { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open {p} r error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open {p} r error: {e}"); return 6; } };
        let mut buf_acc:Vec<u8>=Vec::new();
        let mut line_no:usize=0;
        let mut cnt:usize=0;
        let mut early_exit=false;
        loop {
            match client.read_chunk(h, 4096) {
                Ok(Ok((data,eof))) => {
                    if data.is_empty() && eof { break; }
                    buf_acc.extend_from_slice(&data);
                    let mut start=0; let mut i=0;
                    while i < buf_acc.len() {
                        if buf_acc[i]==b'\n' {
                            line_no+=1; let line_slice=&buf_acc[start..i];
                            let mut selected=false;
                            if let Ok(s)=std::str::from_utf8(line_slice) {
                                if let Some(re)=&regex_opt { if re.is_match(s) { selected=true; } }
                                else if whole_line {
                                    let cmp = if ignore_case { s.to_lowercase() } else { s.to_string() };
                                    selected = cmp==needle;
                                } else {
                                    let cmp = if ignore_case { s.to_lowercase() } else { s.to_string() };
                                    if !needle.is_empty() && cmp.contains(&needle) { selected=true; }
                                }
                            }
                            if invert { selected = !selected; }
                            if selected {
                                any_selected=true;
                                if quiet { early_exit=true; }
                                if count_only { cnt+=1; } else {
                                    if with_filename { let _=write!(stdout_handle, "{}:", p); }
                                    if show_line { let _=write!(stdout_handle, "{}:", line_no); }
                                    let _=stdout_handle.write_all(line_slice);
                                    let _=stdout_handle.write_all(b"\n");
                                }
                            }
                            start=i+1;
                        }
                        i+=1;
                    }
                    if start>0 { buf_acc.drain(0..start); }
                    if eof {
                        if !buf_acc.is_empty() {
                            line_no+=1;
                            let line_slice=&buf_acc[..];
                            let mut selected=false;
                            if let Ok(s)=std::str::from_utf8(line_slice) {
                                if let Some(re)=&regex_opt { if re.is_match(s) { selected=true; } }
                                else if whole_line {
                                    let cmp = if ignore_case { s.to_lowercase() } else { s.to_string() };
                                    selected = cmp==needle;
                                } else {
                                    let cmp = if ignore_case { s.to_lowercase() } else { s.to_string() };
                                    if !needle.is_empty() && cmp.contains(&needle) { selected=true; }
                                }
                            }
                            if invert { selected=!selected; }
                            if selected {
                                any_selected=true;
                                if quiet { early_exit=true; }
                                if count_only { cnt+=1; } else {
                                    if with_filename { let _=write!(stdout_handle, "{}:", p); }
                                    if show_line { let _=write!(stdout_handle, "{}:", line_no); }
                                    let _=stdout_handle.write_all(line_slice);
                                    let _=stdout_handle.write_all(b"\n");
                                }
                            }
                            buf_acc.clear();
                        }
                        break;
                    }
                    if early_exit { break; }
                },
                Ok(Err(code)) => { eprintln!("read error {:?}", code); return map_exit(&code); },
                Err(e) => { eprintln!("protocol read error: {e}"); return 6; },
            }
        }
        let _ = client.close(h);
        if count_only && !quiet {
            if with_filename { let _=write!(stdout_handle, "{}:", p); }
            let _ = writeln!(stdout_handle, "{}", cnt);
        }
        if quiet && any_selected { break; }
    }
    let _ = stdout_handle.flush();
    if any_selected { 0 } else { 1 }
}

// ---- tr (simple 1:1 mapping, optional -d delete) ----
fn run_builtin_tr(args:&Vec<String>, redir:&RedirSpec, _allow_read:&[String], _allow_write:&[String], _pass_fds:Option<(i32,i32)>) -> i32 {
    if args.is_empty() { eprintln!("tr: missing args"); return 2; }
    let mut delete_mode=false; let mut complement=false; let mut squeeze=false; let mut sets:Vec<String>=Vec::new();
    for a in args {
        if a=="-d" { delete_mode=true; }
        else if a=="-c" { complement=true; }
        else if a=="-s" { squeeze=true; }
        else { sets.push(a.clone()); }
    }
    // Helper: expand set syntax (ranges a-z and POSIX classes [:digit:] [:alpha:] [:alnum:] [:lower:] [:upper:] [:space:] [:blank:] [:xdigit:])
    fn expand_set(spec:&str) -> Vec<u8> {
        let mut out:Vec<u8>=Vec::new();
        let bytes=spec.as_bytes();
        let mut i=0;
        while i < bytes.len() {
            if i+1 < bytes.len() && bytes[i]==b'[' && bytes[i+1]==b':' {
                // class
                if let Some(end) = spec[i+2..].find(":]") { let name=&spec[i+2..i+2+end];
                    match name {
                        "digit" => out.extend(b"0123456789"),
                        "alpha" => { out.extend(b"ABCDEFGHIJKLMNOPQRSTUVWXYZ"); out.extend(b"abcdefghijklmnopqrstuvwxyz"); },
                        "alnum" => { out.extend(b"0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"); },
                        "lower" => out.extend(b"abcdefghijklmnopqrstuvwxyz"),
                        "upper" => out.extend(b"ABCDEFGHIJKLMNOPQRSTUVWXYZ"),
                        "space" => out.extend([b' ', b'\t', b'\n', b'\r', 0x0b, 0x0c]),
                        "blank" => out.extend([b' ', b'\t']),
                        "xdigit" => out.extend(b"0123456789ABCDEFabcdef"),
                        _ => {}
                    }
                    i += 2 + end + 2; // skip [:name:]
                    continue;
                }
            }
            if i+2 < bytes.len() && bytes[i+1]==b'-' && bytes[i]!=b'-' && bytes[i+2]!=b'-' {
                let start=bytes[i]; let end=bytes[i+2];
                if start <= end { for c in start..=end { out.push(c); } }
                else { for c in (end..=start).rev() { out.push(c); } }
                i+=3; continue;
            }
            out.push(bytes[i]); i+=1;
        }
        out
    }
    // Validate sets
    if delete_mode {
        if sets.len()!=1 { eprintln!("tr -d requires 1 set"); return 2; }
    } else {
        if squeeze && sets.len()==1 {
            // ok: squeeze only, no translation
        } else if sets.len()!=2 { eprintln!("tr requires 2 sets unless using -s only"); return 2; }
    }
    if !delete_mode && sets.len()==2 && sets[0].len()!=sets[1].len() { eprintln!("tr: sets must be same length (no padding implemented)"); return 2; }

    // Build delete/mapping data
    let set1_expanded = expand_set(&sets[0]);
    let set2_expanded = if delete_mode || (squeeze && sets.len()==1) { Vec::new() } else { expand_set(&sets[1]) };
    if !delete_mode && !set2_expanded.is_empty() && set1_expanded.len()!=set2_expanded.len() { eprintln!("tr: expanded sets must be same length"); return 2; }
    if complement && !delete_mode && !set2_expanded.is_empty() {
        eprintln!("tr: -c with translation not supported (only with -d/-s)");
        return 2;
    }

    // gather inputs
    let mut files:Vec<String>=Vec::new();
    if let Some(f)=&redir.in_file { files.push(f.clone()); }
    if files.is_empty() { return 0; }
    let client = child_mux();
    let mut out_handle: Option<u32> = None; // vfs handle if writing to file
    let mut stdout_handle = std::io::stdout();

    // Build translate map (identity by default)
    let mut map = [0u16;256]; for i in 0..256 { map[i]=i as u16; }
    if delete_mode {
        let mut del_mask=[false;256];
        if complement {
            for i in 0..256 { del_mask[i]=true; }
            for &b in &set1_expanded { del_mask[b as usize]=false; }
        } else {
            for &b in &set1_expanded { del_mask[b as usize]=true; }
        }
        for i in 0..256 { if del_mask[i] { map[i]=0xFFFF; } }
    } else if !set2_expanded.is_empty() {
        for (a,b) in set1_expanded.iter().zip(set2_expanded.iter()) { map[*a as usize] = *b as u16; }
    } // else: squeeze-only, keep identity map

    // Build squeeze set mask
    let mut squeeze_mask=[false;256];
    if squeeze {
        let base = if !delete_mode && !set2_expanded.is_empty() { &set2_expanded } else { &set1_expanded };
        if complement {
            for i in 0..256 { squeeze_mask[i]=true; }
            for &b in base { squeeze_mask[b as usize]=false; }
        } else {
            for &b in base { squeeze_mask[b as usize]=true; }
        }
    }

    if let Some((of, append))=&redir.out_file {
        let mode = if *append { "a" } else { "w" };
        match client.open(of, mode) { Ok(Ok(h))=> out_handle=Some(h), Ok(Err(code))=> { eprintln!("open {of} {mode} error: {:?}", code); return map_exit(&code); }, Err(e)=> { eprintln!("protocol open {of} {mode} error: {e}"); return 6; } }
    }
    for p in files.iter() {
        let h = match client.open(p, "r") { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open {p} r error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open {p} r error: {e}"); return 6; } };
        // keep last output byte for squeeze across chunks
        let mut last_out:Option<u8>=None; let mut last_in_s=false;
        loop {
            match client.read_chunk(h, 8192) {
                Ok(Ok((data,eof))) => {
                    if data.is_empty() && eof { break; }
                    if !data.is_empty() {
                        let mut out:Vec<u8>=Vec::with_capacity(data.len());
                        for &b in &data {
                            let m = map[b as usize];
                            if m==0xFFFF { continue; } // delete
                            let ob = if m<256 { m as u8 } else { b };
                            if squeeze {
                                let in_s = squeeze_mask[ob as usize];
                                if let Some(prev)=last_out {
                                    if in_s && last_in_s && prev==ob { /* squeeze duplicate */ }
                                    else { out.push(ob); last_out=Some(ob); last_in_s=in_s; }
                                } else { out.push(ob); last_out=Some(ob); last_in_s=in_s; }
                            } else {
                                out.push(ob);
                            }
                        }
                        if let Some(hout)=out_handle {
                            if let Err(e)=client.write_chunk(hout, &out) { eprintln!("write error: {e}"); return 6; }
                        } else if let Err(e)=stdout_handle.write_all(&out) { eprintln!("stdout write error: {e}"); return 7; }
                    }
                    if eof { break; }
                }
                Ok(Err(code)) => { eprintln!("read error: {:?}", code); return map_exit(&code); }
                Err(e) => { eprintln!("protocol read error: {e}"); return 6; }
            }
        }
        let _ = client.close(h);
    }
    if let Some(hout)=out_handle { let _=client.close(hout); } else { let _ = stdout_handle.flush(); }
    0
}

// ---- sed (subset: only single substitution expression s<delim>pat<delim>repl<delim>[flags], literal match (no regex), flags: g) ----
fn run_builtin_sed(args:&Vec<String>, redir:&RedirSpec, _allow_read:&[String], _allow_write:&[String], _pass_fds:Option<(i32,i32)>) -> i32 {
    if args.is_empty() { eprintln!("sed: missing script"); return 2; }
    let script = &args[0];
    let mut files:Vec<String> = if args.len()>1 { args[1..].to_vec() } else { Vec::new() };
    if files.is_empty() { if let Some(f)=&redir.in_file { files.push(f.clone()); } }
    if files.is_empty() { return 0; }
    // parse s command
    if !script.starts_with('s') || script.len()<2 { eprintln!("sed: only s/// supported"); return 2; }
    let delim = script.chars().nth(1).unwrap();
    let mut i=2; let chars:Vec<char>=script.chars().collect();
    let mut cur=String::new(); let mut parts:Vec<String>=Vec::new(); let mut escape=false;
    while i < chars.len() {
        let c=chars[i]; i+=1;
        if escape { cur.push(c); escape=false; continue; }
        if c=='\\' { escape=true; continue; }
        if c==delim { parts.push(cur.clone()); cur.clear(); if parts.len()==2 { break; } continue; }
        cur.push(c);
    }
    if parts.len()!=2 { eprintln!("sed: bad script"); return 2; }
    let pattern = parts[0].replace(&format!("\\{}",delim), &delim.to_string());
    let repl = parts[1].replace(&format!("\\{}",delim), &delim.to_string());
    // collect replacement flags after second delimiter until delimiter again
    // we already broke after replacement delimiter; gather flags rest of script
    let mut flags=String::new(); while i < chars.len() { flags.push(chars[i]); i+=1; }
    let global = flags.contains('g');
    let use_regex = flags.contains('r') || flags.contains('E'); // extended regex flags
    let ignore_case = flags.contains('I'); // custom: case-insensitive regex
    let client = child_mux();
    let mut out_handle: Option<u32> = None;
    if let Some((path, append))=&redir.out_file { let mode = if *append { "a" } else { "w" }; match client.open(path, mode) { Ok(Ok(h))=>out_handle=Some(h), Ok(Err(code))=>{ eprintln!("open {path} {mode} error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open {path} {mode} error: {e}"); return 6; } } }
    let mut stdout_handle = std::io::stdout();
    for p in files.iter() {
        let h = match client.open(p, "r") { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open {p} r error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open {p} r error: {e}"); return 6; } };
        let mut buf_acc:Vec<u8>=Vec::new();
        loop {
            match client.read_chunk(h, 4096) {
                Ok(Ok((data,eof))) => {
                    if data.is_empty() && eof { break; }
                    buf_acc.extend_from_slice(&data);
                    let mut start=0; let mut idx=0;
                    while idx < buf_acc.len() {
                        if buf_acc[idx]==b'\n' { // process line
                            let line_bytes=&buf_acc[start..idx];
                            let mut line = String::from_utf8_lossy(line_bytes).to_string();
                            if !pattern.is_empty() {
                                if use_regex {
                                    let pat_src = if ignore_case { format!("(?i){}", pattern) } else { pattern.clone() };
                                    let pat = match Regex::new(&pat_src) { Ok(r)=>r, Err(e)=> { eprintln!("sed: invalid regex: {e}"); return 2; } };
                                    // Support & and backrefs in replacement when regex
                                    let replaced = if global { pat.replace_all(&line, |caps: &regex::Captures| {
                                        let mut out=String::new();
                                        let mut chars=repl.chars().peekable();
                                        while let Some(c)=chars.next() {
                                            if c=='&' { out.push_str(caps.get(0).map(|m| m.as_str()).unwrap_or("")); }
                                            else if c=='\\' {
                                                if let Some(n)=chars.peek().cloned() {
                                                    if n.is_ascii_digit() {
                                                        let _=chars.next();
                                                        let idx=(n as u8 - b'0') as usize;
                                                        out.push_str(caps.get(idx).map(|m| m.as_str()).unwrap_or(""));
                                                    } else { out.push(n); let _=chars.next(); }
                                                }
                                            } else { out.push(c); }
                                        }
                                        out
                                    }).to_string() } else { pat.replace(&line, |caps: &regex::Captures| {
                                        let mut out=String::new();
                                        let mut chars=repl.chars().peekable();
                                        while let Some(c)=chars.next() {
                                            if c=='&' { out.push_str(caps.get(0).map(|m| m.as_str()).unwrap_or("")); }
                                            else if c=='\\' {
                                                if let Some(n)=chars.peek().cloned() {
                                                    if n.is_ascii_digit() {
                                                        let _=chars.next();
                                                        let idx=(n as u8 - b'0') as usize;
                                                        out.push_str(caps.get(idx).map(|m| m.as_str()).unwrap_or(""));
                                                    } else { out.push(n); let _=chars.next(); }
                                                }
                                            } else { out.push(c); }
                                        }
                                        out
                                    }).to_string() };
                                    line = replaced;
                                } else {
                                    if global { let mut out_line=String::new(); let mut pos=0; while let Some(pos2)=line[pos..].find(&pattern) { out_line.push_str(&line[pos..pos+pos2]); out_line.push_str(&repl); pos += pos2 + pattern.len(); } out_line.push_str(&line[pos..]); line=out_line; } else if let Some(pos2)=line.find(&pattern) { line = format!("{}{}{}", &line[..pos2], repl, &line[pos2+pattern.len()..]); }
                                }
                            }
                            let mut out_line = line.into_bytes(); out_line.push(b'\n');
                            if let Some(hout)=out_handle { match client.write_chunk(hout, &out_line) { Ok(Ok(_))=>{}, Ok(Err(code))=>{ eprintln!("write error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol write error: {e}"); return 6; } } }
                            else if let Err(e)=stdout_handle.write_all(&out_line) { eprintln!("stdout write error: {e}"); return 7; }
                            start=idx+1;
                        }
                        idx+=1;
                    }
                    if start>0 { buf_acc.drain(0..start); }
                    if eof { // flush last partial line
                        if !buf_acc.is_empty() { let mut line = String::from_utf8_lossy(&buf_acc).to_string(); if !pattern.is_empty() { if use_regex { let pat_src = if ignore_case { format!("(?i){}", pattern) } else { pattern.clone() }; let pat = match Regex::new(&pat_src) { Ok(r)=>r, Err(e)=>{ eprintln!("sed: invalid regex: {e}"); return 2; } }; let replaced = if global { pat.replace_all(&line, |caps: &regex::Captures| { let mut out=String::new(); let mut chars=repl.chars().peekable(); while let Some(c)=chars.next() { if c=='&' { out.push_str(caps.get(0).map(|m| m.as_str()).unwrap_or("")); } else if c=='\\' { if let Some(n)=chars.peek().cloned() { if n.is_ascii_digit() { let _=chars.next(); let idx=(n as u8 - b'0') as usize; out.push_str(caps.get(idx).map(|m| m.as_str()).unwrap_or("")); } else { out.push(n); let _=chars.next(); } } } else { out.push(c); } } out }).to_string() } else { pat.replace(&line, |caps: &regex::Captures| { let mut out=String::new(); let mut chars=repl.chars().peekable(); while let Some(c)=chars.next() { if c=='&' { out.push_str(caps.get(0).map(|m| m.as_str()).unwrap_or("")); } else if c=='\\' { if let Some(n)=chars.peek().cloned() { if n.is_ascii_digit() { let _=chars.next(); let idx=(n as u8 - b'0') as usize; out.push_str(caps.get(idx).map(|m| m.as_str()).unwrap_or("")); } else { out.push(n); let _=chars.next(); } } } else { out.push(c); } } out }).to_string() }; line = replaced; } else { if global { let mut out_line=String::new(); let mut pos=0; while let Some(pos2)=line[pos..].find(&pattern) { out_line.push_str(&line[pos..pos+pos2]); out_line.push_str(&repl); pos+=pos2+pattern.len(); } out_line.push_str(&line[pos..]); line=out_line; } else if let Some(pos2)=line.find(&pattern) { line = format!("{}{}{}", &line[..pos2], repl, &line[pos2+pattern.len()..]); } } } let out_line = line.into_bytes(); if let Some(hout)=out_handle { match client.write_chunk(hout, &out_line) { Ok(Ok(_))=>{}, Ok(Err(code))=>{ eprintln!("write error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol write error: {e}"); return 6; } } } else if let Err(e)=stdout_handle.write_all(&out_line) { eprintln!("stdout write error: {e}"); return 7; } buf_acc.clear(); }
                        break; }
                }
                Ok(Err(code)) => { eprintln!("read error: {:?}", code); return map_exit(&code); }
                Err(e) => { eprintln!("protocol read error: {e}"); return 6; }
            }
        }
        let _ = client.close(h);
    }
    if let Some(hout)=out_handle { let _=client.close(hout); } else { let _=stdout_handle.flush(); }
    0
}

// ---- llmsh (invoke nested shell) ----
fn run_builtin_llmsh(args:&Vec<String>, _redir:&RedirSpec, _allow_read:&[String], _allow_write:&[String], pass_fds:Option<(i32,i32)>) -> i32 {
    // Only supported form: llmsh -c 'script'
    let mut idx=0; let mut script:Option<String>=None;
    while idx < args.len() {
        if args[idx]=="-c" { if idx+1 >= args.len() { eprintln!("llmsh: missing script after -c"); return 2; } script=Some(args[idx+1].clone()); idx+=2; }
        else { eprintln!("llmsh: unsupported arg {} (only -c allowed)", args[idx]); return 2; }
    }
    let script = if let Some(s) = script { s } else { eprintln!("llmsh: -c required"); return 2; };
    // Determine path to llmsh binary; allow override via LLMSH_BIN else assume 'llmsh'
    let bin = env::var("LLMSH_BIN").unwrap_or_else(|_| "llmsh".to_string());
    let mut cmd = Command::new(&bin);
    cmd.arg("-c").arg(&script);
    // Nested shell does not need -i / -o propagation (server-managed paths)
    if let Some((rfd,wfd))=pass_fds { cmd.arg("--vfs-fds").arg(format!("{},{}", rfd, wfd)); }
    match cmd.status() { Ok(st)=> st.code().unwrap_or(1), Err(e)=> { eprintln!("llmsh exec failed: {e}"); 5 } }
}

struct HelpEntry { name:&'static str, usage:&'static str, desc:&'static str,
    options:&'static [(&'static str,&'static str)], examples:&'static [(&'static str,&'static str)], related:&'static [&'static str] }

static HELP_ENTRIES:&[HelpEntry] = &[
    HelpEntry { name:"cat", usage:"cat [file...]", desc:"concatenate files and print on stdout", options:&[], examples:&[("cat file.txt","Display contents"),("cat a b","Concatenate a and b")], related:&["head","tail"] },
    HelpEntry { name:"head", usage:"head [-n lines] [file...]", desc:"output the first part of files", options:&[("-n N","output first N lines")], examples:&[("head -10 file.txt","Show first 10 lines")], related:&["tail","cat"] },
    HelpEntry { name:"tail", usage:"tail [-n lines] [file...]", desc:"output the last part of files", options:&[("-n N","output last N lines")], examples:&[("tail -20 file.txt","Show last 20 lines")], related:&["head","cat"] },
    HelpEntry { name:"wc", usage:"wc [options] [file...]", desc:"print line, word, byte counts", options:&[("-l","lines only"),("-w","words only"),("-c","bytes only")], examples:&[("wc file.txt","Show all counts")], related:&["grep","cat"] },
    HelpEntry { name:"grep", usage:"grep [-i] [-n] [-E] [-v] [-c] [-x] [-w] [-H|-h] [-q] pattern [file...]", desc:"search lines matching pattern (literal by default; -E enables regex)", options:&[("-i","ignore case"),("-n","show line numbers"),("-E","regex pattern (alias -r)"),("-v","invert match"),("-c","count matches per file"),("-x","match whole line"),("-w","match word"),("-H","force show filename"),("-h","suppress filename"),("-q","quiet (exit on first match, no output)")], examples:&[("grep foo file","Find foo"),("grep -i -E 'foo|bar' file","Regex OR match"),("cat f | grep -n bar","Pipe with line numbers"),("grep -c pattern a b","Count per file"),("grep -w err log","Word match"),("grep -q pat file && echo found","Quiet check")], related:&["sed","wc"] },
    HelpEntry { name:"tr", usage:"tr [-d] [-c] [-s] set1 [set2]", desc:"translate or delete characters with ranges/classes; -s squeeze repeats; -c complement for -d/-s", options:&[("-d","delete chars in set1"),("-c","complement set1 (with -d/-s)"),("-s","squeeze repeats of set1/translated set")], examples:&[("tr abc xyz < in","Map a->x b->y c->z"),("tr 'a-z' 'A-Z' < in","Uppercase"),("tr -d '[:digit:]' < in","Delete digits"),("tr -s ' ' < in","Squeeze spaces")], related:&["sed"] },
    HelpEntry { name:"sed", usage:"sed s/pat/repl/[gEI] [file...]", desc:"stream substitution (literal by default; E enables regex; I ignore-case for regex). Supports & and \\1..\\9 in repl when regex.", options:&[("g","global replace"),("E","enable regex (alias r)"),("I","ignore case (with regex)")], examples:&[("sed s/foo/bar/ file","Replace first foo"),("sed s/foo/bar/g file","Replace all foo"),("sed 's/(foo)(bar)/X\\1Y\\2/E' file","Backrefs replace"),("sed 's/foo/bar/EI' file","Case-insensitive")], related:&["grep","tr"] },
    HelpEntry { name:"help", usage:"help [command]", desc:"list commands or show detailed help", options:&[], examples:&[("help","List commands"),("help grep","Show grep help")], related:&[] },
    HelpEntry { name:"llmsh", usage:"llmsh [-c 'pipeline'] [--vfs-fds R,W | (-i path ... -o path ...)]", desc:"invoke nested llmsh instance (only -c supported; --vfs-fds is exclusive with -i/-o)", options:&[("-c SCRIPT","execute pipeline SCRIPT"),("--vfs-fds R,W","reuse parent VFS pipe fds (no -i/-o allowed)")], examples:&[("llmsh -c 'cat file.txt' -i file.txt","Run with allow list"),("llmsh -c 'grep foo a | wc -l' --vfs-fds 5,6","Nested pipeline sharing VFS pipes")], related:&["help"] },
];

fn find_help(name:&str)->Option<&'static HelpEntry>{
    for e in HELP_ENTRIES { if e.name==name { return Some(e); } }
    None
}

fn format_entry(e:&HelpEntry)->String {
    let mut s=String::new();
    use std::fmt::Write as _;
    let _=write!(s,"NAME\n    {} - {}\n\n", e.name, e.desc);
    let _=write!(s,"USAGE\n    {}\n\n", e.usage);
    if !e.options.is_empty() { let _=write!(s,"OPTIONS\n"); for (f,d) in e.options { let _=write!(s,"    {:<12} {}\n", f, d); } let _=write!(s,"\n"); }
    if !e.examples.is_empty() { let _=write!(s,"EXAMPLES\n"); for (cmd,desc) in e.examples { let _=write!(s,"    {}\n        {}\n\n", cmd, desc); } }
    if !e.related.is_empty() { let _=write!(s,"SEE ALSO\n    {}\n", e.related.join(", ")); }
    s
}

fn run_builtin_help(args:&Vec<String>) -> i32 {
    let mut stdout = std::io::stdout();
    if args.is_empty() {
        // list
        let mut names:Vec<&str>=HELP_ENTRIES.iter().map(|e| e.name).collect();
        names.sort();
        let _=writeln!(stdout, "LLMSH Builtins:\n");
        for (i,n) in names.iter().enumerate() { if i%6==0 { let _=write!(stdout, "    "); } let _=write!(stdout, "{:<10}", n); if i%6==5 { let _=write!(stdout, "\n"); } }
        if names.len()%6!=0 { let _=write!(stdout, "\n"); }
        let _=writeln!(stdout, "\nUse: help <command>  for details\n");
        let _=stdout.flush();
        return 0;
    }
    let mut status=0;
    for name in args { if let Some(e) = find_help(name) { let txt = format_entry(e); let _=write!(stdout, "{}", txt); } else { let _=writeln!(stdout, "no help for command: {}", name); status=2; } }
    let _=stdout.flush();
    status
}
// ----- Duplicate legacy block removed above (parsing + skeleton impl) -----
