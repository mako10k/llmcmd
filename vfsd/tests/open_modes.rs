use serde_json::json;
mod test_util; use test_util::spawn_vfsd;

#[test]
fn open_r_nonexistent_returns_noent() {
    let req = json!({"id":"1","op":"open","params":{"path":"nonexistent.txt","mode":"r"}});
    let mut conn = spawn_vfsd(&[], &[]).expect("spawn");
    let resp = conn.send(&req).unwrap();
    assert_eq!(resp.get("ok").and_then(|b| b.as_bool()), Some(false));
    let code = resp.pointer("/error/code").and_then(|v| v.as_str()).unwrap_or("");
    assert_eq!(code, "E_NOENT");
}
