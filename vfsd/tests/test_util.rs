use std::io::{Read, Write};
use std::process::{Child, Command, Stdio, ChildStdin, ChildStdout};
use serde_json::Value;
use anyhow::Result;

pub struct Conn {
    child: Child,
    stdin: ChildStdin,
    stdout: ChildStdout,
}

impl Conn {
    pub fn send(&mut self, v: &Value) -> Result<Value> {
        let data = serde_json::to_vec(v)?;
        let len = (data.len() as u32).to_be_bytes();
        self.stdin.write_all(&len)?;
        self.stdin.write_all(&data)?;
        self.stdin.flush()?;
        // read response frame
        let mut len_buf = [0u8;4];
        self.stdout.read_exact(&mut len_buf)?;
        let resp_len = u32::from_be_bytes(len_buf) as usize;
        let mut buf = vec![0u8; resp_len];
        self.stdout.read_exact(&mut buf)?;
        Ok(serde_json::from_slice(&buf)?)
    }

    pub fn send_many(&mut self, reqs: Vec<Value>) -> Result<Vec<Value>> {
        for v in &reqs {
            let data = serde_json::to_vec(v)?;
            let len = (data.len() as u32).to_be_bytes();
            self.stdin.write_all(&len)?;
            self.stdin.write_all(&data)?;
        }
        self.stdin.flush()?;
        let mut resps = Vec::with_capacity(reqs.len());
        for _ in 0..reqs.len() {
            let mut len_buf=[0u8;4];
            self.stdout.read_exact(&mut len_buf)?;
            let resp_len = u32::from_be_bytes(len_buf) as usize;
            let mut buf = vec![0u8; resp_len];
            self.stdout.read_exact(&mut buf)?;
            resps.push(serde_json::from_slice(&buf)?);
        }
        Ok(resps)
    }
}

impl Drop for Conn {
    fn drop(&mut self) {
        let _ = self.child.wait();
    }
}

pub fn spawn_vfsd(allow_read: &[&str], allow_write: &[&str]) -> Result<Conn> {
    let mut cmd = Command::new(env!("CARGO_BIN_EXE_vfsd"));
    cmd.env("VFS_TEST_DEBUG", "1");
    // Default stdio mode (no args needed). Allow lists via -i/-o
    for r in allow_read { cmd.arg("-i").arg(r); }
    for w in allow_write { cmd.arg("-o").arg(w); }
    cmd.stdin(Stdio::piped()).stdout(Stdio::piped()).stderr(Stdio::inherit());
    let mut child = cmd.spawn()?;
    let stdin = child.stdin.take().expect("child stdin");
    let stdout = child.stdout.take().expect("child stdout");
    Ok(Conn { child, stdin, stdout })
}
