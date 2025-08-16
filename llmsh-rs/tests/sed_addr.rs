use std::process::{Command, Stdio};
use std::io::Read;

fn llmsh_path() -> String {
    std::env::var("CARGO_BIN_EXE_llmsh-rs").unwrap_or_else(|_| "target/debug/llmsh-rs".to_string())
}

fn ensure_vfsd() -> Option<String> {
    if let Ok(p) = std::env::var("LLMSH_VFSD_BIN") { return Some(p); }
    if let Some(path_var) = std::env::var_os("PATH") {
        for comp in std::env::split_paths(&path_var) {
            let cand = comp.join("vfsd");
            if cand.is_file() { return Some(cand.to_string_lossy().to_string()); }
        }
    }
    let llmsh = llmsh_path();
    if let Some(dir) = std::path::Path::new(&llmsh).parent() {
        let cand = dir.join("vfsd");
        if cand.is_file() { return Some(cand.to_string_lossy().to_string()); }
    }
    None
}

fn run_llmsh(script: &str, inputs: &[(&str, &str)]) -> Option<(i32, String)> {
    let vfsd = ensure_vfsd()?;
    std::env::set_var("LLMSH_VFSD_BIN", vfsd);
    let bin = llmsh_path();
    let mut cmd = Command::new(bin);
    cmd.arg("-c").arg(script);
    for (flag, path) in inputs { cmd.arg(*flag).arg(*path); }
    cmd.stdout(Stdio::piped());
    let mut child = cmd.spawn().ok()?;
    let mut out = String::new();
    child.stdout.as_mut().unwrap().read_to_string(&mut out).ok()?;
    let status = child.wait().ok()?;
    Some((status.code().unwrap_or(1), out))
}

#[test]
fn sed_negation_print_non_matches() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().expect("tempdir");
    let path = dir.path().join("in.txt");
    std::fs::write(&path, b"ok\nerror A\nok\nerror B\n").unwrap();
    let script = format!("sed -n /{}/!p {}", "error", path.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", path.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "ok\nok\n");
}

#[test]
fn sed_last_line_append() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().expect("tempdir");
    let path = dir.path().join("in.txt");
    std::fs::write(&path, b"a\nb\nc\n").unwrap();
    let script = format!("sed '$a EOF' {}", path.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", path.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "a\nb\nc\nEOF\n");
}

#[test]
fn sed_range_negation_print_outside_range() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().expect("tempdir");
    let path = dir.path().join("in.txt");
    std::fs::write(&path, b"1\n2\n3\n4\n5\n").unwrap();
    let script = format!("sed -n 2,4!p {}", path.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", path.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "1\n5\n");
}

#[test]
fn sed_regex_address_range_print() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().expect("tempfile");
    let path = dir.path().join("in.txt");
    std::fs::write(&path, b"foo\nstart\nmid\nend\nbar\n").unwrap();
    let script = format!("sed -n /start/,/end/p {}", path.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", path.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "start\nmid\nend\n");
}
