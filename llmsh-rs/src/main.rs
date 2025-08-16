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
mod mux;
use mux::{child_mux, init_child_mux, VfsErrorCode};
use regex::Regex;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum BuiltinKind { Cat, Head, Tail, Wc, Grep, Tr, Sed, Help, Llmsh, Exit }

#[derive(Debug, Default, Clone)]
struct RedirSpec { in_file: Option<String>, out_file: Option<(String,bool)> }

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
    if ["cat","head","tail","wc","grep","tr","sed","help","llmsh","exit"].contains(&cmd.as_str()) {
        if !args.is_empty() && cmd == "cat" { redir.in_file = None; }
        let kind = match cmd.as_str() {
            "cat"=>BuiltinKind::Cat,
            "head"=>BuiltinKind::Head,
            "tail"=>BuiltinKind::Tail,
            "wc"=>BuiltinKind::Wc,
            "grep"=>BuiltinKind::Grep,
            "tr"=>BuiltinKind::Tr,
            "sed"=>BuiltinKind::Sed,
            "help"=>BuiltinKind::Help,
            "llmsh"=>BuiltinKind::Llmsh,
            "exit"=>BuiltinKind::Exit,
            _=>BuiltinKind::Cat
        };
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
        BuiltinKind::Exit => run_builtin_exit(args),
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
    // Parse CLI
    let mut args_iter = env::args().skip(1);
    let mut script_arg: Option<String> = None; // -c SCRIPT
    let mut free_args: Vec<String> = Vec::new(); // for file (no -c)
    let mut allow_read: Vec<String> = Vec::new();
    let mut allow_write: Vec<String> = Vec::new();
    let mut pass_fds: Option<(i32,i32)> = None;
    let mut vfsd_arg: Option<String> = None; // --vfsd explicit binary path
    while let Some(a) = args_iter.next() {
        match a.as_str() {
            "-c" => { script_arg = Some(args_iter.next().ok_or_else(|| anyhow::anyhow!("missing script after -c"))?); }
            "-i" | "--input" => { let v = args_iter.next().ok_or_else(|| anyhow::anyhow!("missing value after -i"))?; allow_read.push(v); }
            "-o" | "--output" => { let v = args_iter.next().ok_or_else(|| anyhow::anyhow!("missing value after -o"))?; allow_write.push(v); }
            "--vfs-fds" => { let v = args_iter.next().ok_or_else(|| anyhow::anyhow!("missing value after --vfs-fds"))?; let parts: Vec<&str> = v.split(',').collect(); if parts.len()!=2 { anyhow::bail!("--vfs-fds requires '<rfd>,<wfd>'"); } let rfd: i32 = parts[0].parse().map_err(|_| anyhow::anyhow!("invalid rfd"))?; let wfd: i32 = parts[1].parse().map_err(|_| anyhow::anyhow!("invalid wfd"))?; pass_fds = Some((rfd,wfd)); }
            "--vfsd" => { let v = args_iter.next().ok_or_else(|| anyhow::anyhow!("missing value after --vfsd"))?; vfsd_arg = Some(v); }
            other if other.starts_with('-') => { eprintln!("unknown arg: {other}"); }
            other => { free_args.push(other.to_string()); }
        }
    }
    if pass_fds.is_some() && (!allow_read.is_empty() || !allow_write.is_empty()) {
        eprintln!("error: --vfs-fds cannot be combined with -i/-o options");
        std::process::exit(2);
    }

    // Resolve vfsd
    fn is_executable(p: &Path) -> bool { p.is_file() && std::fs::metadata(p).map(|m| m.permissions().mode() & 0o111 != 0).unwrap_or(false) }
    fn resolve_vfsd(arg: &Option<String>) -> Result<String> {
        if let Some(a) = arg { let p = PathBuf::from(a); if is_executable(&p) { return Ok(p.to_string_lossy().to_string()); } else { anyhow::bail!("--vfsd path not executable or not found: {a}"); } }
        if let Ok(envv) = env::var("LLMSH_VFSD_BIN") { let p = PathBuf::from(&envv); if is_executable(&p) { return Ok(p.to_string_lossy().to_string()); } }
        if let Ok(exe) = env::current_exe() { if let Some(dir) = exe.parent() {
                // 1) sibling 'vfsd'
                let cand = dir.join("vfsd"); if is_executable(&cand) { return Ok(cand.to_string_lossy().to_string()); }
                // 2) repo dev path: ../../../vfsd/target/debug/vfsd (from llmsh-rs/target/debug)
                let mut up = dir.to_path_buf();
                for _ in 0..3 { if let Some(p) = up.parent() { up = p.to_path_buf(); } }
                let dev_cand = up.join("vfsd").join("target").join("debug").join("vfsd");
                if is_executable(&dev_cand) { return Ok(dev_cand.to_string_lossy().to_string()); }
            } }
        if let Some(path_var) = env::var_os("PATH") { for comp in env::split_paths(&path_var) { let cand = comp.join("vfsd"); if is_executable(&cand) { return Ok(cand.to_string_lossy().to_string()); } } }
        anyhow::bail!("vfsd binary not found: specify --vfsd or set LLMSH_VFSD_BIN or place 'vfsd' alongside llmsh or in PATH")
    }
    let resolved_vfsd = match resolve_vfsd(&vfsd_arg) { Ok(p)=>p, Err(e)=> { eprintln!("vfsd resolve error: {e}"); std::process::exit(9); } };
    env::set_var("LLMSH_VFSD_BIN", &resolved_vfsd);

    // Helper: remove lines starting with '#' as comments (used for -c only). For file/stdin, we iterate lines instead.
    fn strip_hash_comments(s:&str)->String{ s.lines().filter(|l| !l.trim_start().starts_with('#')).collect::<Vec<_>>().join("\n") }
    // Execute multi-line script text line-by-line, skipping empty and comment lines.
    fn exec_script_text(text:&str, allow_read:&[String], allow_write:&[String], pass_fds:Option<(i32,i32)>) -> Result<i32> {
        let mut last=0;
        for raw in text.lines() {
            let l = raw.trim();
            if l.is_empty() || l.starts_with('#') { continue; }
            let units = parse_pipeline(l);
            // Special-case: single `exit [N]` line terminates script immediately
            if units.len()==1 {
                if let ExecUnit::Builtin{ kind: BuiltinKind::Exit, args, .. } = &units[0] {
                    let code = parse_exit_code(args, last);
                    return Ok(code);
                }
            }
            let code = spawn_pipeline(&units, allow_read, allow_write, pass_fds)?;
            last = code;
        }
        Ok(last)
    }

    // Execution modes
    if let Some(script) = script_arg {
        // -c supplied: run pipeline(s) from string (after removing # comments)
    let s = strip_hash_comments(&script);
    let units = parse_pipeline(&s);
    // Special-case: single `exit [N]` should terminate this llmsh process
    if units.len()==1 { if let ExecUnit::Builtin{ kind: BuiltinKind::Exit, args, .. } = &units[0] {
        let code = parse_exit_code(args, 0);
        std::process::exit(code);
        } }
        let code = spawn_pipeline(&units, &allow_read, &allow_write, pass_fds)?;
        std::process::exit(code);
    }

    // If first free arg exists, treat as script file (rest currently error as未対応)
    if !free_args.is_empty() {
        if free_args.len() > 1 { eprintln!("error: extra positional arguments are not supported yet"); std::process::exit(2); }
        let path = &free_args[0];
    let content = std::fs::read_to_string(path).map_err(|e| anyhow::anyhow!("failed to read script file {path}: {e}"))?;
    // Special-case: fileの1行がexitだけ、などの単純ケースは通常処理に任せる
    let code = exec_script_text(&content, &allow_read, &allow_write, pass_fds)?;
        std::process::exit(code);
    }

    // No -c and no file: decide from stdin TTY
    let stdin_is_tty = atty::is(atty::Stream::Stdin);
    if stdin_is_tty {
        // Simple REPL using std::io (no history)
        use std::io::{self};
        let mut input = String::new();
        let mut last_status:i32 = 0;
        loop {
            input.clear();
            eprint!("llmsh> ");
            io::stderr().flush().ok();
            let n = io::stdin().read_line(&mut input)?;
            if n == 0 { break; } // EOF
            let l = strip_hash_comments(input.trim_end());
            if l.trim().is_empty() { continue; }
            let units = parse_pipeline(&l);
            // REPL: single `exit [N]` exits this process immediately
            if units.len()==1 { if let ExecUnit::Builtin{ kind: BuiltinKind::Exit, args, .. } = &units[0] {
                    let code = parse_exit_code(args, last_status);
                    std::process::exit(code);
                } }
            let code = spawn_pipeline(&units, &allow_read, &allow_write, pass_fds)?;
            last_status = code;
            if code != 0 { eprintln!("exit status: {code}"); }
        }
        Ok(())
    } else {
        // Non-tty stdin: read all and execute as script
    use std::io::Read; let mut buf=String::new(); let _=std::io::stdin().read_to_string(&mut buf)?;
        let code = exec_script_text(&buf, &allow_read, &allow_write, pass_fds)?;
        std::process::exit(code);
    }
}

fn parse_exit_code(args:&Vec<String>, default:i32)->i32{
    if args.is_empty() { return default; }
    match args[0].parse::<i32>() { Ok(v)=>v, Err(_)=>default }
}

fn run_builtin_exit(args:&Vec<String>) -> i32 {
    // In child/pipeline contexts, we just return the desired code and the caller will process.exit(code)
    parse_exit_code(args, 0)
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

// ---- grep (expanded flags: -i -n -E|-r -v -c -x -w -H/-h -q, plus -o and -A/-B/-C) ----
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
    let mut only_matching=false; // -o
    let mut ctx_after:usize=0; // -A N
    let mut ctx_before:usize=0; // -B N
    let mut force_with_filename:Option<bool>=None; // -H(true)/-h(false)
    let mut quiet=false;       // -q
    while idx < args.len() && args[idx].starts_with('-') && args[idx] != "-" {
        let a = &args[idx];
        // Context options: support both split form (-A 2) and compact form (-A2)
        if a=="-A" || a=="-B" || a=="-C" {
            if idx+1>=args.len() { eprintln!("grep: {} requires a number", a); return 2; }
            let n:usize = match args[idx+1].parse(){ Ok(v)=>v, Err(_)=>{ eprintln!("grep: invalid number for {}", a); return 2; } };
            match a.as_str() { "-A"=>ctx_after=n, "-B"=>ctx_before=n, "-C"=>{ctx_after=n;ctx_before=n;}, _=>{} }
            idx+=2; continue;
        } else if a.starts_with("-A") && a.len()>2 {
            let num=&a[2..]; let n:usize = match num.parse(){ Ok(v)=>v, Err(_)=>{ eprintln!("grep: invalid number for -A"); return 2; } }; ctx_after=n; idx+=1; continue;
        } else if a.starts_with("-B") && a.len()>2 {
            let num=&a[2..]; let n:usize = match num.parse(){ Ok(v)=>v, Err(_)=>{ eprintln!("grep: invalid number for -B"); return 2; } }; ctx_before=n; idx+=1; continue;
        } else if a.starts_with("-C") && a.len()>2 {
            let num=&a[2..]; let n:usize = match num.parse(){ Ok(v)=>v, Err(_)=>{ eprintln!("grep: invalid number for -C"); return 2; } }; ctx_after=n; ctx_before=n; idx+=1; continue;
        }
        for ch in a.chars().skip(1) {
            match ch {
                'i' => ignore_case=true,
                'n' => show_line=true,
                'E' | 'r' => use_regex=true,
                'v' => invert=true,
                'c' => count_only=true,
                'x' => whole_line=true,
                'w' => word_match=true,
                'o' => only_matching=true,
                'H' => force_with_filename=Some(true),
                'h' => force_with_filename=Some(false),
                'q' => quiet=true,
                _ => {}
            }
        }
        idx+=1;
    }
    if idx>=args.len() { eprintln!("grep: missing pattern"); return 2; }
    let pattern_raw=args[idx].clone(); idx+=1;
    let mut files:Vec<String> = if idx < args.len() { args[idx..].to_vec() } else { Vec::new() };
    if files.is_empty() { if let Some(f)=&redir.in_file { files.push(f.clone()); } }
    if files.is_empty() { return 0; }
    let with_filename_default = files.len()>1;
    let with_filename = force_with_filename.unwrap_or(with_filename_default);
    if invert && only_matching { // -o with -v is nonsensical; disable -o
        only_matching=false;
    }

    // Build matcher
    let mut regex_opt: Option<Regex> = None;
    let mut needle: String = String::new();
    if use_regex {
        let mut pat = pattern_raw.clone();
        if word_match { pat = format!("\\b{}\\b", pat); }
        if whole_line { pat = format!("^{}$", pat); }
        if ignore_case { pat = format!("(?i){}", pat); }
        match Regex::new(&pat) { Ok(r)=>regex_opt=Some(r), Err(e)=>{ eprintln!("grep: invalid regex: {e}"); return 2; } }
    } else {
        if word_match || whole_line { // use regex for these literal modes
            let mut esc = regex::escape(&pattern_raw);
            if word_match { esc = format!("\\b{}\\b", esc); }
            if whole_line { esc = format!("^{}$", esc); }
            if ignore_case { esc = format!("(?i){}", esc); }
            match Regex::new(&esc) { Ok(r)=>{ regex_opt=Some(r); use_regex=true; }, Err(e)=>{ eprintln!("grep: internal regex build failed: {e}"); return 2; } }
        } else {
            needle = pattern_raw.clone();
            if ignore_case { needle = needle.to_lowercase(); }
        }
    }

    let client = child_mux();
    let mut stdout_handle = std::io::stdout();
    let mut any_selected=false;
    use std::collections::VecDeque;

    for p in files.iter() {
        let h = match client.open(p, "r") { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open {p} r error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open {p} r error: {e}"); return 6; } };
        let mut buf_acc:Vec<u8>=Vec::new();
        let mut line_no:usize=0;
        let mut cnt:usize=0;
        let mut prev_lines:VecDeque<(usize,Vec<u8>)> = VecDeque::new();
        let mut after_rem:usize=0;
        let mut last_printed:usize=0; // last line number printed (to avoid duplicates)
        let mut early_exit=false;
        loop {
            match client.read_chunk(h, 4096) {
                Ok(Ok((data,eof))) => {
                    if data.is_empty() && eof { // handle possible trailing partial line then break
                        if !buf_acc.is_empty() {
                            line_no+=1;
                            let line_slice=&buf_acc[..];
                            // evaluate selection
                            let mut selected=false;
                            if let Some(re)=&regex_opt {
                                if let Ok(s)=std::str::from_utf8(line_slice) { selected = re.is_match(s); }
                            } else {
                                if let Ok(s)=std::str::from_utf8(line_slice) {
                                    if whole_line { let cmp = if ignore_case { s.to_lowercase() } else { s.to_string() }; selected = cmp==needle; }
                                    else { let hay = if ignore_case { s.to_lowercase() } else { s.to_string() }; if !needle.is_empty() && hay.contains(&needle) { selected=true; } }
                                } else {
                                    // non-utf8 fallback: simple byte search (case-sensitive)
                                    if !needle.is_empty() && !ignore_case && !use_regex { if line_slice.windows(needle.as_bytes().len()).any(|w| w==needle.as_bytes()) { selected=true; } }
                                }
                            }
                            if invert { selected = !selected; }
                            if selected {
                                any_selected=true;
                                if count_only {
                                    if only_matching {
                                        if let Some(re)=&regex_opt {
                                            if let Ok(s)=std::str::from_utf8(line_slice) { cnt += re.find_iter(s).count(); }
                                        } else if whole_line { cnt += 1; }
                                        else if let Ok(s)=std::str::from_utf8(line_slice) { let hay = if ignore_case { s.to_lowercase() } else { s.to_string() }; let mut pos=0; while let Some(off)=hay[pos..].find(&needle) { cnt+=1; pos+=off+needle.len(); } }
                                    } else { cnt+=1; }
                                } else {
                                    // before-context
                                    if ctx_before>0 {
                                        while let Some((ln,_)) = prev_lines.front() { if *ln <= last_printed { prev_lines.pop_front(); } else { break; } }
                                        for (ln,bytes) in prev_lines.iter() { if *ln > last_printed { if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", ln); } let _=stdout_handle.write_all(bytes); let _=stdout_handle.write_all(b"\n"); last_printed=*ln; } }
                                    }
                                    // match line
                                    if only_matching {
                                        if let Some(re)=&regex_opt {
                                            if let Ok(s)=std::str::from_utf8(line_slice) { for m in re.find_iter(s) { if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(m.as_str().as_bytes()); let _=stdout_handle.write_all(b"\n"); } }
                                        } else if whole_line {
                                            if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(line_slice); let _=stdout_handle.write_all(b"\n");
                                        } else if let Ok(s)=std::str::from_utf8(line_slice) {
                                            let hay = if ignore_case { s.to_lowercase() } else { s.to_string() };
                                            let mut pos=0; while let Some(off)=hay[pos..].find(&needle) { let start_o=pos+off; let end_o=start_o+needle.len(); if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(&line_slice[start_o..end_o]); let _=stdout_handle.write_all(b"\n"); pos=end_o; }
                                        }
                                    } else {
                                        if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(line_slice); let _=stdout_handle.write_all(b"\n");
                                    }
                                    // At EOF trailing-line processing, there will be no further lines,
                                    // so we avoid updating after-context counters to prevent unused assignment warnings.
                                }
                            } else if after_rem>0 && !count_only && !quiet {
                                if line_no > last_printed { if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(line_slice); let _=stdout_handle.write_all(b"\n"); last_printed=line_no; let _ = last_printed; }
                                if after_rem>0 { after_rem-=1; let _ = after_rem; }
                            }
                            // maintain before-context buffer
                            if ctx_before>0 { prev_lines.push_back((line_no, line_slice.to_vec())); if prev_lines.len()>ctx_before { let _=prev_lines.pop_front(); } }
                            buf_acc.clear();
                        }
                        break; }
                    if !data.is_empty() { buf_acc.extend_from_slice(&data); }
                    // process any complete lines in buffer
                    let mut start=0usize; let mut i=0usize;
                    while i < buf_acc.len() {
                        if buf_acc[i]==b'\n' {
                            let line_slice=&buf_acc[start..i];
                            line_no+=1;
                            // evaluate selection
                            let mut selected=false;
                            if let Some(re)=&regex_opt {
                                if let Ok(s)=std::str::from_utf8(line_slice) { selected = re.is_match(s); }
                            } else {
                                if let Ok(s)=std::str::from_utf8(line_slice) {
                                    if whole_line { let cmp = if ignore_case { s.to_lowercase() } else { s.to_string() }; selected = cmp==needle; }
                                    else { let hay = if ignore_case { s.to_lowercase() } else { s.to_string() }; if !needle.is_empty() && hay.contains(&needle) { selected=true; } }
                                } else {
                                    // non-utf8 fallback: simple byte search (case-sensitive)
                                    if !needle.is_empty() && !ignore_case && !use_regex { if line_slice.windows(needle.as_bytes().len()).any(|w| w==needle.as_bytes()) { selected=true; } }
                                }
                            }
                            if invert { selected = !selected; }
                            if selected {
                                any_selected=true;
                                if quiet { early_exit=true; }
                                if count_only {
                                    if only_matching {
                                        if let Some(re)=&regex_opt { if let Ok(s)=std::str::from_utf8(line_slice) { cnt += re.find_iter(s).count(); } }
                                        else if whole_line { cnt += 1; }
                                        else if let Ok(s)=std::str::from_utf8(line_slice) { let hay = if ignore_case { s.to_lowercase() } else { s.to_string() }; let mut pos=0; while let Some(off)=hay[pos..].find(&needle) { cnt+=1; pos+=off+needle.len(); } }
                                    } else { cnt+=1; }
                                } else {
                                    // before-context
                                    if ctx_before>0 {
                                        while let Some((ln,_)) = prev_lines.front() { if *ln <= last_printed { prev_lines.pop_front(); } else { break; } }
                                        for (ln,bytes) in prev_lines.iter() { if *ln > last_printed { if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", ln); } let _=stdout_handle.write_all(bytes); let _=stdout_handle.write_all(b"\n"); last_printed=*ln; } }
                                    }
                                    // match line
                                    if only_matching {
                                        if let Some(re)=&regex_opt {
                                            if let Ok(s)=std::str::from_utf8(line_slice) { for m in re.find_iter(s) { if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(m.as_str().as_bytes()); let _=stdout_handle.write_all(b"\n"); } }
                                        } else if whole_line {
                                            if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(line_slice); let _=stdout_handle.write_all(b"\n");
                                        } else if let Ok(s)=std::str::from_utf8(line_slice) {
                                            let hay = if ignore_case { s.to_lowercase() } else { s.to_string() };
                                            let mut pos=0; while let Some(off)=hay[pos..].find(&needle) { let start_o=pos+off; let end_o=start_o+needle.len(); if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(&line_slice[start_o..end_o]); let _=stdout_handle.write_all(b"\n"); pos=end_o; }
                                        }
                                    } else {
                                        if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(line_slice); let _=stdout_handle.write_all(b"\n");
                                    }
                                    last_printed=line_no; after_rem=ctx_after;
                                }
                            } else if after_rem>0 && !count_only && !quiet {
                                if line_no > last_printed { if with_filename { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(line_slice); let _=stdout_handle.write_all(b"\n"); last_printed=line_no; }
                                if after_rem>0 { after_rem-=1; }
                            }
                            // maintain before-context buffer
                            if ctx_before>0 { prev_lines.push_back((line_no, line_slice.to_vec())); if prev_lines.len()>ctx_before { let _=prev_lines.pop_front(); } }
                            start=i+1;
                        }
                        i+=1;
                    }
                    if start>0 { buf_acc.drain(0..start); }
                    if early_exit { break; }
                    if eof { break; }
                }
                Ok(Err(code)) => { eprintln!("read error {:?}", code); let _ = client.close(h); return map_exit(&code); },
                Err(e) => { eprintln!("protocol read error: {e}"); let _ = client.close(h); return 6; },
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

// ---- sed: basic engine with multi-command program, addresses, -n, p/d/a/i + s/// + hold space (G/g/H/h/x) + labels and t/T ----
#[derive(Clone)]
enum SedAddr { Line(usize), Last, Re(Regex) }

#[derive(Clone, Default)]
struct SedCond { a1: Option<SedAddr>, a2: Option<SedAddr>, neg: bool, in_range: bool }

#[derive(Clone)]
enum SedCmd {
    S { pat:String, repl:String, g:bool, re:bool, icase:bool },
    P, D, A(String), I(String),
    GUpper, GLower, HUpper, HLower, Xchg,
    Label(String),
    BranchIf { on_subst: bool, target: Option<String> }, // t (on_subst=true), T (on_subst=false)
}

#[derive(Clone)]
struct SedInstr { cond: SedCond, cmd: SedCmd }

fn parse_addr_component(s:&[char], mut i:usize) -> (Option<SedAddr>, usize) {
    if i>=s.len() { return (None,i); }
    match s[i] {
        '/' => { i+=1; let mut cur=String::new(); let mut esc=false; while i<s.len() { let c=s[i]; i+=1; if esc { cur.push(c); esc=false; continue; } if c=='\\' { esc=true; continue; } if c=='/' { break; } cur.push(c); } if cur.is_empty() { return (None,i); } let re=Regex::new(&cur).ok(); return (re.map(SedAddr::Re), i); }
        '$' => { return (Some(SedAddr::Last), i+1); }
        c if c.is_ascii_digit() => { let mut val:usize=0; while i<s.len() && s[i].is_ascii_digit() { val = val*10 + (s[i] as u8 - b'0') as usize; i+=1; } if val==0 { (None,i) } else { (Some(SedAddr::Line(val)), i) } }
        _ => (None,i)
    }
}

fn parse_s_cmd(s:&[char], mut i:usize) -> Option<(SedCmd, usize)> {
    if i>=s.len() || s[i] != 's' { return None; }
    i+=1; if i>=s.len() { return None; }
    let delim = s[i]; i+=1;
    let mut cur=String::new(); let mut parts:Vec<String>=Vec::new(); let mut esc=false;
    while i<s.len() {
        let c=s[i]; i+=1; if esc { cur.push(c); esc=false; continue; }
        if c=='\\' { esc=true; continue; }
        if c==delim { parts.push(cur.clone()); cur.clear(); if parts.len()==2 { break; } continue; }
        cur.push(c);
    }
    if parts.len()!=2 { return None; }
    let pattern = parts[0].replace(&format!("\\{}",delim), &delim.to_string());
    let repl = parts[1].replace(&format!("\\{}",delim), &delim.to_string());
    let mut flags=String::new(); while i<s.len() { flags.push(s[i]); i+=1; }
    let g = flags.contains('g');
    let re = flags.contains('r') || flags.contains('E');
    let icase = flags.contains('I');
    Some((SedCmd::S{ pat:pattern, repl, g, re, icase }, i))
}

fn parse_program(script:&str) -> Result<(Vec<SedInstr>, std::collections::HashMap<String, usize>), i32> {
    let chars:Vec<char>=script.chars().collect();
    let mut i=0usize;
    let mut prog:Vec<SedInstr>=Vec::new();
    use std::collections::HashMap; let mut labels:HashMap<String,usize>=HashMap::new();
    fn skip_ws(chars:&[char], mut i:usize)->usize{ while i<chars.len() && chars[i].is_whitespace(){ i+=1; } i }
    fn read_label(chars:&[char], mut i:usize)->(String,usize){ let mut s=String::new(); while i<chars.len(){ let c=chars[i]; if c.is_ascii_alphanumeric()||c=='_' { s.push(c); i+=1; } else { break; } } (s,i) }
    while i < chars.len() {
        i = skip_ws(&chars, i);
        // optional delimiter ';'
        if i<chars.len() && chars[i]==';' { i+=1; continue; }
        if i>=chars.len() { break; }
        // label
        if chars[i]==':' {
            i+=1; let (name, j2)=read_label(&chars, i); i=j2; if name.is_empty(){ return Err(2); }
            labels.insert(name, prog.len());
            continue;
        }
        // addresses
        let (a1,i1)=parse_addr_component(&chars, i); let mut a2=None; let mut j=i1; let mut neg=false;
        if a1.is_some() && j<chars.len() && chars[j]==',' { let (a2opt,j2)=parse_addr_component(&chars, j+1); a2=a2opt; j=j2; }
        if j<chars.len() && chars[j]=='!' { neg=true; j+=1; }
        if j>=chars.len() { break; }
        // command
        let (cmd, next_i) = if let Some((cmd, endp)) = parse_s_cmd(&chars, j) { (cmd, endp) } else {
            match chars[j] {
                'p' => (SedCmd::P, j+1),
                'd' => (SedCmd::D, j+1),
                'G' => (SedCmd::GUpper, j+1),
                'g' => (SedCmd::GLower, j+1),
                'H' => (SedCmd::HUpper, j+1),
                'h' => (SedCmd::HLower, j+1),
                'x' => (SedCmd::Xchg, j+1),
                'a' => { // read until next ';' or end
                    let mut k=j+1; k = skip_ws(&chars, k);
                    let start=k; while k<chars.len() && chars[k]!=';' { k+=1; }
                    let text: String = chars[start..k].iter().collect::<String>().trim_start().to_string();
                    (SedCmd::A(text), k)
                },
                'i' => {
                    let mut k=j+1; k = skip_ws(&chars, k);
                    let start=k; while k<chars.len() && chars[k]!=';' { k+=1; }
                    let text: String = chars[start..k].iter().collect::<String>().trim_start().to_string();
                    (SedCmd::I(text), k)
                },
                't' | 'T' => {
                    let on_subst = chars[j]=='t';
                    let mut k=j+1; k = skip_ws(&chars, k);
                    if k<chars.len() && chars[k]!=';' {
                        let (name, k2)=read_label(&chars, k);
                        (SedCmd::BranchIf{ on_subst, target: if name.is_empty(){ None } else { Some(name) } }, k2)
                    } else { (SedCmd::BranchIf{ on_subst, target: None }, k) }
                },
                _ => return Err(2),
            }
        };
        prog.push(SedInstr{ cond: SedCond{ a1, a2, neg, in_range:false }, cmd });
        i = next_i;
    }
    if prog.is_empty(){ return Err(2); }
    Ok((prog, labels))
}

fn match_addr(addr:&SedAddr, line:&str, n:usize, is_last:bool)->bool{ match addr { SedAddr::Line(k)=> *k==n, SedAddr::Last=> is_last, SedAddr::Re(r)=> r.is_match(line) } }

fn cond_applies(cond:&mut SedCond, line:&str, n:usize, is_last:bool)->bool{
    let single = cond.a1.is_some() && cond.a2.is_none();
    if single {
        let m = cond.a1.as_ref().map(|a| match_addr(a, line, n, is_last)).unwrap_or(true);
        return if cond.neg { !m } else { m };
    }
    if cond.a1.is_none() && cond.a2.is_none() { return if cond.neg { false } else { true }; }
    // range
    if !cond.in_range {
        if let Some(a1)=&cond.a1 { if match_addr(a1, line, n, is_last) { cond.in_range=true; } }
    }
    let mut applies = cond.in_range;
    if cond.in_range { if let Some(a2)=&cond.a2 { if match_addr(a2, line, n, is_last) { applies = true; cond.in_range=false; } } }
    if cond.neg { !applies } else { applies }
}

fn run_builtin_sed(args:&Vec<String>, redir:&RedirSpec, _allow_read:&[String], _allow_write:&[String], _pass_fds:Option<(i32,i32)>) -> i32 {
    if args.is_empty() { eprintln!("sed: missing script"); return 2; }
    // flags (support only -n)
    let mut idx=0; let mut auto_print=true;
    while idx<args.len() && args[idx].starts_with('-') {
        if args[idx]=="-n" { auto_print=false; idx+=1; } else { break; }
    }
    if idx>=args.len() { eprintln!("sed: missing script"); return 2; }
    let script = &args[idx]; idx+=1;
    let mut files:Vec<String> = if idx<args.len() { args[idx..].to_vec() } else { Vec::new() };
    if files.is_empty() { if let Some(f)=&redir.in_file { files.push(f.clone()); } }
    if files.is_empty() { return 0; }

    let (mut program, labels) = match parse_program(script) {
        Ok(v)=>v,
        Err(_)=> { eprintln!("sed: unsupported or invalid script"); return 2; }
    };

    let client = child_mux();
    let mut out_handle: Option<u32> = None;
    if let Some((path, append))=&redir.out_file {
        let mode = if *append { "a" } else { "w" };
        match client.open(path, mode) {
            Ok(Ok(h))=> out_handle=Some(h),
            Ok(Err(code))=> { eprintln!("open {path} {mode} error: {:?}", code); return map_exit(&code); },
            Err(e)=> { eprintln!("protocol open {path} {mode} error: {e}"); return 6; }
        }
    }
    let mut stdout_handle = std::io::stdout();

    let mut hold:String = String::new();
    for p in files.iter() {
        let h = match client.open(p, "r") {
            Ok(Ok(h))=>h,
            Ok(Err(code))=>{ eprintln!("open {p} r error: {:?}", code); return map_exit(&code); },
            Err(e)=>{ eprintln!("protocol open {p} r error: {e}"); return 6; }
        };
        let mut buf_acc:Vec<u8>=Vec::new();
        let mut line_no:usize=0;
        loop {
            match client.read_chunk(h, 4096) {
                Ok(Ok((data,eof))) => {
                    if !data.is_empty() { buf_acc.extend_from_slice(&data); }
                    // process complete lines
                    let mut start = 0usize;
                    while let Some(pos) = buf_acc[start..].iter().position(|&b| b==b'\n') {
                        let end = start + pos;
                        let line_bytes = &buf_acc[start..end];
                        line_no += 1;
                        let mut line = String::from_utf8_lossy(line_bytes).to_string();
                        let mut deleted=false; let mut printed=false; let mut insert_txt:Option<String>=None; let mut append_txt:Option<String>=None; let mut last_subst=false;
                        // execute program
                        let mut ip:usize=0;
                        while ip < program.len() {
                            let instr = &mut program[ip];
                            let applies = cond_applies(&mut instr.cond, &line, line_no, false);
                            if applies {
                                match &instr.cmd {
                                    SedCmd::P => { printed=true; }
                                    SedCmd::D => { deleted=true; break; }
                                    SedCmd::A(t) => { append_txt=Some(t.clone()); }
                                    SedCmd::I(t) => { insert_txt=Some(t.clone()); }
                                    SedCmd::GUpper => { line.push('\n'); line.push_str(&hold); }
                                    SedCmd::GLower => { line = hold.clone(); }
                                    SedCmd::HUpper => { if hold.is_empty() { hold = line.clone(); } else { hold.push('\n'); hold.push_str(&line); } }
                                    SedCmd::HLower => { hold = line.clone(); }
                                    SedCmd::Xchg => { std::mem::swap(&mut line, &mut hold); }
                                    SedCmd::Label(_name) => { /* no-op */ }
                                    SedCmd::BranchIf{ on_subst, target } => {
                                        let take = if *on_subst { last_subst } else { !last_subst };
                                        last_subst=false; // reset per sed semantics
                                        if take {
                                            if let Some(tg)=target {
                                                if let Some(pos)=labels.get(tg) { ip=*pos; continue; } else { /* unknown label: treat as no target -> end */ break; }
                                            } else { break; }
                                        }
                                    }
                                    SedCmd::S{ pat, repl, g, re, icase } => {
                                        if !pat.is_empty() {
                                            if *re {
                                                let pat_src = if *icase { format!("(?i){}", pat) } else { pat.clone() };
                                                let pat_re = match Regex::new(&pat_src) { Ok(r)=>r, Err(e)=> { eprintln!("sed: invalid regex: {e}"); return 2; } };
                                                if *g {
                                                    let out = pat_re.replace_all(&line, |caps: &regex::Captures| {
                                                        let mut out=String::new(); let mut cs=repl.chars().peekable();
                                                        while let Some(c)=cs.next(){
                                                            if c=='&' { out.push_str(caps.get(0).map(|m| m.as_str()).unwrap_or("")); }
                                                            else if c=='\\' {
                                                                if let Some(n)=cs.peek().cloned(){
                                                                    if n.is_ascii_digit(){ let _=cs.next(); let idx=(n as u8 - b'0') as usize; out.push_str(caps.get(idx).map(|m| m.as_str()).unwrap_or("")); }
                                                                    else { out.push(n); let _=cs.next(); }
                                                                }
                                                            } else { out.push(c); }
                                                        }
                                                        out
                                                    }).to_string();
                                                    if out != line { last_subst = true; }
                                                    line = out;
                                                } else {
                                                    let out = pat_re.replace(&line, |caps: &regex::Captures| {
                                                        let mut out=String::new(); let mut cs=repl.chars().peekable();
                                                        while let Some(c)=cs.next(){
                                                            if c=='&' { out.push_str(caps.get(0).map(|m| m.as_str()).unwrap_or("")); }
                                                            else if c=='\\' {
                                                                if let Some(n)=cs.peek().cloned(){
                                                                    if n.is_ascii_digit(){ let _=cs.next(); let idx=(n as u8 - b'0') as usize; out.push_str(caps.get(idx).map(|m| m.as_str()).unwrap_or("")); }
                                                                    else { out.push(n); let _=cs.next(); }
                                                                }
                                                            } else { out.push(c); }
                                                        }
                                                        out
                                                    }).to_string();
                                                    if out != line { last_subst = true; }
                                                    line = out;
                                                }
                                            } else {
                                                if *g {
                                                    let mut out_line=String::new(); let mut pos2=0usize; let mut did=false;
                                                    while let Some(found)=line[pos2..].find(pat) {
                                                        out_line.push_str(&line[pos2..pos2+found]);
                                                        out_line.push_str(repl);
                                                        pos2 += found + pat.len(); did=true;
                                                    }
                                                    out_line.push_str(&line[pos2..]);
                                                    if did { last_subst=true; }
                                                    line=out_line;
                                                } else if let Some(found)=line.find(pat) {
                                                    line = format!("{}{}{}", &line[..found], repl, &line[found+pat.len()..]); last_subst=true;
                                                }
                                            }
                                        }
                                    }
                            }
                            ip+=1;
                        }
                        }
                            if !deleted {
                            if let Some(t)=insert_txt { let mut b=t.into_bytes(); b.push(b'\n'); if let Some(hout)=out_handle { let _=client.write_chunk(hout, &b); } else { let _=stdout_handle.write_all(&b); } }
                            if auto_print || printed { let mut b=line.into_bytes(); b.push(b'\n'); if let Some(hout)=out_handle { let _=client.write_chunk(hout, &b); } else { let _=stdout_handle.write_all(&b); } }
                            if let Some(t)=append_txt { let mut b=t.into_bytes(); b.push(b'\n'); if let Some(hout)=out_handle { let _=client.write_chunk(hout, &b); } else { let _=stdout_handle.write_all(&b); } }
                        }
                        start = end + 1;
                    }
                    if start>0 { buf_acc.drain(0..start); }

                    if eof {
                        if !buf_acc.is_empty() {
                            let mut line = String::from_utf8_lossy(&buf_acc).to_string();
                            line_no += 1;
                            let mut deleted=false; let mut printed=false; let mut insert_txt:Option<String>=None; let mut append_txt:Option<String>=None; let mut last_subst=false;
                            let mut ip:usize=0;
                            while ip < program.len() {
                                let instr = &mut program[ip];
                                let applies = cond_applies(&mut instr.cond, &line, line_no, true);
                                if applies {
                                    match &instr.cmd {
                                        SedCmd::P => { printed=true; }
                                        SedCmd::D => { deleted=true; break; }
                                        SedCmd::A(t)=>{ append_txt=Some(t.clone()); }
                                        SedCmd::I(t)=>{ insert_txt=Some(t.clone()); }
                                        SedCmd::GUpper => { line.push('\n'); line.push_str(&hold); }
                                        SedCmd::GLower => { line = hold.clone(); }
                                        SedCmd::HUpper => { if hold.is_empty() { hold = line.clone(); } else { hold.push('\n'); hold.push_str(&line); } }
                                        SedCmd::HLower => { hold = line.clone(); }
                                        SedCmd::Xchg => { std::mem::swap(&mut line, &mut hold); }
                                        SedCmd::Label(_)=>{},
                                        SedCmd::BranchIf{ on_subst, target }=>{
                                            let take = if *on_subst { last_subst } else { !last_subst }; last_subst=false; if take { if let Some(tg)=target { if let Some(pos)=labels.get(tg){ ip=*pos; continue; } else { break; } } else { break; } }
                                        }
                                        SedCmd::S{ pat, repl, g, re, icase }=>{
                                            if !pat.is_empty(){
                                                if *re {
                                                    let pat_src = if *icase { format!("(?i){}", pat) } else { pat.clone() };
                                                    let pat_re = match Regex::new(&pat_src) { Ok(r)=>r, Err(e)=> { eprintln!("sed: invalid regex: {e}"); return 2; } };
                                                    if *g {
                                                        let out = pat_re.replace_all(&line, |caps: &regex::Captures| {
                                                            let mut out=String::new(); let mut cs=repl.chars().peekable();
                                                            while let Some(c)=cs.next(){
                                                                if c=='&' { out.push_str(caps.get(0).map(|m| m.as_str()).unwrap_or("")); }
                                                                else if c=='\\' {
                                                                    if let Some(n)=cs.peek().cloned(){
                                                                        if n.is_ascii_digit(){ let _=cs.next(); let idx=(n as u8 - b'0') as usize; out.push_str(caps.get(idx).map(|m| m.as_str()).unwrap_or("")); }
                                                                        else { out.push(n); let _=cs.next(); }
                                                                    }
                                                                } else { out.push(c); }
                                                            }
                                                            out
                                                        }).to_string();
                                                        if out != line { last_subst=true; }
                                                        line = out;
                                                    } else {
                                                        let out = pat_re.replace(&line, |caps: &regex::Captures| {
                                                            let mut out=String::new(); let mut cs=repl.chars().peekable();
                                                            while let Some(c)=cs.next(){
                                                                if c=='&' { out.push_str(caps.get(0).map(|m| m.as_str()).unwrap_or("")); }
                                                                else if c=='\\' {
                                                                    if let Some(n)=cs.peek().cloned(){
                                                                        if n.is_ascii_digit(){ let _=cs.next(); let idx=(n as u8 - b'0') as usize; out.push_str(caps.get(idx).map(|m| m.as_str()).unwrap_or("")); }
                                                                        else { out.push(n); let _=cs.next(); }
                                                                    }
                                                                } else { out.push(c); }
                                                            }
                                                            out
                                                        }).to_string();
                                                        if out != line { last_subst=true; }
                                                        line = out;
                                                    }
                                                } else {
                                                    if *g {
                                                        let mut out_line=String::new(); let mut pos2=0usize; let mut did=false;
                                                        while let Some(found)=line[pos2..].find(pat) {
                                                            out_line.push_str(&line[pos2..pos2+found]);
                                                            out_line.push_str(repl);
                                                            pos2 += found + pat.len(); did=true;
                                                        }
                                                        out_line.push_str(&line[pos2..]);
                                                        if did { last_subst=true; }
                                                        line=out_line;
                                                    } else if let Some(found)=line.find(pat) {
                                                        line = format!("{}{}{}", &line[..found], repl, &line[found+pat.len()..]); last_subst=true;
                                                    }
                                                }
                                            }
                                        }
                                    }
                                }
                                ip+=1;
                            }
                            if !deleted {
                                if let Some(t)=insert_txt { let mut b=t.into_bytes(); if !b.ends_with(&[b'\n']) { b.push(b'\n'); } if let Some(hout)=out_handle { let _=client.write_chunk(hout, &b); } else { let _=stdout_handle.write_all(&b); } }
                                if auto_print || printed { let b=line.into_bytes(); if let Some(hout)=out_handle { let _=client.write_chunk(hout, &b); } else { let _=stdout_handle.write_all(&b); } }
                                if let Some(t)=append_txt { let mut b=t.into_bytes(); if !b.ends_with(&[b'\n']) { b.push(b'\n'); } if let Some(hout)=out_handle { let _=client.write_chunk(hout, &b); } else { let _=stdout_handle.write_all(&b); } }
                            }
                            buf_acc.clear();
                        }
                        break;
                    }
                }
                Ok(Err(code)) => { eprintln!("read error: {:?}", code); let _ = client.close(h); return map_exit(&code); }
                Err(e) => { eprintln!("protocol read error: {e}"); let _ = client.close(h); return 6; }
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
    HelpEntry { name:"grep", usage:"grep [-i] [-n] [-E] [-v] [-c] [-x] [-w] [-o] [-H|-h] [-q] [-A N] [-B N] [-C N] pattern [file...]", desc:"search lines matching pattern (literal by default; -E enables regex)", options:&[("-i","ignore case"),("-n","show line numbers"),("-E","regex pattern (alias -r)"),("-v","invert match"),("-c","count matches per file"),("-x","match whole line"),("-w","match word"),("-o","only matching (each match on its own line)"),("-H","force show filename"),("-h","suppress filename"),("-q","quiet (exit on first match, no output)"),("-A N","print N lines of trailing context"),("-B N","print N lines of leading context"),("-C N","print N lines of output context")] , examples:&[("grep foo file","Find foo"),("grep -i -E 'foo|bar' file","Regex OR match"),("cat f | grep -n bar","Pipe with line numbers"),("grep -c pattern a b","Count per file"),("grep -w err log","Word match"),("grep -o -n 'er+'/E log","Only matches with line numbers"),("grep -C 2 pat file","With context")], related:&["sed","wc"] },
    HelpEntry { name:"tr", usage:"tr [-d] [-c] [-s] set1 [set2]", desc:"translate or delete characters with ranges/classes; -s squeeze repeats; -c complement for -d/-s", options:&[("-d","delete chars in set1"),("-c","complement set1 (with -d/-s)"),("-s","squeeze repeats of set1/translated set")], examples:&[("tr abc xyz < in","Map a->x b->y c->z"),("tr 'a-z' 'A-Z' < in","Uppercase"),("tr -d '[:digit:]' < in","Delete digits"),("tr -s ' ' < in","Squeeze spaces")], related:&["sed"] },
    HelpEntry { name:"sed", usage:"sed [-n] [[addr1][,[addr2]]][!] {p|d|a text|i text|G|g|H|h|x|s/pat/repl/[gEI]|:label|t [label]|T [label]} [file...]", desc:"stream editor subset: addresses (N, $, /re/), ranges, negation (!); commands p,d,a,i,G,g,H,h,x,s///, labels (:label), and branches t/T. s supports regex (E|r), ignore-case (I), global (g), and &/\\1..\\9 backrefs. t jumps if last s/// substituted; T if not. Label target optional (no label ends the program).", options:&[("-n","suppress auto-print"),("g","global replace for s///"),("E","enable regex for s/// (alias r)"),("I","ignore case with regex")], examples:&[("sed -n '/error/p' file","Print lines matching 'error'"),("sed '2,5d' file","Delete lines 2..5"),("sed '3a appended' file","Append after line 3"),("sed '1i header' file","Insert before line 1"),("sed 'G' file","Append hold space to each pattern"),("sed 'h' file","Copy pattern to hold space"),("sed 'x' file","Swap pattern and hold"),("sed ':a; s/aa/a/; t a' file","Loop while substitutions occur"),("sed 's/foo/bar/; T skip; s/baz/BAZ/; :skip' file","Branch on no-substitution"),("sed 's/foo/bar/g' file","Replace all foo with bar")], related:&["grep","tr"] },
    HelpEntry { name:"help", usage:"help [command]", desc:"list commands or show detailed help", options:&[], examples:&[("help","List commands"),("help grep","Show grep help")], related:&[] },
    HelpEntry { name:"exit", usage:"exit [N]", desc:"exit the current shell or pipeline with status N (default: last command status or 0)", options:&[], examples:&[("exit","Exit with last status"),("exit 2","Exit with status 2")], related:&[] },
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
