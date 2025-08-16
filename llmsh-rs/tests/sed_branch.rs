use std::process::{Command, Stdio};
use std::io::Read;

fn llmsh_path() -> String { std::env::var("CARGO_BIN_EXE_llmsh-rs").unwrap_or_else(|_| "target/debug/llmsh-rs".to_string()) }

fn ensure_vfsd() -> Option<String> {
    if let Ok(p) = std::env::var("LLMSH_VFSD_BIN") { return Some(p); }
    if let Some(path_var) = std::env::var_os("PATH") { for comp in std::env::split_paths(&path_var) { let cand = comp.join("vfsd"); if cand.is_file() { return Some(cand.to_string_lossy().to_string()); } } }
    let llmsh = llmsh_path(); if let Some(dir) = std::path::Path::new(&llmsh).parent() { let cand = dir.join("vfsd"); if cand.is_file() { return Some(cand.to_string_lossy().to_string()); } }
    None
}

fn run_llmsh(script: &str, inputs: &[(&str, &str)]) -> Option<(i32, String)> {
    let vfsd = ensure_vfsd()?; std::env::set_var("LLMSH_VFSD_BIN", vfsd);
    let bin = llmsh_path(); let mut cmd = Command::new(bin); cmd.arg("-c").arg(script); for (flag, path) in inputs { cmd.arg(*flag).arg(*path); }
    cmd.stdout(Stdio::piped()); let mut child = cmd.spawn().ok()?; let mut out = String::new(); child.stdout.as_mut().unwrap().read_to_string(&mut out).ok()?; let status = child.wait().ok()?; Some((status.code().unwrap_or(1), out))
}

#[test]
fn sed_branch_t_loop_converges() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().unwrap(); let p = dir.path().join("in.txt");
    std::fs::write(&p, b"aaaa\n").unwrap();
    // Repeatedly reduce double a's until none remain
    let script = format!("sed ':a; s/aa/a/; t a' {}", p.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", p.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "a\n");
}

#[test]
fn sed_branch_t_skips_on_no_subst() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().unwrap(); let p = dir.path().join("in.txt");
    std::fs::write(&p, b"foo\nbaz\n").unwrap();
    // If first s/// does not substitute, jump to :skip and avoid changing baz
    let script = format!("sed 's/foo/bar/; T skip; s/baz/BAZ/; :skip' {}", p.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", p.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "bar\nbaz\n");
}
