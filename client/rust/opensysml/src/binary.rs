//! Downloading, verifying and caching the `sysml-grpc` release binary.
//!
//! The cache at `~/.opensysml/bin` is shared with the Python and Java clients,
//! metadata shape included, so every client can tell which release it holds. It
//! is replaced under the lock file they all take, and what a caller is handed is
//! a link to the cached binary under its own digest rather than the cache path,
//! which a later install replaces.
//!
//! # Known limitation: this client verifies pins only
//!
//! A download is verified against the digest table this crate ships
//! (`release-digests.json`, embedded at build time). Unlike the Python, Node and
//! Java clients, it does **not** verify the sigstore-signed `SHA256SUMS.txt`
//! manifest a release publishes, so a release this crate pins no digest for
//! cannot be verified here at all: it is refused. Setting
//! `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` is the only way through, and it accepts
//! the checksum served beside the binary, which is same-origin trust.

use std::borrow::Cow;
use std::collections::BTreeMap;
use std::env;
use std::fs;
use std::fs::File;
use std::io::Read;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Mutex, MutexGuard, OnceLock, PoisonError};
use std::time::Duration;

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use crate::error::Error;

/// Repository releases are downloaded from unless `$OPENSYSML_GITHUB_REPO` names another.
pub(crate) const DEFAULT_GITHUB_REPO: &str = "Open-MBEE/OpenSysML";

/// Origin serving release assets.
const DEFAULT_RELEASE_BASE_URL: &str = "https://github.com";

/// Origin answering the releases API.
const DEFAULT_API_BASE_URL: &str = "https://api.github.com";

/// Set to the repository whose unpinned downloads may be accepted (`1` for any).
const ALLOW_UNPINNED_ENV: &str = "OPENSYSML_ALLOW_UNPINNED_DOWNLOAD";

/// A cached binary is checked against the releases API before it is reused, so
/// every request here is bounded rather than left to hang.
const NETWORK_TIMEOUT: Duration = Duration::from_secs(15);

/// Release binaries are tens of megabytes; this only bounds a runaway response.
const MAX_DOWNLOAD_BYTES: u64 = 512 * 1024 * 1024;

/// Checksums and release metadata are a few kilobytes at most.
const MAX_METADATA_BYTES: u64 = 1024 * 1024;

/// The pinned digest table this crate ships, embedded so the published crate carries it.
const PINNED_DIGESTS: &str = include_str!("../release-digests.json");

/// Repository -> release tag -> asset name -> SHA-256 hex digest.
pub(crate) type PinTable = BTreeMap<String, BTreeMap<String, BTreeMap<String, String>>>;

/// The shipped pins, parsed once.
pub(crate) fn shipped_pins() -> &'static PinTable {
    static PINS: OnceLock<PinTable> = OnceLock::new();
    PINS.get_or_init(|| {
        serde_json::from_str(PINNED_DIGESTS)
            .expect("release-digests.json shipped with this crate is a digest table")
    })
}

/// What the cache metadata beside the binary records, shared with the Python client.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub(crate) struct CacheMetadata {
    /// Release tag downloaded, resolved (never `latest`).
    pub(crate) version: String,
    /// SHA-256 hex digest of the binary written.
    pub(crate) sha256: String,
    /// Repository (owner/repo) it was downloaded from.
    pub(crate) repo: String,
}

/// The `sysml-grpc` file name for the host platform.
pub(crate) fn binary_file_name() -> &'static str {
    if cfg!(windows) {
        "sysml-grpc.exe"
    } else {
        "sysml-grpc"
    }
}

/// The shared cache directory, `~/.opensysml/bin`.
///
/// An unset home is an error rather than the working directory, which would put
/// the cache wherever the process happens to run.
pub(crate) fn default_cache_dir() -> Result<PathBuf, Error> {
    #[cfg(windows)]
    let (variable, home) = ("USERPROFILE", env::var_os("USERPROFILE"));
    #[cfg(not(windows))]
    let (variable, home) = ("HOME", env::var_os("HOME"));
    let home = home.filter(|home| !home.is_empty()).ok_or_else(|| {
        Error::BinaryDownload(format!(
            "${variable} names no home directory, so there is no ~/.opensysml/bin to cache \
             sysml-grpc in"
        ))
    })?;
    Ok(PathBuf::from(home).join(".opensysml").join("bin"))
}

/// Repository releases are downloaded from.
pub(crate) fn default_github_repo() -> String {
    env::var("OPENSYSML_GITHUB_REPO")
        .ok()
        .filter(|repo| !repo.trim().is_empty())
        .unwrap_or_else(|| DEFAULT_GITHUB_REPO.to_owned())
}

/// The release tag `$OPENSYSML_GRPC_VERSION` asks for, if any.
pub(crate) fn env_release_version() -> Option<String> {
    env::var("OPENSYSML_GRPC_VERSION")
        .ok()
        .map(|version| version.trim().to_owned())
        .filter(|version| !version.is_empty())
}

/// Name a release publishes the service binary for a platform under.
///
/// The names follow Go's `GOOS`/`GOARCH`, which is what the release pipeline builds for.
pub(crate) fn release_asset_name(os: &str, arch: &str) -> Result<String, Error> {
    let goos = match os {
        "linux" => "linux",
        "macos" => "darwin",
        "windows" => "windows",
        _ => {
            return Err(Error::UnsupportedPlatform {
                os: os.to_owned(),
                arch: arch.to_owned(),
            })
        }
    };
    let goarch = match arch {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        _ => {
            return Err(Error::UnsupportedPlatform {
                os: os.to_owned(),
                arch: arch.to_owned(),
            })
        }
    };
    // No windows/arm64 build is published, so naming one would only 404 later.
    if goos == "windows" && goarch == "arm64" {
        return Err(Error::UnsupportedPlatform {
            os: os.to_owned(),
            arch: arch.to_owned(),
        });
    }
    let name = format!("sysml-grpc-{goos}-{goarch}");
    Ok(if goos == "windows" {
        format!("{name}.exe")
    } else {
        name
    })
}

/// The release asset for the platform this crate was built for.
pub(crate) fn host_release_asset_name() -> Result<String, Error> {
    release_asset_name(env::consts::OS, env::consts::ARCH)
}

/// Whether an unpinned download from `repo` may fall back to same-origin trust.
///
/// `1` allows any repository; naming repositories keeps the trust with the fork
/// it is granted for.
fn unpinned_allowed(raw: Option<&str>, repo: &str) -> bool {
    let Some(value) = raw.map(str::trim).filter(|value| !value.is_empty()) else {
        return false;
    };
    match value.to_ascii_lowercase().as_str() {
        "0" | "false" | "no" => false,
        "1" | "true" | "yes" => true,
        _ => value.split(',').any(|named| named.trim() == repo),
    }
}

/// Reject a repository that would not name one, before it is put in a URL.
fn validate_repo(repo: &str) -> Result<(), Error> {
    let mut segments = repo.split('/');
    let named = match (segments.next(), segments.next(), segments.next()) {
        (Some(owner), Some(name), None) => [owner, name],
        _ => {
            return Err(Error::BinaryDownload(format!(
                "${{OPENSYSML_GITHUB_REPO}} is {repo:?}, which is not an owner/repo"
            )))
        }
    };
    let usable = named.iter().all(|segment| {
        !segment.is_empty()
            && *segment != "."
            && *segment != ".."
            && segment
                .chars()
                .all(|c| c.is_ascii_alphanumeric() || matches!(c, '.' | '-' | '_'))
    });
    if !usable {
        return Err(Error::BinaryDownload(format!(
            "${{OPENSYSML_GITHUB_REPO}} is {repo:?}, which is not an owner/repo"
        )));
    }
    Ok(())
}

/// Reject a release tag that would not name one, before it is put in a URL.
fn validate_version(version: &str) -> Result<(), Error> {
    let usable = !version.is_empty()
        && !version.starts_with('-')
        && !version.contains("..")
        && version
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, '.' | '-' | '_' | '+'));
    if !usable {
        return Err(Error::BinaryDownload(format!(
            "{version:?} is not a release tag"
        )));
    }
    Ok(())
}

/// Path of the advisory lock every client holds while it replaces the cache.
///
/// Python and Java lock this same file, so no client pairs one release's bytes
/// with another's metadata.
pub(crate) fn lock_path(cache_dir: &Path) -> PathBuf {
    cache_dir.join(format!("{}.lock", binary_file_name()))
}

/// Report to the caller the way this client can, since it has no logger.
fn warn_on_stderr(message: &str) {
    eprintln!("opensysml: warning: {message}");
}

/// One lock per cache, because a POSIX file lock belongs to the process rather
/// than to the thread that took it.
fn process_lock(cache_dir: &Path) -> &'static Mutex<()> {
    static LOCKS: OnceLock<Mutex<BTreeMap<PathBuf, &Mutex<()>>>> = OnceLock::new();
    let mut locks = LOCKS
        .get_or_init(|| Mutex::new(BTreeMap::new()))
        .lock()
        .unwrap_or_else(PoisonError::into_inner);
    // Leaked because the lock outlives every downloader that takes it, and a
    // process holds locks for a handful of caches at most.
    locks
        .entry(lock_path(cache_dir))
        .or_insert_with(|| &*Box::leak(Box::new(Mutex::new(()))))
}

/// The shared cache, held against other threads, processes and clients.
struct CacheGuard {
    /// Dropped first, since closing any descriptor for the file drops the POSIX
    /// lock this whole process holds on it.
    _file: Option<File>,
    /// Released last, so the next thread opens the lock file after this one closed it.
    _threads: MutexGuard<'static, ()>,
}

/// Hold the shared cache until the guard is dropped.
///
/// A cache that cannot be locked across processes is still locked within this
/// one, with a warning, rather than refusing to resolve a binary at all.
fn cache_lock(cache_dir: &Path, warn: &dyn Fn(String)) -> CacheGuard {
    let threads = process_lock(cache_dir)
        .lock()
        .unwrap_or_else(PoisonError::into_inner);
    let path = lock_path(cache_dir);
    let file = match lock_file(&path) {
        Ok(file) => Some(file),
        Err(error) => {
            warn(format!(
                "could not lock {} against other processes: {error}",
                path.display(),
            ));
            None
        }
    };
    CacheGuard {
        _file: file,
        _threads: threads,
    }
}

/// Open the lock file and hold an exclusive lock on it.
fn lock_file(path: &Path) -> Result<File, std::io::Error> {
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
    let file = options.open(path)?;
    lock_exclusively(&file)?;
    Ok(file)
}

/// `fcntl(F_SETLKW)`, which is the lock Python's `lockf` and Java's `FileLock` take.
#[cfg(unix)]
fn lock_exclusively(file: &File) -> Result<(), std::io::Error> {
    rustix::fs::fcntl_lock(file, rustix::fs::FlockOperation::LockExclusive).map_err(Into::into)
}

/// `LockFileEx`, which is the lock Python's `msvcrt.locking` and Java's `FileLock` take.
#[cfg(windows)]
fn lock_exclusively(file: &File) -> Result<(), std::io::Error> {
    use fs4::fs_std::FileExt;
    file.lock()
}

/// Path the cached binary is linked to under its own digest, named as the Python
/// and Java clients name it so all three start the same file.
fn stable_binary_path(binary_path: &Path, digest: &str) -> PathBuf {
    let name = binary_file_name();
    let (root, extension) = match name.strip_suffix(".exe") {
        Some(root) => (root, ".exe"),
        None => (name, ""),
    };
    binary_path.with_file_name(format!("{root}-{}{extension}", &digest[..16]))
}

/// Link the cached binary under its own digest and return that path, so an
/// install over the cache cannot change what a caller starts.
///
/// Call it holding the cache lock, so nothing replaces the cache between the
/// hashing and the linking.
fn stable_binary(binary_path: &Path, warn: &dyn Fn(String)) -> PathBuf {
    let digest = match file_digest(binary_path) {
        Ok(digest) => digest,
        Err(error) => {
            warn(format!(
                "could not hash {}, so a release installed over it may be started: {error}",
                binary_path.display(),
            ));
            return binary_path.to_owned();
        }
    };
    let stable = stable_binary_path(binary_path, &digest);
    if is_executable(&stable) {
        return stable;
    }
    match link(binary_path, &stable) {
        Ok(()) => stable,
        Err(error) => {
            warn(format!(
                "could not link {} to {}, so a release installed over it may be started: {error}",
                binary_path.display(),
                stable.display(),
            ));
            let _ = fs::remove_file(&stable);
            binary_path.to_owned()
        }
    }
}

/// Hard link a file where the filesystem has links, and copy it where it does not.
fn link(source: &Path, target: &Path) -> Result<(), std::io::Error> {
    let mut staged = target.as_os_str().to_owned();
    staged.push(".tmp");
    let staged = PathBuf::from(staged);
    let _ = fs::remove_file(&staged);
    if fs::hard_link(source, &staged).is_err() {
        fs::copy(source, &staged)?;
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        if let Err(error) = fs::set_permissions(&staged, fs::Permissions::from_mode(0o700)) {
            let _ = fs::remove_file(&staged);
            return Err(error);
        }
    }
    fs::rename(&staged, target).inspect_err(|_| {
        let _ = fs::remove_file(&staged);
    })
}

/// Whether a path is a file this user can execute.
fn is_executable(path: &Path) -> bool {
    let Ok(metadata) = fs::metadata(path) else {
        return false;
    };
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        metadata.is_file() && metadata.permissions().mode() & 0o100 != 0
    }
    #[cfg(not(unix))]
    {
        metadata.is_file()
    }
}

/// The digest-named link to the cached binary, when one is cached.
///
/// Resolution reaches the cache without a downloader when no release was asked for.
pub(crate) fn stable_cached_binary(binary_path: &Path) -> Option<PathBuf> {
    let cache_dir = binary_path.parent()?;
    let _held = cache_lock(cache_dir, &|message| warn_on_stderr(&message));
    binary_path
        .is_file()
        .then(|| stable_binary(binary_path, &|message| warn_on_stderr(&message)))
}

/// The SHA-256 hex digest of a file.
fn file_digest(path: &Path) -> Result<String, std::io::Error> {
    let mut file = fs::File::open(path)?;
    let mut hasher = Sha256::new();
    let mut buffer = [0u8; 64 * 1024];
    loop {
        let read = file.read(&mut buffer)?;
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
    }
    Ok(format!("{:x}", hasher.finalize()))
}

/// A download that did not install, and whether it had already replaced the cache.
struct Failure {
    error: Error,
    installed: bool,
}

impl Failure {
    /// A failure with the cache still as it was.
    fn before(error: impl Into<Error>) -> Self {
        Self {
            error: error.into(),
            installed: false,
        }
    }

    /// A failure after the cache was replaced, so the old binary is gone.
    fn after_installing(error: Error) -> Self {
        Self {
            error,
            installed: true,
        }
    }
}

/// Downloads release binaries into the shared cache, verifying them against the shipped pins.
pub(crate) struct Downloader {
    repo: String,
    release_base_url: String,
    api_base_url: String,
    cache_dir: PathBuf,
    asset: String,
    allow_unpinned: Option<String>,
    pins: Cow<'static, PinTable>,
    agent: ureq::Agent,
    warnings: Mutex<Vec<String>>,
}

impl Downloader {
    /// A downloader configured from the environment, for the host platform.
    pub(crate) fn from_env() -> Result<Self, Error> {
        let repo = default_github_repo();
        validate_repo(&repo)?;
        Ok(Self {
            repo,
            release_base_url: DEFAULT_RELEASE_BASE_URL.to_owned(),
            api_base_url: DEFAULT_API_BASE_URL.to_owned(),
            cache_dir: default_cache_dir()?,
            asset: host_release_asset_name()?,
            allow_unpinned: env::var(ALLOW_UNPINNED_ENV).ok(),
            pins: Cow::Borrowed(shipped_pins()),
            agent: agent(),
            warnings: Mutex::new(Vec::new()),
        })
    }

    /// Path the cached binary is installed at.
    pub(crate) fn binary_path(&self) -> PathBuf {
        self.cache_dir.join(binary_file_name())
    }

    /// Path of the record of which release the cached binary was downloaded from.
    fn metadata_path(&self) -> PathBuf {
        let mut name = binary_file_name().to_owned();
        name.push_str(".json");
        self.cache_dir.join(name)
    }

    /// What was recorded beside the cached binary, or nothing readable was.
    fn read_metadata(&self) -> Option<CacheMetadata> {
        let recorded = fs::read_to_string(self.metadata_path()).ok()?;
        serde_json::from_str(&recorded).ok()
    }

    fn write_metadata(&self, metadata: &CacheMetadata) -> Result<(), Error> {
        let path = self.metadata_path();
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }
        let recorded = serde_json::to_string(metadata).map_err(|error| {
            Error::BinaryDownload(format!("could not record the cache: {error}"))
        })?;
        fs::write(path, recorded)?;
        Ok(())
    }

    /// Warn the caller the way this client can: on stderr, and recorded for tests.
    fn warn(&self, message: String) {
        eprintln!("opensysml: warning: {message}");
        self.warnings
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .push(message);
    }

    /// The digest this crate pins for a release asset, if it pins one.
    fn pinned_digest(&self, version: &str, asset: &str) -> Option<&str> {
        self.pins
            .get(&self.repo)?
            .get(version)?
            .get(asset)
            .map(String::as_str)
    }

    /// The digest a download must have.
    ///
    /// The pin wins wherever there is one, and a served checksum contradicting
    /// it is tampering rather than a reason to prefer either. Where there is no
    /// pin this client has nothing left to verify against, because it does not
    /// check the release's signed checksum manifest.
    fn expected_digest(&self, version: &str, asset: &str, served: &str) -> Result<String, Error> {
        let Some(pinned) = self.pinned_digest(version, asset) else {
            if !unpinned_allowed(self.allow_unpinned.as_deref(), &self.repo) {
                return Err(Error::UnpinnedRelease(format!(
                    "opensysml pins no SHA-256 digest for {asset} of {version} of {self_repo}, \
                     and this Rust client does not verify the signed SHA256SUMS.txt manifest \
                     that the Python, Node and Java clients fall back to, so the only checksum \
                     available is the one served beside the binary, which a compromised release \
                     would serve too. Upgrade opensysml to a release that pins {version}, ask \
                     for a pinned release, or accept same-origin trust for this repository by \
                     setting ${ALLOW_UNPINNED_ENV}={self_repo} (or =1 for any repository).",
                    self_repo = self.repo,
                )));
            }
            self.warn(format!(
                "opensysml pins no digest for {asset} of {version} of {}; verifying it against \
                 the checksum served beside it, which detects corruption but not a compromised \
                 release (${ALLOW_UNPINNED_ENV} is set).",
                self.repo,
            ));
            return Ok(served.to_owned());
        };
        if served != pinned {
            return Err(Error::ChecksumMismatch(format!(
                "checksum mismatch for {asset} of {version}: {} serves {served}, but opensysml \
                 pins {pinned}. The release was republished with another binary, or the download \
                 is being tampered with; it was not installed.",
                self.repo,
            )));
        }
        Ok(pinned.to_owned())
    }

    fn download_url(&self, version: &str, asset: &str) -> String {
        format!(
            "{}/{}/releases/download/{version}/{asset}",
            self.release_base_url.trim_end_matches('/'),
            self.repo,
        )
    }

    fn fetch(&self, url: &str, limit: u64) -> Result<Vec<u8>, Error> {
        let response = self
            .agent
            .get(url)
            .call()
            .map_err(|error| Error::BinaryDownload(format!("{url}: {error}")))?;
        response
            .into_body()
            .with_config()
            .limit(limit)
            .read_to_vec()
            .map_err(|error| Error::BinaryDownload(format!("{url}: {error}")))
    }

    /// Resolve the tag of the repository's latest published release.
    pub(crate) fn resolve_latest_version(&self) -> Result<String, Error> {
        let url = format!(
            "{}/repos/{}/releases/latest",
            self.api_base_url.trim_end_matches('/'),
            self.repo,
        );
        let body = self.fetch(&url, MAX_METADATA_BYTES)?;
        #[derive(Deserialize)]
        struct Release {
            tag_name: Option<String>,
        }
        let release: Release = serde_json::from_slice(&body).map_err(|error| {
            Error::BinaryDownload(format!(
                "failed to resolve latest release from {url}: {error}"
            ))
        })?;
        release
            .tag_name
            .filter(|tag| !tag.is_empty())
            .ok_or_else(|| {
                Error::BinaryDownload(format!("latest release of {} has no tag name", self.repo))
            })
    }

    /// Release tag the cached binary was downloaded from, if from this repository.
    ///
    /// The digest is re-checked, so a binary swapped in by hand is not read as
    /// the release it displaced, and forks publishing the same tags do not
    /// answer for each other.
    pub(crate) fn cached_release(&self) -> Option<String> {
        let recorded = self.read_metadata()?;
        if recorded.version.is_empty() || recorded.sha256.is_empty() || recorded.repo != self.repo {
            return None;
        }
        let actual = file_digest(&self.binary_path()).ok()?;
        (actual == recorded.sha256).then_some(recorded.version)
    }

    /// Why the cached binary is not the release that was asked for.
    ///
    /// Nothing asked for is answered by any cache, a locally built binary included.
    pub(crate) fn stale_cache_reason(&self, version: Option<&str>) -> Option<String> {
        let asked = match version {
            None => return None,
            Some("latest") => match self.resolve_latest_version() {
                Ok(resolved) => resolved,
                // Unreachable releases are no reason to discard a working cache.
                Err(_) => return None,
            },
            Some(version) => version.to_owned(),
        };
        let path = self.binary_path().display().to_string();
        match self.cached_release() {
            Some(have) if have == asked => None,
            Some(have) => Some(format!(
                "the binary cached at {path} is {have}, but {asked} was asked for"
            )),
            None => match self.read_metadata().map(|recorded| recorded.repo) {
                Some(recorded_repo) if recorded_repo != self.repo => Some(format!(
                    "the binary cached at {path} was downloaded from {recorded_repo}, but \
                     {asked} of {} was asked for",
                    self.repo,
                )),
                _ => Some(format!(
                    "the binary cached at {path} was not downloaded by this client, so which \
                     release it is cannot be told, and {asked} was asked for"
                )),
            },
        }
    }

    /// [`Self::install`] under the cache lock, without the record of whether the
    /// cache was replaced.
    #[cfg(test)]
    fn download_binary(&self, version: &str) -> Result<PathBuf, Error> {
        let _held = self.hold_cache();
        self.install(version).map_err(|failure| failure.error)
    }

    /// Hold the shared cache against other threads, processes and clients.
    fn hold_cache(&self) -> CacheGuard {
        cache_lock(&self.cache_dir, &|message| self.warn(message))
    }

    /// Download a release binary into the cache, verifying it before installing it.
    ///
    /// A download that fails before the cache is replaced leaves it intact.
    fn install(&self, version: &str) -> Result<PathBuf, Failure> {
        let version = if version == "latest" {
            self.resolve_latest_version().map_err(Failure::before)?
        } else {
            validate_version(version).map_err(Failure::before)?;
            version.to_owned()
        };
        let asset = self.asset.clone();
        let binary_path = self.binary_path();
        fs::create_dir_all(&self.cache_dir).map_err(Failure::before)?;

        let sidecar_url = self.download_url(&version, &format!("{asset}.sha256"));
        let sidecar = self
            .fetch(&sidecar_url, MAX_METADATA_BYTES)
            .map_err(Failure::before)?;
        let served = String::from_utf8_lossy(&sidecar)
            .split_whitespace()
            .next()
            .map(str::to_owned)
            .ok_or_else(|| {
                Failure::before(Error::BinaryDownload(format!(
                    "{sidecar_url} served no checksum for {asset}"
                )))
            })?;
        // Refusing an unverifiable release here means its binary is never fetched.
        let expected = self
            .expected_digest(&version, &asset, &served)
            .map_err(Failure::before)?;

        let body = self
            .fetch(&self.download_url(&version, &asset), MAX_DOWNLOAD_BYTES)
            .map_err(Failure::before)?;
        // A temporary name of this process's own, so concurrent installs do not
        // write over each other's download.
        static NEXT_TEMP: AtomicU64 = AtomicU64::new(0);
        let temp_path = self.cache_dir.join(format!(
            "{}.{}.{}.tmp",
            binary_file_name(),
            std::process::id(),
            NEXT_TEMP.fetch_add(1, Ordering::Relaxed),
        ));
        let staged = self.stage(&temp_path, &body, &version, &asset, &expected);
        if staged.is_err() {
            let _ = fs::remove_file(&temp_path);
        }
        staged.map_err(Failure::before)?;

        // Everything that can still fail is done before the cache is replaced, so
        // a failure here is a failure to install rather than a broken cache.
        if let Err(error) = fs::rename(&temp_path, &binary_path) {
            let _ = fs::remove_file(&temp_path);
            return Err(Failure::before(Error::BinaryDownload(format!(
                "downloaded {version} but could not install it at {}: {error}. A running service \
                 holding that file is the usual cause.",
                binary_path.display(),
            ))));
        }

        self.write_metadata(&CacheMetadata {
            version,
            sha256: expected,
            repo: self.repo.clone(),
        })
        .map_err(Failure::after_installing)?;
        Ok(binary_path)
    }

    /// Write the download to `temp_path` and make it the binary that may replace the cache.
    fn stage(
        &self,
        temp_path: &Path,
        body: &[u8],
        version: &str,
        asset: &str,
        expected: &str,
    ) -> Result<(), Error> {
        fs::write(temp_path, body)?;
        let actual = file_digest(temp_path)?;
        if actual != *expected {
            return Err(Error::ChecksumMismatch(format!(
                "checksum mismatch for {asset} of {version}: expected {expected}, but the \
                 download hashes to {actual}. It may be corrupted or tampered with; it was not \
                 installed."
            )));
        }
        // The cache is this user's, so no one else needs it.
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(temp_path, fs::Permissions::from_mode(0o700))?;
        }
        Ok(())
    }

    /// The cached binary for the release asked for, downloading it when it is not already there.
    ///
    /// A cache that is the release asked for is reused; a stale one is replaced
    /// with a warning; a replacement that cannot be downloaded leaves the working
    /// cache in place, unless the download was refused for integrity reasons.
    ///
    /// The decision is made holding the shared cache lock, and what is returned
    /// for the cache is a digest-named link no later install replaces.
    pub(crate) fn ensure_binary(&self, version: Option<&str>) -> Result<PathBuf, Error> {
        let _held = self.hold_cache();
        let binary_path = self.binary_path();
        let chosen = self.ensure_binary_locked(version)?;
        Ok(if chosen == binary_path {
            stable_binary(&binary_path, &|message| self.warn(message))
        } else {
            chosen
        })
    }

    /// [`Self::ensure_binary`], with the shared cache held.
    fn ensure_binary_locked(&self, version: Option<&str>) -> Result<PathBuf, Error> {
        let binary_path = self.binary_path();
        let mut cached = None;
        if binary_path.is_file() {
            match self.stale_cache_reason(version) {
                None => return Ok(binary_path),
                Some(stale) => {
                    let asked = version.unwrap_or("latest");
                    cached = Some(binary_path);
                    self.warn(format!(
                        "replacing the cached sysml-grpc: {stale}. Downloading {asked}."
                    ));
                }
            }
        }
        let Some(version) = version else {
            return Err(Error::BinaryDownload(format!(
                "no sysml-grpc binary is cached at {} and no release was asked for; build one, \
                 or set $OPENSYSML_GRPC_VERSION to download one.",
                self.binary_path().display(),
            )));
        };

        match self.install(version) {
            Ok(path) => Ok(path),
            // A download refused for integrity is never answered from the cache,
            // and neither is one that already replaced it.
            Err(Failure {
                error: error @ (Error::ChecksumMismatch(_) | Error::UnpinnedRelease(_)),
                ..
            })
            | Err(Failure {
                error,
                installed: true,
            }) => Err(error),
            Err(Failure { error, .. }) => match cached {
                // A release that cannot be had is no reason to lose a working binary.
                Some(path) => {
                    self.warn(format!(
                        "keeping the cached sysml-grpc at {}: {version} was not downloaded \
                         ({error}). It may be an older release than asked for.",
                        path.display(),
                    ));
                    Ok(path)
                }
                None => Err(error),
            },
        }
    }

    #[cfg(test)]
    fn warnings(&self) -> Vec<String> {
        self.warnings
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .clone()
    }
}

fn agent() -> ureq::Agent {
    ureq::Agent::config_builder()
        .timeout_global(Some(NETWORK_TIMEOUT))
        .build()
        .into()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;
    use std::io::Write;
    use std::net::{TcpListener, TcpStream};
    use std::sync::Arc;
    use std::thread;

    /// A release origin serving fixture bytes, so no test touches the network.
    struct FixtureServer {
        base_url: String,
    }

    impl FixtureServer {
        fn start(routes: HashMap<String, Vec<u8>>) -> Self {
            let listener = TcpListener::bind("127.0.0.1:0").expect("bind a fixture port");
            let base_url = format!("http://{}", listener.local_addr().expect("local address"));
            let routes = Arc::new(routes);
            thread::spawn(move || {
                for stream in listener.incoming() {
                    let Ok(stream) = stream else { break };
                    let routes = Arc::clone(&routes);
                    thread::spawn(move || serve(stream, &routes));
                }
            });
            Self { base_url }
        }
    }

    fn serve(mut stream: TcpStream, routes: &HashMap<String, Vec<u8>>) {
        let mut request = [0u8; 4096];
        let read = match stream.read(&mut request) {
            Ok(read) if read > 0 => read,
            _ => return,
        };
        let head = String::from_utf8_lossy(&request[..read]).to_string();
        let path = head
            .split_whitespace()
            .nth(1)
            .unwrap_or_default()
            .to_owned();
        let response = match routes.get(&path) {
            Some(body) => {
                let mut response = format!(
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    body.len()
                )
                .into_bytes();
                response.extend_from_slice(body);
                response
            }
            None => {
                b"HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".to_vec()
            }
        };
        let _ = stream.write_all(&response);
        let _ = stream.flush();
    }

    struct Harness {
        downloader: Downloader,
        _cache: tempdir::TempDir,
    }

    /// A minimal owned temporary directory, so the tests add no dependency.
    mod tempdir {
        use std::fs;
        use std::path::{Path, PathBuf};
        use std::sync::atomic::{AtomicU64, Ordering};

        pub struct TempDir(PathBuf);

        impl TempDir {
            pub fn new(label: &str) -> Self {
                static COUNTER: AtomicU64 = AtomicU64::new(0);
                let unique = format!(
                    "opensysml-{label}-{}-{}",
                    std::process::id(),
                    COUNTER.fetch_add(1, Ordering::Relaxed)
                );
                let path = std::env::temp_dir().join(unique);
                fs::create_dir_all(&path).expect("create a temporary directory");
                Self(path)
            }

            pub fn path(&self) -> &Path {
                &self.0
            }
        }

        impl Drop for TempDir {
            fn drop(&mut self) {
                let _ = fs::remove_dir_all(&self.0);
            }
        }
    }

    const ASSET: &str = "sysml-grpc-linux-amd64";
    const REPO: &str = "Open-MBEE/OpenSysML";

    /// Whether any staged download was left behind in the cache directory.
    fn temporaries_left(cache_dir: &Path) -> bool {
        fs::read_dir(cache_dir)
            .expect("read the cache directory")
            .filter_map(Result::ok)
            .any(|entry| entry.file_name().to_string_lossy().ends_with(".tmp"))
    }

    fn digest_of(bytes: &[u8]) -> String {
        format!("{:x}", Sha256::digest(bytes))
    }

    /// The digest-named path a caller is handed for cached content.
    fn linked(downloader: &Downloader, content: &[u8]) -> PathBuf {
        stable_binary_path(&downloader.binary_path(), &digest_of(content))
    }

    fn release_routes(version: &str, body: &[u8], served_digest: &str) -> HashMap<String, Vec<u8>> {
        let mut routes = HashMap::new();
        routes.insert(
            format!("/{REPO}/releases/download/{version}/{ASSET}"),
            body.to_vec(),
        );
        routes.insert(
            format!("/{REPO}/releases/download/{version}/{ASSET}.sha256"),
            format!("{served_digest}  {ASSET}\n").into_bytes(),
        );
        routes.insert(
            format!("/repos/{REPO}/releases/latest"),
            format!(r#"{{"tag_name": "{version}"}}"#).into_bytes(),
        );
        routes
    }

    fn harness(label: &str, routes: HashMap<String, Vec<u8>>, pins: PinTable) -> Harness {
        let server = FixtureServer::start(routes);
        let cache = tempdir::TempDir::new(label);
        Harness {
            downloader: Downloader {
                repo: REPO.to_owned(),
                release_base_url: server.base_url.clone(),
                api_base_url: server.base_url,
                cache_dir: cache.path().to_path_buf(),
                asset: ASSET.to_owned(),
                allow_unpinned: None,
                pins: Cow::Owned(pins),
                agent: agent(),
                warnings: Mutex::new(Vec::new()),
            },
            _cache: cache,
        }
    }

    fn pin_table(version: &str, digest: &str) -> PinTable {
        let mut assets = BTreeMap::new();
        assets.insert(ASSET.to_owned(), digest.to_owned());
        let mut releases = BTreeMap::new();
        releases.insert(version.to_owned(), assets);
        let mut table = PinTable::new();
        table.insert(REPO.to_owned(), releases);
        table
    }

    #[test]
    fn a_pinned_release_is_downloaded_verified_and_cached() {
        let body = b"the pinned release binary";
        let digest = digest_of(body);
        let harness = harness(
            "pinned",
            release_routes("v0.1.0", body, &digest),
            pin_table("v0.1.0", &digest),
        );
        let downloader = &harness.downloader;

        let path = downloader.download_binary("v0.1.0").expect("download");
        assert_eq!(fs::read(&path).expect("read the cache"), body);
        assert_eq!(downloader.cached_release().as_deref(), Some("v0.1.0"));
        assert!(!temporaries_left(&downloader.cache_dir));
        assert!(downloader.warnings().is_empty());

        let recorded = downloader.read_metadata().expect("metadata");
        assert_eq!(recorded.version, "v0.1.0");
        assert_eq!(recorded.sha256, digest);
        assert_eq!(recorded.repo, REPO);

        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mode = fs::metadata(&path).expect("stat").permissions().mode();
            assert_eq!(mode & 0o777, 0o700);
        }
    }

    #[test]
    fn latest_is_resolved_through_the_releases_api() {
        let body = b"the latest release binary";
        let digest = digest_of(body);
        let harness = harness(
            "latest",
            release_routes("v0.2.0", body, &digest),
            pin_table("v0.2.0", &digest),
        );

        let path = harness
            .downloader
            .download_binary("latest")
            .expect("download");
        assert_eq!(fs::read(path).expect("read the cache"), body);
        assert_eq!(
            harness.downloader.cached_release().as_deref(),
            Some("v0.2.0")
        );
    }

    #[test]
    fn a_served_checksum_contradicting_the_pin_is_refused() {
        let body = b"the release the origin serves";
        let harness = harness(
            "contradicted",
            release_routes("v0.1.0", body, &digest_of(body)),
            pin_table("v0.1.0", &"cd".repeat(32)),
        );
        let downloader = &harness.downloader;
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.5".to_owned(),
                sha256: digest_of(b"the cached binary"),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");
        fs::write(downloader.binary_path(), b"the cached binary").expect("place a cache");

        let error = downloader.download_binary("v0.1.0").expect_err("refusal");
        assert!(
            matches!(&error, Error::ChecksumMismatch(message) if message.contains("opensysml pins")),
            "{error}"
        );
        assert_eq!(
            fs::read(downloader.binary_path()).expect("read the cache"),
            b"the cached binary"
        );
        assert_eq!(downloader.cached_release().as_deref(), Some("v0.0.5"));
        assert!(!temporaries_left(&downloader.cache_dir));
    }

    #[test]
    fn a_tampered_download_is_refused_and_leaves_the_cache_alone() {
        // Pin and sidecar agree; the body served is not the binary they name.
        let promised = digest_of(b"the release that was pinned");
        let harness = harness(
            "tampered",
            release_routes("v0.1.0", b"truncated", &promised),
            pin_table("v0.1.0", &promised),
        );
        let downloader = &harness.downloader;
        fs::write(downloader.binary_path(), b"the cached binary").expect("place a cache");

        let error = downloader.download_binary("v0.1.0").expect_err("refusal");
        assert!(
            matches!(&error, Error::ChecksumMismatch(message) if message.contains("hashes to")),
            "{error}"
        );
        assert_eq!(
            fs::read(downloader.binary_path()).expect("read the cache"),
            b"the cached binary"
        );
        assert!(!temporaries_left(&downloader.cache_dir));
    }

    #[test]
    fn an_unpinned_release_is_refused_naming_the_gap() {
        let body = b"an unpinned release binary";
        let harness = harness(
            "unpinned",
            release_routes("v9.9.9", body, &digest_of(body)),
            PinTable::new(),
        );

        let error = harness
            .downloader
            .download_binary("v9.9.9")
            .expect_err("refusal");
        let Error::UnpinnedRelease(message) = &error else {
            panic!("expected an unpinned refusal, got {error}");
        };
        assert!(message.contains("pins no SHA-256 digest"), "{message}");
        assert!(message.contains("v9.9.9"), "{message}");
        assert!(
            message.contains("does not verify the signed SHA256SUMS.txt manifest"),
            "{message}"
        );
        assert!(message.contains(ALLOW_UNPINNED_ENV), "{message}");
        assert!(!harness.downloader.binary_path().exists());
    }

    #[test]
    fn an_unpinned_release_is_accepted_with_a_warning_when_opted_in() {
        for opt_in in ["1", REPO] {
            let body = b"an unpinned release binary";
            let digest = digest_of(body);
            let mut harness = harness(
                "unpinned-allowed",
                release_routes("v9.9.9", body, &digest),
                PinTable::new(),
            );
            harness.downloader.allow_unpinned = Some(opt_in.to_owned());

            let path = harness
                .downloader
                .download_binary("v9.9.9")
                .expect("download");
            assert_eq!(fs::read(path).expect("read the cache"), body);
            let warnings = harness.downloader.warnings();
            assert!(
                warnings
                    .iter()
                    .any(|warning| warning.contains("pins no digest")
                        && warning.contains("not a compromised release")),
                "{warnings:?}"
            );
        }
    }

    #[test]
    fn opting_in_for_one_repository_is_not_opting_in_for_another() {
        assert!(!unpinned_allowed(Some("a-fork/OpenSysML"), REPO));
        assert!(unpinned_allowed(
            Some("a-fork/OpenSysML"),
            "a-fork/OpenSysML"
        ));
        assert!(unpinned_allowed(
            Some("a-fork/OpenSysML, Open-MBEE/OpenSysML"),
            REPO
        ));
        assert!(unpinned_allowed(Some("1"), REPO));
        assert!(!unpinned_allowed(Some("0"), REPO));
        assert!(!unpinned_allowed(Some(""), REPO));
        assert!(!unpinned_allowed(None, REPO));
    }

    #[test]
    fn a_stale_cache_is_replaced_with_a_warning() {
        let body = b"the release asked for";
        let digest = digest_of(body);
        let harness = harness(
            "stale",
            release_routes("v0.0.7", body, &digest),
            pin_table("v0.0.7", &digest),
        );
        let downloader = &harness.downloader;
        let older = b"the release before";
        fs::write(downloader.binary_path(), older).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.5".to_owned(),
                sha256: digest_of(older),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        let reason = downloader
            .stale_cache_reason(Some("v0.0.7"))
            .expect("a stale cache");
        assert!(
            reason.contains("v0.0.5") && reason.contains("v0.0.7"),
            "{reason}"
        );

        let path = downloader
            .ensure_binary(Some("v0.0.7"))
            .expect("replacement");
        assert_eq!(fs::read(path).expect("read the cache"), body);
        assert_eq!(downloader.cached_release().as_deref(), Some("v0.0.7"));
        let warnings = downloader.warnings();
        assert!(
            warnings
                .iter()
                .any(|warning| warning.contains("replacing the cached sysml-grpc")),
            "{warnings:?}"
        );
    }

    #[test]
    fn a_cache_survives_a_replacement_that_cannot_be_downloaded() {
        // The fixture origin publishes v0.0.7 only, so v0.0.9 is a 404.
        let body = b"the release published";
        let digest = digest_of(body);
        let harness = harness(
            "kept",
            release_routes("v0.0.7", body, &digest),
            pin_table("v0.0.7", &digest),
        );
        let downloader = &harness.downloader;
        let cached = b"the working cache";
        fs::write(downloader.binary_path(), cached).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.5".to_owned(),
                sha256: digest_of(cached),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        let path = downloader.ensure_binary(Some("v0.0.9")).expect("the cache");
        assert_eq!(fs::read(path).expect("read the cache"), cached);
        assert_eq!(downloader.cached_release().as_deref(), Some("v0.0.5"));
        let warnings = downloader.warnings();
        assert!(
            warnings
                .iter()
                .any(|warning| warning.contains("keeping the cached sysml-grpc")),
            "{warnings:?}"
        );
    }

    #[test]
    fn a_cache_holding_the_release_asked_for_is_reused() {
        let body = b"the release asked for";
        let digest = digest_of(body);
        let harness = harness(
            "reused",
            release_routes("v0.0.7", body, &digest),
            pin_table("v0.0.7", &digest),
        );
        let downloader = &harness.downloader;
        fs::write(downloader.binary_path(), body).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.7".to_owned(),
                sha256: digest.clone(),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        assert!(downloader.stale_cache_reason(Some("v0.0.7")).is_none());
        assert!(downloader.stale_cache_reason(None).is_none());
        assert_eq!(
            downloader.ensure_binary(Some("v0.0.7")).expect("the cache"),
            linked(downloader, body)
        );
        assert!(downloader.warnings().is_empty());
    }

    #[test]
    fn a_binary_swapped_under_a_recorded_name_is_not_that_release() {
        let harness = harness("swapped", HashMap::new(), PinTable::new());
        let downloader = &harness.downloader;
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.7".to_owned(),
                sha256: digest_of(b"downloaded"),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");
        fs::write(downloader.binary_path(), b"something else entirely").expect("swap it");

        assert!(downloader.cached_release().is_none());
        assert!(downloader
            .stale_cache_reason(Some("v0.0.7"))
            .expect("stale")
            .contains("cannot be told"));
    }

    #[test]
    fn a_cache_from_another_repository_does_not_answer_for_this_one() {
        let mut harness = harness("fork", HashMap::new(), PinTable::new());
        harness.downloader.repo = "someone/fork".to_owned();
        let downloader = &harness.downloader;
        let body = b"the other repository's build";
        fs::write(downloader.binary_path(), body).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.7".to_owned(),
                sha256: digest_of(body),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        assert!(downloader.cached_release().is_none());
        let reason = downloader
            .stale_cache_reason(Some("v0.0.7"))
            .expect("stale");
        assert!(
            reason.contains("downloaded from Open-MBEE/OpenSysML"),
            "{reason}"
        );
        assert!(reason.contains("someone/fork"), "{reason}");
    }

    #[test]
    fn an_unreachable_release_query_keeps_a_working_cache() {
        // The fixture origin answers nothing, so resolving 'latest' fails.
        let harness = harness("offline", HashMap::new(), PinTable::new());
        let downloader = &harness.downloader;
        let body = b"the working cache";
        fs::write(downloader.binary_path(), body).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.7".to_owned(),
                sha256: digest_of(body),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        assert!(downloader.stale_cache_reason(Some("latest")).is_none());
        assert_eq!(
            downloader.ensure_binary(Some("latest")).expect("the cache"),
            linked(downloader, body)
        );
    }

    #[test]
    fn no_cache_and_no_release_asked_for_is_an_error() {
        let harness = harness("nothing", HashMap::new(), PinTable::new());
        let error = harness.downloader.ensure_binary(None).expect_err("refusal");
        assert!(
            matches!(&error, Error::BinaryDownload(message)
                if message.contains("OPENSYSML_GRPC_VERSION")),
            "{error}"
        );
    }

    #[test]
    #[cfg(unix)]
    fn a_cache_replaced_before_the_failure_is_not_reported_as_kept() {
        // The metadata is written last; a directory in its place fails that write
        // after the binary is already replaced.
        let body = b"the release asked for";
        let digest = digest_of(body);
        let harness = harness(
            "post-install",
            release_routes("v0.0.7", body, &digest),
            pin_table("v0.0.7", &digest),
        );
        let downloader = &harness.downloader;
        let older = b"the release before";
        fs::write(downloader.binary_path(), older).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.5".to_owned(),
                sha256: digest_of(older),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");
        fs::remove_file(downloader.metadata_path()).expect("clear the record");
        fs::create_dir(downloader.metadata_path()).expect("block the record");

        let error = downloader
            .ensure_binary(Some("v0.0.7"))
            .expect_err("the replaced cache is not answered for");
        assert!(matches!(error, Error::Io(_)), "{error}");
        assert_eq!(
            fs::read(downloader.binary_path()).expect("read the cache"),
            body
        );
    }

    #[test]
    fn an_unpinned_release_is_refused_even_with_a_cache_to_fall_back_on() {
        let body = b"the unpinned release";
        let harness = harness(
            "unpinned-with-cache",
            release_routes("v0.3.0", body, &digest_of(body)),
            PinTable::new(),
        );
        let downloader = &harness.downloader;
        let older = b"the release before";
        fs::write(downloader.binary_path(), older).expect("place a cache");
        downloader
            .write_metadata(&CacheMetadata {
                version: "v0.0.5".to_owned(),
                sha256: digest_of(older),
                repo: REPO.to_owned(),
            })
            .expect("record a cache");

        let error = downloader
            .ensure_binary(Some("v0.3.0"))
            .expect_err("an unverifiable release is not answered from the cache");
        assert!(matches!(error, Error::UnpinnedRelease(_)), "{error}");
        assert_eq!(
            fs::read(downloader.binary_path()).expect("read the cache"),
            older
        );
    }

    #[test]
    fn a_repository_or_tag_that_could_reshape_a_url_is_refused() {
        for repo in [
            "Open-MBEE",
            "Open-MBEE/OpenSysML/extra",
            "Open-MBEE/../secrets",
            "Open-MBEE/Open SysML",
            "",
        ] {
            assert!(validate_repo(repo).is_err(), "{repo}");
        }
        assert!(validate_repo("Open-MBEE/OpenSysML").is_ok());
        assert!(validate_repo("a_fork.of/Open-SysML").is_ok());

        for version in ["", "../v0.0.5", "v0.0.5/../..", "v0.0.5 v0.0.6", "-v0.0.5"] {
            assert!(validate_version(version).is_err(), "{version:?}");
        }
        assert!(validate_version("v0.0.5").is_ok());
        assert!(validate_version("v1.0.0-rc.1+build.2").is_ok());

        let harness = harness("traversal", HashMap::new(), PinTable::new());
        let error = harness
            .downloader
            .download_binary("../../other/releases/download/v1")
            .expect_err("refusal");
        assert!(
            matches!(&error, Error::BinaryDownload(message) if message.contains("release tag")),
            "{error}"
        );
    }

    #[test]
    fn release_assets_are_named_for_the_platform() {
        assert_eq!(
            release_asset_name("linux", "x86_64").expect("linux/amd64"),
            "sysml-grpc-linux-amd64"
        );
        assert_eq!(
            release_asset_name("linux", "aarch64").expect("linux/arm64"),
            "sysml-grpc-linux-arm64"
        );
        assert_eq!(
            release_asset_name("macos", "x86_64").expect("darwin/amd64"),
            "sysml-grpc-darwin-amd64"
        );
        assert_eq!(
            release_asset_name("macos", "aarch64").expect("darwin/arm64"),
            "sysml-grpc-darwin-arm64"
        );
        assert_eq!(
            release_asset_name("windows", "x86_64").expect("windows/amd64"),
            "sysml-grpc-windows-amd64.exe"
        );
        for (os, arch) in [
            ("windows", "aarch64"),
            ("freebsd", "x86_64"),
            ("linux", "riscv64"),
        ] {
            let error = release_asset_name(os, arch).expect_err("unsupported");
            assert!(
                matches!(&error, Error::UnsupportedPlatform { os: reported, .. } if reported == os),
                "{error}"
            );
        }
    }

    #[test]
    fn every_shipped_pin_is_a_sha256_of_a_published_asset() {
        let published: Vec<String> = [
            ("linux", "x86_64"),
            ("linux", "aarch64"),
            ("macos", "x86_64"),
            ("macos", "aarch64"),
            ("windows", "x86_64"),
        ]
        .iter()
        .map(|(os, arch)| release_asset_name(os, arch).expect("a published platform"))
        .collect();

        let pins = shipped_pins();
        assert!(
            !pins.is_empty(),
            "no release is pinned, so every download is refused"
        );
        for (repo, releases) in pins {
            assert!(repo.contains('/'), "{repo}");
            for (version, assets) in releases {
                assert!(version.starts_with('v'), "{repo} {version}");
                let names: Vec<&String> = assets.keys().collect();
                assert_eq!(
                    names.len(),
                    published.len(),
                    "{repo} {version} does not pin every published platform"
                );
                for asset in &published {
                    let digest = assets
                        .get(asset)
                        .unwrap_or_else(|| panic!("{repo} {version} pins no {asset}"));
                    assert!(
                        digest.len() == 64
                            && digest
                                .chars()
                                .all(|c| c.is_ascii_hexdigit() && !c.is_ascii_uppercase()),
                        "{repo} {version} {asset}: {digest}"
                    );
                }
            }
        }
    }

    /// Whether another process can take the cache lock right now, or `None` when
    /// there is no probe to ask with.
    #[cfg(unix)]
    fn lock_is_free(path: &Path) -> Option<bool> {
        // The Python client takes exactly this lock, so the probe is also a check
        // that the two contend with one another.
        let probe = "import fcntl, os, sys\n\
                     fd = os.open(sys.argv[1], os.O_RDWR | os.O_CREAT, 0o600)\n\
                     try:\n\
                     \x20   fcntl.lockf(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)\n\
                     except OSError:\n\
                     \x20   sys.exit(1)\n\
                     sys.exit(0)\n";
        let status = std::process::Command::new("python3")
            .arg("-c")
            .arg(probe)
            .arg(path)
            .status()
            .ok()?;
        Some(status.success())
    }

    #[cfg(unix)]
    #[test]
    fn the_cache_lock_shuts_other_processes_out() {
        let harness = harness("locked", HashMap::new(), PinTable::new());
        let downloader = &harness.downloader;
        let path = lock_path(&downloader.cache_dir);
        assert_eq!(path, downloader.binary_path().with_extension("lock"));

        let held = downloader.hold_cache();
        let Some(free) = lock_is_free(&path) else {
            eprintln!("skipping the cross-process lock test: python3 is not installed");
            return;
        };
        assert!(!free, "another process took the lock while it was held");
        drop(held);
        assert_eq!(lock_is_free(&path), Some(true));
        assert!(downloader.warnings().is_empty());
    }

    #[test]
    fn an_installer_waits_for_the_cache_to_be_free() {
        let body = b"the release asked for";
        let digest = digest_of(body);
        let harness = harness(
            "waited",
            release_routes("v0.1.0", body, &digest),
            pin_table("v0.1.0", &digest),
        );
        let downloader = &harness.downloader;
        let binary_path = downloader.binary_path();

        let held = downloader.hold_cache();
        thread::scope(|scope| {
            let installing = scope.spawn(|| downloader.ensure_binary(Some("v0.1.0")));
            thread::sleep(Duration::from_millis(250));
            assert!(
                !binary_path.exists(),
                "a second installer replaced the cache while it was held"
            );
            drop(held);
            let path = installing
                .join()
                .expect("the installing thread")
                .expect("install");
            assert_eq!(fs::read(path).expect("read the link"), body);
        });
        assert_eq!(fs::read(&binary_path).expect("read the cache"), body);
    }

    #[test]
    fn what_a_caller_is_handed_is_not_replaced_by_a_later_install() {
        let first = b"the release installed first";
        let second = b"the release installed after it";
        let mut routes = release_routes("v0.1.0", first, &digest_of(first));
        routes.extend(release_routes("v0.2.0", second, &digest_of(second)));
        let mut pins = pin_table("v0.1.0", &digest_of(first));
        pins.get_mut(REPO).expect("the pinned repository").insert(
            "v0.2.0".to_owned(),
            pin_table("v0.2.0", &digest_of(second))[REPO]["v0.2.0"].clone(),
        );
        let harness = harness("stable", routes, pins);
        let downloader = &harness.downloader;

        let handed = downloader.ensure_binary(Some("v0.1.0")).expect("install");
        assert_eq!(handed, linked(downloader, first));
        assert_ne!(handed, downloader.binary_path());

        let replaced = downloader.ensure_binary(Some("v0.2.0")).expect("replace");
        assert_eq!(fs::read(&handed).expect("read the link"), first);
        assert_eq!(fs::read(replaced).expect("read the link"), second);
        assert_eq!(
            fs::read(downloader.binary_path()).expect("read the cache"),
            second
        );
        assert!(!temporaries_left(&downloader.cache_dir));
    }

    #[test]
    fn a_cache_reached_without_a_downloader_is_handed_over_by_digest() {
        let harness = harness("linked", HashMap::new(), PinTable::new());
        let downloader = &harness.downloader;
        let binary_path = downloader.binary_path();
        assert!(stable_cached_binary(&binary_path).is_none());

        let body = b"the cached binary";
        fs::write(&binary_path, body).expect("place a cache");
        let handed = stable_cached_binary(&binary_path).expect("the cache");
        assert_eq!(handed, linked(downloader, body));

        // Installed the way an install installs: written aside, then renamed over.
        let staged = binary_path.with_extension("next");
        fs::write(&staged, b"another release entirely").expect("stage a release");
        fs::rename(&staged, &binary_path).expect("replace the cache");
        assert_eq!(fs::read(handed).expect("read the link"), body);
    }
}
