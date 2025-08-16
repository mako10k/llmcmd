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
fn sed_hold_h_then_g_upper() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().unwrap();
    let p = dir.path().join("in.txt");
    std::fs::write(&p, b"A\nB\n").unwrap();
    // h: hold=A (on line1), G on line2: pattern= "B\nA"
    let script = format!("sed '1h;2G' {}", p.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", p.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "A\nB\nA\n");
}

#[test]
fn sed_hold_h_upper_then_g() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().unwrap();
    let p = dir.path().join("in.txt");
    std::fs::write(&p, b"A\nB\n").unwrap();
    // H on line1 appends "A" to hold; g on line2 replaces pattern with hold => "A"
    let script = format!("sed '1H;2g' {}", p.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", p.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    assert_eq!(out, "A\nA\n");
}

#[test]
fn sed_hold_x_swap() {
    if ensure_vfsd().is_none() { eprintln!("SKIP: vfsd not found"); return; }
    let dir = tempfile::tempdir().unwrap();
    let p = dir.path().join("in.txt");
    std::fs::write(&p, b"L1\nL2\n").unwrap();
    // x on line1 swaps empty hold with L1 => pattern becomes empty, auto print prints empty line; on line2, swap L2<->L1 then print L1
    let script = format!("sed 'x' {}", p.to_string_lossy());
    let (code, out) = run_llmsh(&script, &[("-i", p.to_str().unwrap())]).unwrap();
    assert_eq!(code, 0);
    // Behavior note: Our engine prints each (possibly modified) pattern space. After x on first line, pattern becomes empty => prints "\n".
    assert_eq!(out, "\nL1\n");
}
