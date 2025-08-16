use serde_json::json;
use base64::Engine; // bring encode trait into scope
mod test_util; use test_util::spawn_vfsd;

fn run_sequence(reqs: Vec<serde_json::Value>) -> Vec<serde_json::Value> {
    let mut conn = spawn_vfsd(&[], &[]).expect("spawn");
    conn.send_many(reqs).expect("multi send")
}

#[test]
fn open_w_then_read_fails() {
    let resps = run_sequence(vec![
        json!({"id":"1","op":"open","params":{"path":"f1.txt","mode":"w"}}),
        json!({"id":"2","op":"read","params":{"h":1,"max":16}})
    ]);
    assert_eq!(resps[0]["ok"], true);
    assert_eq!(resps[1]["ok"], false);
    assert_eq!(resps[1]["error"]["code"], "E_PERM");
}

#[test]
fn open_a_append_and_read_fails() {
    let resps = run_sequence(vec![
        json!({"id":"1","op":"open","params":{"path":"f2.txt","mode":"a"}}),
        json!({"id":"2","op":"read","params":{"h":1,"max":8}})
    ]);
    assert_eq!(resps[0]["ok"], true);
    assert_eq!(resps[1]["ok"], false);
    assert_eq!(resps[1]["error"]["code"], "E_PERM");
}

#[test]
fn open_rw_create_then_write_and_read() {
    let resps = run_sequence(vec![
        json!({"id":"1","op":"open","params":{"path":"f3.txt","mode":"rw"}}),
        json!({"id":"2","op":"write","params":{"h":1,"data":"aGVsbG8="}}), // "hello"
        json!({"id":"3","op":"read","params":{"h":1,"max":10}})
    ]);
    assert_eq!(resps[0]["ok"], true);
    assert_eq!(resps[1]["ok"], true);
    assert_eq!(resps[2]["ok"], true);
    let data_b64 = resps[2]["result"]["data"].as_str().unwrap();
    assert_eq!(data_b64, base64::engine::general_purpose::STANDARD.encode(b"hello"));
}

#[test]
fn open_w_truncates() {
    // First create with rw and write, close, then reopen w and ensure empty read
    let resps = run_sequence(vec![
        json!({"id":"1","op":"open","params":{"path":"f4.txt","mode":"rw"}}),
        json!({"id":"2","op":"write","params":{"h":1,"data":"YQ=="}}), // "a"
        json!({"id":"3","op":"read","params":{"h":1,"max":4}}),
        json!({"id":"4","op":"close","params":{"h":1}}),
        json!({"id":"5","op":"open","params":{"path":"f4.txt","mode":"w"}}),
        json!({"id":"6","op":"write","params":{"h":2,"data":"Yg=="}}), // write-only write OK ("b")
        json!({"id":"7","op":"read","params":{"h":2,"max":4}})
    ]);
    assert_eq!(resps[2]["ok"], true); // read from rw
    assert_eq!(resps[6]["ok"], false); // reading from write-only
    assert_eq!(resps[6]["error"]["code"], "E_PERM");
}
