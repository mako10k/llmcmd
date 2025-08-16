use serde_json::json;
use base64::engine::general_purpose::STANDARD;
use base64::Engine;
use std::fs::{File, OpenOptions};
use std::io::Write;
mod test_util; use test_util::spawn_vfsd;

fn b64(s: &str) -> String { STANDARD.encode(s.as_bytes()) }

#[test]
fn invalid_mode() {
    let mut conn = spawn_vfsd(&[], &[]).unwrap();
    let resp = conn.send(&json!({"id":"1","op":"open","params":{"path":"x","mode":"rr"}})).unwrap();
    assert_eq!(resp["ok"], false);
    assert_eq!(resp["error"]["code"], "E_ARG");
}

#[test]
fn read_max_zero() {
    let mut conn = spawn_vfsd(&[], &[]).unwrap();
    let _ = conn.send(&json!({"id":"1","op":"open","params":{"path":"virt_rw","mode":"rw"}})).unwrap();
    let resp = conn.send(&json!({"id":"2","op":"read","params":{"h":1,"max":0}})).unwrap();
    assert_eq!(resp["ok"], false); assert_eq!(resp["error"]["code"], "E_ARG");
}

#[test]
fn write_on_read_only() {
    let path = "test_read_only.txt";
    { let mut f = File::create(path).unwrap(); f.write_all(b"DATA").unwrap(); }
    let mut conn = spawn_vfsd(&[path], &[]).unwrap();
    let _ = conn.send(&json!({"id":"1","op":"open","params":{"path":path,"mode":"r"}})).unwrap();
    let resp = conn.send(&json!({"id":"2","op":"write","params":{"h":1,"data": b64("X")}})).unwrap();
    assert_eq!(resp["ok"], false); assert_eq!(resp["error"]["code"], "E_PERM");
}

#[test]
fn append_and_then_read_via_rw() {
    let path = "append_test.txt";
    let mut conn = spawn_vfsd(&[], &[path]).unwrap();
    let resps = conn.send_many(vec![
        json!({"id":"1","op":"open","params":{"path":path,"mode":"a"}}),
        json!({"id":"2","op":"write","params":{"h":1,"data": b64("A")}}),
        json!({"id":"3","op":"write","params":{"h":1,"data": b64("B")}}),
        json!({"id":"4","op":"close","params":{"h":1}}),
    ]).unwrap();
    assert!(resps.iter().all(|r| r["ok"].as_bool().unwrap()));
    let _ = conn.send(&json!({"id":"5","op":"open","params":{"path":path,"mode":"rw"}})).unwrap();
    let read = conn.send(&json!({"id":"6","op":"read","params":{"h":2,"max":16}})).unwrap();
    assert_eq!(read["ok"], true);
    let data = String::from_utf8(STANDARD.decode(read["result"]["data"].as_str().unwrap()).unwrap()).unwrap();
    assert_eq!(data, "AB");
}

#[test]
fn close_then_access() {
    let mut conn = spawn_vfsd(&[], &[]).unwrap();
    let _ = conn.send(&json!({"id":"1","op":"open","params":{"path":"v1","mode":"rw"}})).unwrap();
    let _ = conn.send(&json!({"id":"2","op":"close","params":{"h":1}})).unwrap();
    let resp = conn.send(&json!({"id":"3","op":"read","params":{"h":1,"max":8}})).unwrap();
    assert_eq!(resp["ok"], false); assert_eq!(resp["error"]["code"], "E_CLOSED");
}

#[test]
fn virtual_path_modes_and_truncate() {
    let mut conn = spawn_vfsd(&[], &["real_trunc.txt"]).unwrap();
    let r_resp = conn.send(&json!({"id":"1","op":"open","params":{"path":"virt_missing","mode":"r"}})).unwrap();
    assert_eq!(r_resp["ok"], false); assert_eq!(r_resp["error"]["code"], "E_NOENT");
    let _ = conn.send(&json!({"id":"2","op":"open","params":{"path":"virt_w","mode":"w"}})).unwrap();
    let read_fail = conn.send(&json!({"id":"3","op":"read","params":{"h":2,"max":4}})).unwrap();
    assert_eq!(read_fail["ok"], false); assert_eq!(read_fail["error"]["code"], "E_PERM");
    let resps = conn.send_many(vec![
        json!({"id":"4","op":"open","params":{"path":"virt_rw2","mode":"rw"}}),
        json!({"id":"5","op":"write","params":{"h":3,"data": b64("hi")}}),
        json!({"id":"6","op":"read","params":{"h":3,"max":8}}),
    ]).unwrap();
    assert_eq!(resps[2]["ok"], true);
    let got = String::from_utf8(STANDARD.decode(resps[2]["result"]["data"].as_str().unwrap()).unwrap()).unwrap();
    assert_eq!(got, "hi");
    let path = "real_trunc.txt";
    { let mut f = OpenOptions::new().create(true).write(true).open(path).unwrap(); f.write_all(b"HELLO").unwrap(); }
    let mut conn2 = spawn_vfsd(&[path], &[path]).unwrap();
    let _ = conn2.send(&json!({"id":"1","op":"open","params":{"path":path,"mode":"rw"}})).unwrap();
    let _ = conn2.send(&json!({"id":"2","op":"read","params":{"h":1,"max":16}})).unwrap();
    let _ = conn2.send(&json!({"id":"3","op":"close","params":{"h":1}})).unwrap();
    let _ = conn2.send(&json!({"id":"4","op":"open","params":{"path":path,"mode":"w"}})).unwrap();
    let _ = conn2.send(&json!({"id":"5","op":"write","params":{"h":2,"data": b64("X")}})).unwrap();
    let _ = conn2.send(&json!({"id":"6","op":"close","params":{"h":2}})).unwrap();
    let _ = conn2.send(&json!({"id":"7","op":"open","params":{"path":path,"mode":"rw"}})).unwrap();
    let read = conn2.send(&json!({"id":"8","op":"read","params":{"h":3,"max":16}})).unwrap();
    assert_eq!(read["ok"], true);
    let txt = String::from_utf8(STANDARD.decode(read["result"]["data"].as_str().unwrap()).unwrap()).unwrap();
    assert!(txt == "X" || txt == "", "unexpected content after truncate: {txt}");
}
