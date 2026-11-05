//! Holds or tries the shared cache lock exactly as the Rust client takes it.
//!
//! The .NET client's interoperability tests build and run this beside it. The open
//! and the lock calls are those of `lock_file` and `lock_exclusively` in
//! client/rust/opensysml/src/binary.rs: `rustix::fs::fcntl_lock` over the whole
//! file on Unix, and fs4's `LockFileEx` over the whole file on Windows.
//!
//! Usage: `hold <lock-file>` takes the lock as the client does, waiting for it,
//! prints `held`, releases it at a line or end of input and prints `released`;
//! `try <lock-file>` takes it without waiting and prints `free` (releasing it) or
//! `busy`.

use std::fs::{self, File};
use std::io::{self, BufRead, Write};
use std::path::Path;
use std::process::ExitCode;

fn open(path: &Path) -> io::Result<File> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let mut options = fs::OpenOptions::new();
    options.read(true).write(true).create(true).truncate(false);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    options.open(path)
}

/// Takes the lock, waiting for it or not; `Ok(false)` when another holds it.
#[cfg(unix)]
fn lock(file: &File, wait: bool) -> io::Result<bool> {
    use rustix::fs::{fcntl_lock, FlockOperation};
    let operation = if wait {
        FlockOperation::LockExclusive
    } else {
        FlockOperation::NonBlockingLockExclusive
    };
    match fcntl_lock(file, operation) {
        Ok(()) => Ok(true),
        Err(error)
            if !wait
                && (error == rustix::io::Errno::AGAIN || error == rustix::io::Errno::ACCESS) =>
        {
            Ok(false)
        }
        Err(error) => Err(error.into()),
    }
}

#[cfg(unix)]
fn unlock(file: &File) -> io::Result<()> {
    rustix::fs::fcntl_lock(file, rustix::fs::FlockOperation::Unlock).map_err(Into::into)
}

/// Takes the lock, waiting for it or not; `Ok(false)` when another holds it.
#[cfg(windows)]
fn lock(file: &File, wait: bool) -> io::Result<bool> {
    if wait {
        return fs4::fs_std::FileExt::lock(file).map(|()| true);
    }
    match fs4::fs_std::FileExt::try_lock(file) {
        Ok(()) => Ok(true),
        Err(fs4::TryLockError::WouldBlock) => Ok(false),
        Err(fs4::TryLockError::Error(error)) => Err(error),
    }
}

#[cfg(windows)]
fn unlock(file: &File) -> io::Result<()> {
    fs4::fs_std::FileExt::unlock(file)
}

fn say(word: &str) -> io::Result<()> {
    let mut out = io::stdout().lock();
    writeln!(out, "{word}")?;
    out.flush()
}

fn run(mode: &str, path: &Path) -> io::Result<()> {
    let file = open(path)?;
    if mode == "try" {
        if lock(&file, false)? {
            unlock(&file)?;
            return say("free");
        }
        return say("busy");
    }
    lock(&file, true)?;
    say("held")?;
    let mut line = String::new();
    io::stdin().lock().read_line(&mut line)?;
    unlock(&file)?;
    say("released")
}

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().collect();
    let [_, mode, path] = args.as_slice() else {
        eprintln!("usage: opensysml-lock-holder hold|try <lock-file>");
        return ExitCode::from(2);
    };
    if mode != "hold" && mode != "try" {
        eprintln!("usage: opensysml-lock-holder hold|try <lock-file>");
        return ExitCode::from(2);
    }
    match run(mode, Path::new(path)) {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("opensysml-lock-holder: {error}");
            ExitCode::FAILURE
        }
    }
}
