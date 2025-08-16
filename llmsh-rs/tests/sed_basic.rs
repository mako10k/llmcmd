use std::process::{Command, Stdio};
use std::io::Read;

fn llmsh_path() -> String {
    std::env::var("CARGO_BIN_EXE_llmsh-rs").unwrap_or_else(|_| "target/debug/llmsh-rs".to_string())
}

fn ensure_vfsd() -> Option<String> {
    if let Ok(p) = std::env::var("LLMSH_VFSD_BIN") { return Some(p); }
    // search PATH
    if let Some(path_var) = std::env::var_os("PATH") {
        for comp in std::env::split_paths(&path_var) {
            let cand = comp.join("vfsd");
            if cand.is_file() { return Some(cand.to_string_lossy().to_string()); }
        }
    }
    // sibling of llmsh binary
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
fn sed_print_regex() {
    let vfsd = ensure_vfsd();
    if vfsd.is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().expect("tempdir");
    let path = dir.path().join("in.txt");
    std::fs::write(&path, b"ok\nerror: A\nok\nerror: B\n").unwrap();
    let script = format!("sed -n /{}/p {}", "error", path.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", path.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "error: A\nerror: B\n");
}

#[test]
fn sed_delete_range() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().expect("tempdir");
    let path = dir.path().join("in.txt");
    std::fs::write(&path, b"1\n2\n3\n4\n5\n").unwrap();
    let script = format!("sed 2,3d {}", path.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", path.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "1\n4\n5\n");
}

#[test]
fn sed_append_insert() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().expect("tempdir");
    let path = dir.path().join("in.txt");
    std::fs::write(&path, b"one\ntwo\nthree\n").unwrap();
    // insert before 1, append after 3 (single word texts to avoid tokenization issues)
    let script = format!("sed 1i header {}", path.to_string_lossy());
    let (code1, out1) = run_llmsh(&script, &[("-i", path.to_str().unwrap())]).unwrap();
    assert_eq!(code1, 0);
    assert_eq!(out1, "header\none\ntwo\nthree\n");
    let script2 = format!("sed 3a appended {}", path.to_string_lossy());
    let (code2, out2) = run_llmsh(&script2, &[("-i", path.to_str().unwrap())]).unwrap();
    assert_eq!(code2, 0);
    assert_eq!(out2, "one\ntwo\nthree\nappended\n");
}

#[test]
fn sed_substitute_literal_global() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().expect("tempdir");
    let path = dir.path().join("in.txt");
    std::fs::write(&path, b"foo foo\nbar foo\n").unwrap();
    let script = format!("sed s/foo/bar/g {}", path.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", path.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "bar bar\nbar bar\n");
}

#[test]
fn sed_substitute_regex_backrefs_ignorecase() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().expect("tempdir");
    let path = dir.path().join("in.txt");
    std::fs::write(&path, b"ab AB aB\n").unwrap();
    // Use regex with backrefs and ignore-case
    let script = format!("sed s/(a)(b)/X\\1Y\\2/EI {}", path.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", path.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "XaYb XA YB\n");
}
