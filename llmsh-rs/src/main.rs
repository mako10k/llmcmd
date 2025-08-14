use anyhow::Result;
use std::env;
use std::os::unix::io::RawFd;
use std::os::fd::IntoRawFd; // needed for into_raw_fd on OwnedFd
use std::ffi::CString;
use std::io::{BufReader, Read, Write};
use std::process::{Command, Stdio};
use nix::unistd::{fork, ForkResult, pipe, dup2, close, execvp};
use nix::libc;
use serde_json::json;
use base64::{engine::general_purpose, Engine as _};
use regex::Regex;

// -------- Builtins / Parsing ---------
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum BuiltinKind { Cat, Head, Tail, Wc, Grep, Tr, Sed, Help }

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
    if ["cat","head","tail","wc","grep","tr","sed","help"].contains(&cmd.as_str()) {
        if !args.is_empty() && cmd == "cat" { redir.in_file = None; }
        let kind = match cmd.as_str() { "cat"=>BuiltinKind::Cat, "head"=>BuiltinKind::Head, "tail"=>BuiltinKind::Tail, "wc"=>BuiltinKind::Wc, "grep"=>BuiltinKind::Grep, "tr"=>BuiltinKind::Tr, "sed"=>BuiltinKind::Sed, "help"=>BuiltinKind::Help, _=>BuiltinKind::Cat };
        return Some(ExecUnit::Builtin { kind, args, redir });
    }
    Some(ExecUnit::External { argv: out, redir })
}

fn parse_pipeline(input:&str)->Vec<ExecUnit>{ input.split('|').filter_map(|s| parse_segment(s.trim())).collect() }

// -------- VFS Client --------
struct VfsClient { stdin: std::process::ChildStdin, stdout: BufReader<std::process::ChildStdout>, seq: u64 }

#[derive(Debug)]
enum VfsErrorCode { EArg, ENoEnt, EPerm, EIO, EClosed, EUnsupported, Other(String) }

impl VfsClient {
    fn spawn(vfsd_path:&str, allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> Result<Self> {
        let mut cmd = Command::new(vfsd_path);
        for p in allow_read { cmd.arg("-i").arg(p); }
        for p in allow_write { cmd.arg("-o").arg(p); }
        if let Some(fd) = pass_fd { cmd.arg("--vfs-fd").arg(fd.to_string()); // close our copy after spawn below
        }
        let mut child = cmd.stdin(Stdio::piped()).stdout(Stdio::piped()).stderr(Stdio::inherit()).spawn()?;
        if let Some(fd)=pass_fd { let _ = nix::unistd::close(fd); }
        let stdin = child.stdin.take().expect("child stdin");
        let stdout = child.stdout.take().expect("child stdout");
        Ok(VfsClient { stdin, stdout: BufReader::new(stdout), seq:0 })
    }
    fn next_id(&mut self)->String { self.seq +=1; self.seq.to_string() }
    fn send(&mut self, op:&str, params: serde_json::Value) -> Result<(bool, serde_json::Value)> {
        let id = self.next_id();
        let obj = json!({"id": id, "op": op, "params": params});
        let data = serde_json::to_vec(&obj)?;
        let len = (data.len() as u32).to_be_bytes();
        self.stdin.write_all(&len)?; self.stdin.write_all(&data)?; self.stdin.flush()?;
        let mut len_buf=[0u8;4];
        self.stdout.read_exact(&mut len_buf)?;
        let resp_len = u32::from_be_bytes(len_buf) as usize;
        let mut buf=vec![0u8;resp_len];
        self.stdout.read_exact(&mut buf)?;
        let v: serde_json::Value = serde_json::from_slice(&buf)?;
        let ok = v.get("ok").and_then(|b| b.as_bool()).unwrap_or(false);
        if !ok { let code = v.get("error").and_then(|e| e.get("code").and_then(|c| c.as_str())).unwrap_or("?").to_string(); return Ok((false, json!({"code": code}))); }
        Ok((true, v.get("result").cloned().unwrap_or(json!({}))))
    }
    fn map_code(c:&str)->VfsErrorCode { match c {"E_ARG"=>VfsErrorCode::EArg,"E_NOENT"=>VfsErrorCode::ENoEnt,"E_PERM"=>VfsErrorCode::EPerm,"E_IO"=>VfsErrorCode::EIO,"E_CLOSED"=>VfsErrorCode::EClosed,"E_UNSUPPORTED"=>VfsErrorCode::EUnsupported, other=>VfsErrorCode::Other(other.to_string())} }
    fn open_read(&mut self,path:&str)->Result<Result<u32,VfsErrorCode>>{ let (ok,res)=self.send("open_read", json!({"path":path}))?; if ok { Ok(Ok(res.get("handle").and_then(|h|h.as_u64()).unwrap_or(0) as u32)) } else { Ok(Err(Self::map_code(res.get("code").and_then(|c|c.as_str()).unwrap_or("?")))) } }
    fn open_write(&mut self,path:&str,append:bool)->Result<Result<u32,VfsErrorCode>>{ let (ok,res)=self.send("open_write", json!({"path":path,"append":append}))?; if ok { Ok(Ok(res.get("handle").and_then(|h|h.as_u64()).unwrap_or(0) as u32)) } else { Ok(Err(Self::map_code(res.get("code").and_then(|c|c.as_str()).unwrap_or("?")))) } }
    fn read_chunk(&mut self,h:u32,max:u32)->Result<Result<(Vec<u8>,bool),VfsErrorCode>>{ let (ok,res)=self.send("read", json!({"h":h,"max":max}))?; if ok { let b64=res.get("data").and_then(|d|d.as_str()).unwrap_or(""); let mut data=Vec::new(); if !b64.is_empty(){ data=general_purpose::STANDARD.decode(b64).unwrap_or_default(); } let eof=res.get("eof").and_then(|e|e.as_bool()).unwrap_or(false); Ok(Ok((data,eof))) } else { Ok(Err(Self::map_code(res.get("code").and_then(|c|c.as_str()).unwrap_or("?")))) } }
    fn write_chunk(&mut self,h:u32,data:&[u8])->Result<Result<usize,VfsErrorCode>>{ let b64=general_purpose::STANDARD.encode(data); let (ok,res)=self.send("write", json!({"h":h,"data":b64}))?; if ok { Ok(Ok(res.get("written").and_then(|n|n.as_u64()).unwrap_or(0) as usize)) } else { Ok(Err(Self::map_code(res.get("code").and_then(|c|c.as_str()).unwrap_or("?")))) } }
    fn close(&mut self,h:u32)->Result<Result<(),VfsErrorCode>>{ let (ok,_)=self.send("close", json!({"h":h}))?; if ok { Ok(Ok(())) } else { Ok(Err(VfsErrorCode::EClosed)) } }
}

fn spawn_pipeline(units:&[ExecUnit], allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> Result<i32> {
    if units.is_empty() { return Ok(0); }
    let mut fds: Vec<(RawFd, RawFd)> = Vec::new();
    for _ in 0..units.len().saturating_sub(1) {
        let (r,w)=pipe()?;
        let rfd = r.into_raw_fd();
        let wfd = w.into_raw_fd();
        fds.push((rfd,wfd));
    }
    for (i, unit) in units.iter().enumerate() {
        match unsafe { fork()? } {
            ForkResult::Child => {
                if i > 0 { let (pr,_pw)=fds[i-1]; dup2(pr,0)?; }
                if i < units.len()-1 { let (_pr,pw)=fds[i]; dup2(pw,1)?; }
                for (r,w) in &fds { let _=close(*r); let _=close(*w); }
                match unit {
            ExecUnit::Builtin { kind, args, redir } => { 
                        let code = match kind {
                BuiltinKind::Cat => run_builtin_cat(args, redir, allow_read, allow_write, pass_fd),
                BuiltinKind::Head => run_builtin_head(args, redir, allow_read, allow_write, pass_fd),
                BuiltinKind::Tail => run_builtin_tail(args, redir, allow_read, allow_write, pass_fd),
                BuiltinKind::Wc => run_builtin_wc(args, redir, allow_read, allow_write, pass_fd),
                BuiltinKind::Grep => run_builtin_grep(args, redir, allow_read, allow_write, pass_fd),
                BuiltinKind::Tr => run_builtin_tr(args, redir, allow_read, allow_write, pass_fd),
                BuiltinKind::Sed => run_builtin_sed(args, redir, allow_read, allow_write, pass_fd),
                            BuiltinKind::Help => run_builtin_help(args),
                        }; 
                        std::process::exit(code); 
                    }
                    ExecUnit::External { argv, .. } => {
                        let cstrs: Vec<CString> = argv.iter().map(|s| CString::new(s.as_str()).unwrap()).collect();
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
    let mut status_code = 0;
    for _ in 0..units.len() { let mut status: i32 = 0; unsafe { libc::wait(&mut status); if libc::WIFEXITED(status){ status_code = libc::WEXITSTATUS(status);} } }
    Ok(status_code)
}

fn main() -> Result<()> {
    let mut args = env::args().skip(1);
    let mut script: Option<String> = None;
    let mut allow_read: Vec<String> = Vec::new();
    let mut allow_write: Vec<String> = Vec::new();
    let mut pass_fd: Option<i32> = None;
    while let Some(a) = args.next() {
        match a.as_str() {
            "-c" => { script = Some(args.next().ok_or_else(|| anyhow::anyhow!("missing script after -c"))?); }
            "-i" | "--input" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("missing value after -i"))?; allow_read.push(v); }
            "-o" | "--output" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("missing value after -o"))?; allow_write.push(v); }
        "--vfs-fd" => { let v = args.next().ok_or_else(|| anyhow::anyhow!("missing value after --vfs-fd"))?; let fd: i32 = v.parse().map_err(|_| anyhow::anyhow!("invalid fd"))?; pass_fd = Some(fd); }
            other => { eprintln!("unknown arg: {other}"); }
        }
    }
    let s = script.unwrap_or_else(|| "".to_string());
    let units = parse_pipeline(&s);
    let code = spawn_pipeline(&units, &allow_read, &allow_write, pass_fd)?;
    std::process::exit(code);
}

fn run_builtin_cat(args: &Vec<String>, redir: &RedirSpec, allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> i32 {
    let inputs: Vec<String> = if !args.is_empty() { args.clone() } else if let Some(f)= &redir.in_file { vec![f.clone()] } else { Vec::new() };
    if inputs.is_empty() { return 0; }
    let vfsd_path = env::var("LLMSH_VFSD_BIN").unwrap_or_else(|_| "vfsd/target/debug/vfsd".to_string());
    let mut client = match VfsClient::spawn(&vfsd_path, allow_read, allow_write, pass_fd) { Ok(c)=>c, Err(e)=> { eprintln!("vfsd spawn failed: {e}"); return 5; } };
    let mut out_handle: Option<u32> = None;
    if let Some((path, append)) = &redir.out_file { match client.open_write(path, *append) { Ok(Ok(h))=> out_handle=Some(h), Ok(Err(code))=> { eprintln!("open_write error: {:?}", code); return map_exit(&code); }, Err(e)=> { eprintln!("protocol open_write error: {e}"); return 6; } } }
    let mut stdout_handle = std::io::stdout();
    for p in inputs.iter() {
        let h = match client.open_read(p) { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open_read {} error: {:?}", p, code); return map_exit(&code); }, Err(e)=> { eprintln!("protocol open_read {p} error: {e}"); return 6; } };
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

fn run_builtin_head(args:&Vec<String>, redir:&RedirSpec, allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> i32 {
    let (limit, mut files) = parse_head_tail_args(BuiltinKind::Head, args);
    if files.is_empty() {
        if let Some(f)=&redir.in_file { files.push(f.clone()); } else { return 0; }
    }
    let vfsd_path = env::var("LLMSH_VFSD_BIN").unwrap_or_else(|_| "vfsd/target/debug/vfsd".to_string());
    let mut client = match VfsClient::spawn(&vfsd_path, allow_read, allow_write, pass_fd) { Ok(c)=>c, Err(e)=> { eprintln!("vfsd spawn failed: {e}"); return 5; } };
    let mut stdout_handle = std::io::stdout();
    let mut remaining = limit;
    for p in files.iter() {
        if remaining == 0 { break; }
        let h = match client.open_read(p) { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open_read {} error: {:?}", p, code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open_read {p} error: {e}"); return 6; } };
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

fn run_builtin_tail(args:&Vec<String>, redir:&RedirSpec, allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> i32 {
    let (limit, mut files) = parse_head_tail_args(BuiltinKind::Tail, args);
    if files.is_empty() { if let Some(f)=&redir.in_file { files.push(f.clone()); } else { return 0; } }
    let vfsd_path = env::var("LLMSH_VFSD_BIN").unwrap_or_else(|_| "vfsd/target/debug/vfsd".to_string());
    let mut client = match VfsClient::spawn(&vfsd_path, allow_read, allow_write, pass_fd) { Ok(c)=>c, Err(e)=> { eprintln!("vfsd spawn failed: {e}"); return 5; } };
    let mut stdout_handle = std::io::stdout();
    for (fi,p) in files.iter().enumerate() {
        let h = match client.open_read(p) { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open_read {} error: {:?}", p, code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open_read {p} error: {e}"); return 6; } };
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
fn run_builtin_wc(args:&Vec<String>, redir:&RedirSpec, allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> i32 {
    // Options (subset): support -l (lines), -w (words), -c (bytes), default all three like POSIX.
    let mut show_l=true; let mut show_w=true; let mut show_b=true;
    let mut files:Vec<String>=Vec::new();
    for a in args.iter() {
        if a.starts_with('-') && a.len()>1 { for ch in a.chars().skip(1) { match ch { 'l'=>{show_w=false;show_b=false;}, 'w'=>{show_l=false;show_b=false;}, 'c'=>{show_l=false;show_w=false;}, _=>{} } } } else { files.push(a.clone()); }
    }
    if files.is_empty() { if let Some(f)=&redir.in_file { files.push(f.clone()); } }
    if files.is_empty() { return 0; }
    let vfsd_path = env::var("LLMSH_VFSD_BIN").unwrap_or_else(|_| "vfsd/target/debug/vfsd".to_string());
    let mut client = match VfsClient::spawn(&vfsd_path, allow_read, allow_write, pass_fd) { Ok(c)=>c, Err(e)=> { eprintln!("vfsd spawn failed: {e}"); return 5; } };
    let mut stdout_handle = std::io::stdout();
    let mut total_l=0usize; let mut total_w=0usize; let mut total_b=0usize;
    let multi = files.len()>1;
    for p in files.iter() {
        let h = match client.open_read(p) { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open_read {} error: {:?}", p, code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open_read {p} error: {e}"); return 6; } };
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

// ---- grep (simple substring match, no regex yet) ----
fn run_builtin_grep(args:&Vec<String>, redir:&RedirSpec, allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> i32 {
    if args.is_empty() { eprintln!("grep: missing pattern"); return 2; }
    // parse options: -i (ignore case), -n (line numbers), -E (regex) default literal, -r alias for -E
    let mut idx=0; let mut ignore_case=false; let mut show_line=false; let mut use_regex=false;
    while idx < args.len() && args[idx].starts_with('-') && args[idx] != "-" { for ch in args[idx].chars().skip(1) { match ch { 'i'=>ignore_case=true, 'n'=>show_line=true, 'E'|'r'=>use_regex=true, _=>{} } } idx+=1; }
    if idx>=args.len() { eprintln!("grep: missing pattern"); return 2; }
    let pattern_raw=&args[idx]; idx+=1;
    let mut files:Vec<String> = if idx < args.len() { args[idx..].to_vec() } else { Vec::new() };
    if files.is_empty() { if let Some(f)=&redir.in_file { files.push(f.clone()); } }
    if files.is_empty() { return 0; }
    let regex_opt = if use_regex { let pat = if ignore_case { format!("(?i){}", pattern_raw) } else { pattern_raw.to_string() }; match Regex::new(&pat) { Ok(r)=>Some(r), Err(e)=>{ eprintln!("grep: invalid regex: {e}"); return 2; } } } else { None };
    let needle = if !use_regex { if ignore_case { pattern_raw.to_lowercase() } else { pattern_raw.clone() } } else { String::new() };
    let vfsd_path = env::var("LLMSH_VFSD_BIN").unwrap_or_else(|_| "vfsd/target/debug/vfsd".to_string());
    let mut client = match VfsClient::spawn(&vfsd_path, allow_read, allow_write, pass_fd) { Ok(c)=>c, Err(e)=> { eprintln!("vfsd spawn failed: {e}"); return 5; } };
    let mut stdout_handle = std::io::stdout();
    let multi = files.len()>1;
    let mut any_match=false;
    for p in files.iter() {
        let h = match client.open_read(p) { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open_read {} error: {:?}", p, code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open_read {p} error: {e}"); return 6; } };
        let mut buf_acc:Vec<u8>=Vec::new();
        let mut line_no:usize=0;
        loop { match client.read_chunk(h, 4096) { Ok(Ok((data,eof)))=>{ if data.is_empty() && eof { break; } buf_acc.extend_from_slice(&data);
                let mut start=0; let mut i=0; while i < buf_acc.len() { if buf_acc[i]==b'\n' { line_no+=1; let line_slice=&buf_acc[start..i]; let mut is_match=false; if use_regex { if let Some(re)=&regex_opt { if let Ok(s)=std::str::from_utf8(line_slice) { if re.is_match(s) { is_match=true; } } } } else { if let Ok(s)=std::str::from_utf8(line_slice) { let s_cmp = if ignore_case { s.to_lowercase() } else { s.to_string() }; if !needle.is_empty() && s_cmp.contains(&needle) { is_match=true; } } }
                        if is_match { any_match=true; if multi { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(line_slice); let _=stdout_handle.write_all(b"\n"); }
                        start=i+1; }
                    i+=1; }
                if start>0 { buf_acc.drain(0..start); }
                if eof { if !buf_acc.is_empty() { line_no+=1; let line_slice=&buf_acc[..]; let mut is_match=false; if use_regex { if let Some(re)=&regex_opt { if let Ok(s)=std::str::from_utf8(line_slice) { if re.is_match(s) { is_match=true; } } } } else { if let Ok(s)=std::str::from_utf8(line_slice) { let s_cmp = if ignore_case { s.to_lowercase() } else { s.to_string() }; if !needle.is_empty() && s_cmp.contains(&needle) { is_match=true; } } } if is_match { any_match=true; if multi { let _=write!(stdout_handle, "{}:", p); } if show_line { let _=write!(stdout_handle, "{}:", line_no); } let _=stdout_handle.write_all(line_slice); let _=stdout_handle.write_all(b"\n"); } buf_acc.clear(); }
                    break; }
            }, Ok(Err(code))=>{ eprintln!("read error {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol read error: {e}"); return 6; } } }
        let _ = client.close(h);
    }
    let _ = stdout_handle.flush();
    if any_match { 0 } else { 1 }
}

// ---- tr (simple 1:1 mapping, optional -d delete) ----
fn run_builtin_tr(args:&Vec<String>, redir:&RedirSpec, allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> i32 {
    if args.is_empty() { eprintln!("tr: missing args"); return 2; }
    let mut delete_mode=false; let mut sets:Vec<String>=Vec::new();
    for a in args { if a=="-d" { delete_mode=true; } else { sets.push(a.clone()); } }
    if delete_mode {
        if sets.len()!=1 { eprintln!("tr -d requires 1 set"); return 2; }
    } else if sets.len()!=2 { eprintln!("tr requires 2 sets"); return 2; }
    let set_from = sets.get(0).cloned().unwrap_or_default();
    let set_to = if delete_mode { String::new() } else { sets.get(1).cloned().unwrap_or_default() };
    if !delete_mode && set_from.len()!=set_to.len() { eprintln!("tr: sets must be same length (no padding implemented)"); return 2; }
    // gather inputs (stdin path not yet supported => need file or redir)
    let mut files:Vec<String>=Vec::new();
    // remaining args beyond sets ignored (already parsed)
    if let Some(f)=&redir.in_file { files.push(f.clone()); }
    // if user passed files after sets (not implemented) - future
    if files.is_empty() { return 0; }
    let vfsd_path = env::var("LLMSH_VFSD_BIN").unwrap_or_else(|_| "vfsd/target/debug/vfsd".to_string());
    let mut client = match VfsClient::spawn(&vfsd_path, allow_read, allow_write, pass_fd) { Ok(c)=>c, Err(e)=> { eprintln!("vfsd spawn failed: {e}"); return 5; } };
    let mut out_handle: Option<u32> = None; // vfs handle if writing to file
    let mut stdout_handle = std::io::stdout();
    // build table
    let mut map = [0u16;256];
    for i in 0..256 { map[i]=i as u16; }
    if delete_mode {
        for &b in set_from.as_bytes() { map[b as usize] = 0xFFFF; }
    } else {
    for (fb,tb) in set_from.as_bytes().iter().zip(set_to.as_bytes()) { map[*fb as usize] = *tb as u16; }
    }
    if let Some((of, append))=&redir.out_file {
        match client.open_write(of, *append) { Ok(Ok(h))=> out_handle=Some(h), Ok(Err(code))=> { eprintln!("open_write {} error: {:?}", of, code); return map_exit(&code); }, Err(e)=> { eprintln!("protocol open_write {of} error: {e}"); return 6; } }
    }
    for p in files.iter() {
        let h = match client.open_read(p) { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open_read {} error: {:?}", p, code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open_read {p} error: {e}"); return 6; } };
        loop {
            match client.read_chunk(h, 8192) {
                Ok(Ok((data,eof))) => {
                    if data.is_empty() && eof { break; }
                    if !data.is_empty() {
                        let mut out:Vec<u8>=Vec::with_capacity(data.len());
                        for &b in &data { let m = map[b as usize]; if m==0xFFFF { continue; } if m<256 { out.push(m as u8); } }
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
fn run_builtin_sed(args:&Vec<String>, redir:&RedirSpec, allow_read:&[String], allow_write:&[String], pass_fd:Option<i32>) -> i32 {
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
    let vfsd_path = env::var("LLMSH_VFSD_BIN").unwrap_or_else(|_| "vfsd/target/debug/vfsd".to_string());
    let mut client = match VfsClient::spawn(&vfsd_path, allow_read, allow_write, pass_fd) { Ok(c)=>c, Err(e)=> { eprintln!("vfsd spawn failed: {e}"); return 5; } };
    let mut out_handle: Option<u32> = None;
    if let Some((path, append))=&redir.out_file { match client.open_write(path, *append) { Ok(Ok(h))=>out_handle=Some(h), Ok(Err(code))=>{ eprintln!("open_write error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open_write error: {e}"); return 6; } } }
    let mut stdout_handle = std::io::stdout();
    for p in files.iter() {
        let h = match client.open_read(p) { Ok(Ok(h))=>h, Ok(Err(code))=>{ eprintln!("open_read {} error: {:?}", p, code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol open_read {p} error: {e}"); return 6; } };
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
                                    let pat = match Regex::new(&pattern) { Ok(r)=>r, Err(e)=> { eprintln!("sed: invalid regex: {e}"); return 2; } };
                                    if global { line = pat.replace_all(&line, repl.as_str()).to_string(); } else { line = pat.replace(&line, repl.as_str()).to_string(); }
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
                        if !buf_acc.is_empty() { let mut line = String::from_utf8_lossy(&buf_acc).to_string(); if !pattern.is_empty() { if use_regex { let pat = match Regex::new(&pattern) { Ok(r)=>r, Err(e)=>{ eprintln!("sed: invalid regex: {e}"); return 2; } }; if global { line = pat.replace_all(&line, repl.as_str()).to_string(); } else { line = pat.replace(&line, repl.as_str()).to_string(); } } else { if global { let mut out_line=String::new(); let mut pos=0; while let Some(pos2)=line[pos..].find(&pattern) { out_line.push_str(&line[pos..pos+pos2]); out_line.push_str(&repl); pos+=pos2+pattern.len(); } out_line.push_str(&line[pos..]); line=out_line; } else if let Some(pos2)=line.find(&pattern) { line = format!("{}{}{}", &line[..pos2], repl, &line[pos2+pattern.len()..]); } } } let out_line = line.into_bytes(); if let Some(hout)=out_handle { match client.write_chunk(hout, &out_line) { Ok(Ok(_))=>{}, Ok(Err(code))=>{ eprintln!("write error: {:?}", code); return map_exit(&code); }, Err(e)=>{ eprintln!("protocol write error: {e}"); return 6; } } } else if let Err(e)=stdout_handle.write_all(&out_line) { eprintln!("stdout write error: {e}"); return 7; } buf_acc.clear(); }
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

struct HelpEntry { name:&'static str, usage:&'static str, desc:&'static str,
    options:&'static [(&'static str,&'static str)], examples:&'static [(&'static str,&'static str)], related:&'static [&'static str] }

static HELP_ENTRIES:&[HelpEntry] = &[
    HelpEntry { name:"cat", usage:"cat [file...]", desc:"concatenate files and print on stdout", options:&[], examples:&[("cat file.txt","Display contents"),("cat a b","Concatenate a and b")], related:&["head","tail"] },
    HelpEntry { name:"head", usage:"head [-n lines] [file...]", desc:"output the first part of files", options:&[("-n N","output first N lines")], examples:&[("head -10 file.txt","Show first 10 lines")], related:&["tail","cat"] },
    HelpEntry { name:"tail", usage:"tail [-n lines] [file...]", desc:"output the last part of files", options:&[("-n N","output last N lines")], examples:&[("tail -20 file.txt","Show last 20 lines")], related:&["head","cat"] },
    HelpEntry { name:"wc", usage:"wc [options] [file...]", desc:"print line, word, byte counts", options:&[("-l","lines only"),("-w","words only"),("-c","bytes only")], examples:&[("wc file.txt","Show all counts")], related:&["grep","cat"] },
    HelpEntry { name:"grep", usage:"grep [-i] [-n] [-E] pattern [file...]", desc:"search lines matching pattern (literal default, -E regex)", options:&[("-i","ignore case"),("-n","show line numbers"),("-E","regex pattern")], examples:&[("grep foo file","Find foo"),("grep -i -E 'foo|bar' file","Regex OR match"),("cat f | grep -n bar","Pipe with line numbers")], related:&["sed","wc"] },
    HelpEntry { name:"tr", usage:"tr [-d] set1 [set2]", desc:"translate or delete characters (1:1 literal)", options:&[("-d","delete characters in set1")], examples:&[("tr abc xyz < in","Map a->x b->y c->z"),("tr -d 0-9 < in","Delete digits (range not yet supported)")], related:&["sed"] },
    HelpEntry { name:"sed", usage:"sed s/pat/repl/[gE] [file...]", desc:"stream substitution (literal by default, E enables regex)", options:&[("g","global replace"),("E","enable regex (alias r)")], examples:&[("sed s/foo/bar/ file","Replace first foo"),("sed s/foo/bar/g file","Replace all foo"),("sed 's/fo*/X/E' file","Regex replace")], related:&["grep","tr"] },
    HelpEntry { name:"help", usage:"help [command]", desc:"list commands or show detailed help", options:&[], examples:&[("help","List commands"),("help grep","Show grep help")], related:&[] },
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
    for name in args { match find_help(name) { Some(e)=> { let txt = format_entry(e); let _=write!(stdout, "{}", txt); }, None=> { let _=writeln!(stdout, "no help for command: {}", name); status=2; } } }
    let _=stdout.flush();
    status
}
// ----- Duplicate legacy block removed above (parsing + skeleton impl) -----
