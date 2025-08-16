use std::io::{Read, Write};
use std::process::{Child, Command, Stdio};
use std::os::unix::io::{IntoRawFd, FromRawFd};
use nix::sys::socket::{socketpair, AddressFamily, SockType, SockFlag};
use serde_json::Value;
use anyhow::Result;

pub struct Conn {
    child: Child,
    fd: std::fs::File,
}

impl Conn {
    pub fn send(&mut self, v: &Value) -> Result<Value> {
        let data = serde_json::to_vec(v)?;
        let len = (data.len() as u32).to_be_bytes();
        self.fd.write_all(&len)?;
        self.fd.write_all(&data)?;
        self.fd.flush()?;
        // read response frame
        let mut len_buf = [0u8;4];
        self.fd.read_exact(&mut len_buf)?;
        let resp_len = u32::from_be_bytes(len_buf) as usize;
        let mut buf = vec![0u8; resp_len];
        self.fd.read_exact(&mut buf)?;
        Ok(serde_json::from_slice(&buf)?)
    }

    pub fn send_many(&mut self, reqs: Vec<Value>) -> Result<Vec<Value>> {
        for v in &reqs {
            let data = serde_json::to_vec(v)?;
            let len = (data.len() as u32).to_be_bytes();
            self.fd.write_all(&len)?;
            self.fd.write_all(&data)?;
        }
        self.fd.flush()?;
        let mut resps = Vec::with_capacity(reqs.len());
        for _ in 0..reqs.len() {
            let mut len_buf=[0u8;4];
            self.fd.read_exact(&mut len_buf)?;
            let resp_len = u32::from_be_bytes(len_buf) as usize;
            let mut buf = vec![0u8; resp_len];
            self.fd.read_exact(&mut buf)?;
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
    // Create socketpair for control channel
    let (cli, parent) = socketpair(AddressFamily::Unix, SockType::Stream, None, SockFlag::empty())?;
    let cli_fd = cli.into_raw_fd();
    let parent_fd = parent.into_raw_fd();
    let mut cmd = Command::new(env!("CARGO_BIN_EXE_vfsd"));
    cmd.env("VFS_TEST_DEBUG", "1");
    cmd.arg("--client-vfs-fd").arg(cli_fd.to_string());
    for r in allow_read { cmd.arg("-i").arg(r); }
    for w in allow_write { cmd.arg("-o").arg(w); }
    cmd.stdin(Stdio::null()).stdout(Stdio::null()).stderr(Stdio::inherit());
    let child = cmd.spawn()?;
    // Parent uses the other end
    let fd_file = unsafe { std::fs::File::from_raw_fd(parent_fd) };
    Ok(Conn { child, fd: fd_file })
}
