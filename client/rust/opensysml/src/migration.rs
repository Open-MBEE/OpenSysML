//! Migrating a SysML v1 model — UML XMI, an Eclipse UML2 `.uml` file or a Cameo/MagicDraw
//! `.mdzip` archive — to SysML v2, accounting for every element.

use std::collections::BTreeMap;
use std::ffi::OsString;
use std::fmt;
use std::fs;
use std::io::{self, Write};
use std::path::{Component, Path, PathBuf};

use crate::error::Error;
use crate::wire;

/// The forms a SysML v1 model comes in: UML XMI, an Eclipse UML2 file or a Cameo/MagicDraw archive.
pub const V1_FORMATS: &[&str] = &["xmi", "uml", "mdzip"];

/// Why a SysML v1 model is refused by [`Connection::convert`](crate::Connection::convert), as the
/// `sysml` command words it.
pub const MIGRATED_NOT_CONVERTED: &str = "is a SysML v1 model, which is migrated, not converted: \
every element is mapped, approximated or left unmapped and reported element by element";

/// What the service says of a migration when it says nothing itself.
pub const MIGRATION_NOTICE: &str = "SysML v1 migration is experimental: the mapping covers \
structure, ports and connectors, requirements, constraints, instances and allocations, reports \
every element it approximates or leaves behind, and what it writes for a v1 element may change \
without a compatibility path; see docs/reference/sysml-v1-migration.md § Status";

/// The verdict of an element written as it was.
pub const VERDICT_MAPPED: &str = "mapped";
/// The verdict of an element written in the nearest form SysML v2 has.
pub const VERDICT_APPROXIMATED: &str = "approximated";
/// The verdict of an element left behind, with a note saying why.
pub const VERDICT_UNMAPPED: &str = "unmapped";
/// The verdict of an element deliberately not carried over, such as a diagram.
pub const VERDICT_SKIPPED: &str = "skipped";

/// Dangling symbolic links followed before a path is judged to loop, as the kernel's limit.
const MAX_SYMLINK_HOPS: usize = 40;

/// Whether `from_format` names a SysML v1 model, read as the service reads a format name: in
/// any case and padding.
pub fn is_v1(from_format: &str) -> bool {
    V1_FORMATS.contains(&from_format.trim().to_ascii_lowercase().as_str())
}

/// Whether a path's extension names a SysML v1 model: `.xmi`, `.uml` or `.mdzip`.
pub fn path_is_v1(path: impl AsRef<Path>) -> bool {
    path.as_ref()
        .extension()
        .and_then(|extension| extension.to_str())
        .is_some_and(|extension| V1_FORMATS.contains(&extension.to_ascii_lowercase().as_str()))
}

/// What to migrate: a file the service reads, or the file's bytes carried inline.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum MigrateSource {
    /// A file on the service's filesystem.
    File(PathBuf),
    /// The bytes of a v1 file; [`MigrateOptions::from_format`] says which form they are.
    Content(Vec<u8>),
}

/// A Cameo MTIP layout: diagram positions the migration lays the v2 model out with.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Layout {
    /// A layout file on the service's filesystem.
    Path(PathBuf),
    /// The layout file's content.
    Content(String),
}

/// How to migrate: the options `sysml -migrate` takes.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct MigrateOptions {
    /// The source's form, `xmi`, `uml` or `mdzip`; empty lets the service tell from the path.
    pub from_format: String,
    /// Ask for every element's verdict and the report text, not only the summary and counts.
    pub report: bool,
    /// Ask for the JSON index of the result snapshots the v1 tool stored.
    pub results: bool,
    /// A layout to place the v2 model's diagrams by.
    pub layout: Option<Layout>,
    /// The URL image references in the migrated model are written under.
    pub image_base_url: String,
    /// Refuse a model whose probabilities do not sum to one.
    pub strict: bool,
}

/// One SysML v1 element's verdict in a migration.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct MigrationEntry {
    /// The element's `xmi:id`.
    pub id: String,
    /// Its v1 metaclass, with its applied stereotypes.
    pub kind: String,
    /// Its qualified name in the v1 model.
    pub name: String,
    /// The v2 element it became, or what stands for it.
    pub target: String,
    /// `mapped`, `approximated`, `unmapped` or `skipped`.
    pub verdict: String,
    /// Why, when the verdict is not `mapped`.
    pub note: String,
}

/// The account a migration gives of itself: what became of every element.
///
/// The summary and the four counts come back with every migration; `entries` and `text` when
/// [`MigrateOptions::report`] asks for them.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct MigrationReport {
    /// The v1 file the model was read from.
    pub source: String,
    /// The tool that exported it, as the file names itself.
    pub exporter: String,
    /// The one-line summary `sysml -migrate` prints.
    pub summary: String,
    /// Elements written as they were.
    pub mapped: i32,
    /// Elements written in the nearest form SysML v2 has.
    pub approximated: i32,
    /// Elements left behind.
    pub unmapped: i32,
    /// Elements deliberately not carried over.
    pub skipped: i32,
    /// Every element's verdict, when asked for.
    pub entries: Vec<MigrationEntry>,
    /// The report `sysml -migration-report` writes, when asked for.
    pub text: String,
}

impl MigrationReport {
    /// The entries with `verdict`: `mapped`, `approximated`, `unmapped` or `skipped`.
    pub fn by_verdict(&self, verdict: &str) -> Vec<&MigrationEntry> {
        self.entries
            .iter()
            .filter(|entry| entry.verdict == verdict)
            .collect()
    }
}

/// A migrated model: the notation or Turtle written, with its report and image files.
#[derive(Clone, Debug)]
pub struct Migration {
    /// The migrated model, in `to_format`.
    pub content: String,
    /// The form read: `xmi`, `uml` or `mdzip`.
    pub from_format: String,
    /// The format written.
    pub to_format: String,
    /// What became of every element.
    pub report: MigrationReport,
    /// The JSON index of result snapshots, when asked for; else empty.
    pub results: String,
    /// Image files the model's diagrams embed, by the relative path `content` refers to them with.
    pub files: BTreeMap<String, Vec<u8>>,
    /// The v1 file this was migrated from, absolute, when the source was a path.
    pub source_path: Option<PathBuf>,
    /// Always true: the migration is experimental.
    pub experimental: bool,
    /// What is experimental about it, in the service's own wording.
    pub experimental_notice: String,
    wire: wire::MigrateResponse,
}

impl Migration {
    /// The Migrate response this was read from.
    pub fn wire(&self) -> &wire::MigrateResponse {
        &self.wire
    }

    /// Write the migrated model to `path` and its image files beside it, at their relative paths
    /// under `path`'s directory, as `sysml -migrate -o` writes them. Returns the path.
    ///
    /// Nothing is written until every destination is judged: a `path` naming the v1 model the
    /// migration came from, or an image that would land outside the model's directory — through
    /// `..`, an absolute path or a symbolic link — or through a link at its own path is refused
    /// with [`Error::Unwritable`]. Images are then written without following a link at their
    /// path (`O_NOFOLLOW` where the platform has it), so one put there after the check is not
    /// followed; a directory on the way replaced while the write is under way is not guarded
    /// against, as `sysml -migrate -o` does not either.
    pub fn write(&self, path: impl AsRef<Path>) -> Result<PathBuf, Error> {
        let path = path.as_ref();
        let files = self.destinations(path)?;
        fs::write(path, self.content.as_bytes())?;
        for (file, data) in files {
            if let Some(parent) = file.parent() {
                fs::create_dir_all(parent)?;
            }
            write_image(&file, data)?;
        }
        Ok(path.to_path_buf())
    }

    fn destinations(&self, path: &Path) -> Result<Vec<(PathBuf, &[u8])>, Error> {
        let source = self.source_path.as_deref().filter(|source| source.exists());
        if let Some(source) = source {
            if same_path(path, source)? {
                return Err(Error::Unwritable(format!(
                    "{} names the model being migrated; the v1 model would be replaced by its \
                     migration",
                    path.display()
                )));
            }
        }
        let absolute = normalize(&std::path::absolute(path)?);
        let base = absolute
            .parent()
            .map(Path::to_path_buf)
            .unwrap_or_else(|| absolute.clone());
        let landed_base = landing(&base)?;
        let mut files = Vec::with_capacity(self.files.len());
        for (name, data) in &self.files {
            let segments: Vec<&str> = name.split('/').collect();
            let malformed = segments
                .iter()
                .any(|segment| matches!(*segment, "" | "." | "..") || segment.contains('\\'));
            let file = segments
                .iter()
                .fold(base.clone(), |file, segment| file.join(segment));
            if malformed || !within(&base, &file) || !within(&landed_base, &landing(&file)?) {
                return Err(Error::Unwritable(format!(
                    "the migration's image {name} would land outside {}",
                    base.display()
                )));
            }
            for guarded in [Some(absolute.as_path()), source].into_iter().flatten() {
                if same_path(&file, guarded)? {
                    return Err(Error::Unwritable(format!(
                        "the migration's image {name} would replace {}",
                        guarded.display()
                    )));
                }
            }
            if fs::symlink_metadata(&file).is_ok_and(|meta| meta.file_type().is_symlink()) {
                return Err(Error::Unwritable(format!(
                    "the migration's image {name} would be written through a symbolic link at {}",
                    file.display()
                )));
            }
            files.push((file, data.as_slice()));
        }
        Ok(files)
    }
}

/// Writes an image, created or truncated in place and never through a link at its path.
#[cfg(unix)]
fn write_image(file: &Path, data: &[u8]) -> io::Result<()> {
    use std::os::unix::fs::OpenOptionsExt;
    let mut out = fs::OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .custom_flags(rustix::fs::OFlags::NOFOLLOW.bits() as i32)
        .open(file)?;
    out.write_all(data)
}

#[cfg(not(unix))]
fn write_image(file: &Path, data: &[u8]) -> io::Result<()> {
    fs::write(file, data)
}

impl fmt::Display for Migration {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.content)
    }
}

/// Whether `file` is strictly below `base`, both normalized.
fn within(base: &Path, file: &Path) -> bool {
    file.strip_prefix(base)
        .is_ok_and(|below| !below.as_os_str().is_empty())
}

/// `path` with `.` and `..` resolved lexically.
fn normalize(path: &Path) -> PathBuf {
    let mut normalized = PathBuf::new();
    for component in path.components() {
        match component {
            Component::CurDir => {}
            Component::ParentDir => {
                if matches!(
                    normalized.components().next_back(),
                    Some(Component::Normal(_))
                ) {
                    normalized.pop();
                }
            }
            other => normalized.push(other.as_os_str()),
        }
    }
    normalized
}

/// Where a path lands once every symbolic link on it is followed, existing or dangling: the
/// real path of its deepest existing ancestor, with the missing tail below it.
fn landing(path: &Path) -> Result<PathBuf, Error> {
    let mut head = normalize(&std::path::absolute(path)?);
    let mut tail: Vec<OsString> = Vec::new();
    let mut hops = 0;
    loop {
        if let Ok(real) = fs::canonicalize(&head) {
            return Ok(tail
                .iter()
                .rev()
                .fold(real, |landed, name| landed.join(name)));
        }
        if let Ok(target) = fs::read_link(&head) {
            hops += 1;
            if hops >= MAX_SYMLINK_HOPS {
                return Err(Error::Unwritable(format!(
                    "{}: too many levels of symbolic links",
                    path.display()
                )));
            }
            let from = head.parent().map(Path::to_path_buf).unwrap_or_default();
            head = normalize(&from.join(target));
            continue;
        }
        match (head.parent().map(Path::to_path_buf), head.file_name()) {
            (Some(parent), Some(name)) => {
                tail.push(name.to_owned());
                head = parent;
            }
            _ => {
                return Ok(tail
                    .iter()
                    .rev()
                    .fold(head, |landed, name| landed.join(name)))
            }
        }
    }
}

/// Whether two paths name one file: by identity where both exist, else by where they land.
fn same_path(a: &Path, b: &Path) -> Result<bool, Error> {
    #[cfg(unix)]
    {
        use std::os::unix::fs::MetadataExt;
        if let (Ok(meta_a), Ok(meta_b)) = (fs::metadata(a), fs::metadata(b)) {
            return Ok(meta_a.dev() == meta_b.dev() && meta_a.ino() == meta_b.ino());
        }
    }
    Ok(landing(a)? == landing(b)?)
}

pub(crate) fn request_of(
    to_format: &str,
    source: &MigrateSource,
    options: &MigrateOptions,
) -> wire::MigrateRequest {
    use wire::migrate_request::{Layout as WireLayout, Source};
    wire::MigrateRequest {
        from_format: options.from_format.clone(),
        to_format: to_format.to_owned(),
        report: options.report,
        results: options.results,
        image_base_url: options.image_base_url.clone(),
        strict: options.strict,
        source: Some(match source {
            MigrateSource::File(path) => Source::FilePath(path.to_string_lossy().into_owned()),
            MigrateSource::Content(bytes) => Source::Content(bytes.clone()),
        }),
        layout: options.layout.as_ref().map(|layout| match layout {
            Layout::Path(path) => WireLayout::LayoutPath(path.to_string_lossy().into_owned()),
            Layout::Content(content) => WireLayout::LayoutContent(content.clone()),
        }),
    }
}

pub(crate) fn report_of(report: Option<wire::MigrationReport>) -> MigrationReport {
    let Some(report) = report else {
        return MigrationReport::default();
    };
    MigrationReport {
        source: report.source,
        exporter: report.exporter,
        summary: report.summary,
        mapped: report.mapped,
        approximated: report.approximated,
        unmapped: report.unmapped,
        skipped: report.skipped,
        entries: report
            .entries
            .into_iter()
            .map(|entry| MigrationEntry {
                id: entry.id,
                kind: entry.kind,
                name: entry.name,
                target: entry.target,
                verdict: entry.verdict,
                note: entry.note,
            })
            .collect(),
        text: report.text,
    }
}

pub(crate) fn migration_of(
    response: wire::MigrateResponse,
    source_path: Option<PathBuf>,
) -> Result<Migration, Error> {
    if !response.error.is_empty() {
        return Err(Error::Migration {
            message: response.error,
        });
    }
    let wire = response.clone();
    let experimental_notice = if response.experimental_notice.is_empty() {
        MIGRATION_NOTICE.to_owned()
    } else {
        response.experimental_notice
    };
    Ok(Migration {
        content: response.content,
        from_format: response.from_format,
        to_format: response.to_format,
        report: report_of(response.report),
        results: response.results,
        files: response
            .files
            .into_iter()
            .map(|file| (file.path, file.content))
            .collect(),
        source_path,
        experimental: true,
        experimental_notice,
        wire,
    })
}

#[cfg(test)]
mod tests {
    use std::sync::atomic::{AtomicUsize, Ordering};

    use super::*;

    fn scratch(name: &str) -> PathBuf {
        static COUNTER: AtomicUsize = AtomicUsize::new(0);
        let dir = std::env::temp_dir().join(format!(
            "opensysml-migration-{name}-{}-{}",
            std::process::id(),
            COUNTER.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir_all(&dir).unwrap();
        dir
    }

    fn migration(files: &[(&str, &[u8])], source_path: Option<PathBuf>) -> Migration {
        migration_of(
            wire::MigrateResponse {
                content: "package Vehicle;".to_owned(),
                from_format: "xmi".to_owned(),
                to_format: "sysml".to_owned(),
                files: files
                    .iter()
                    .map(|(path, content)| wire::MigrationFile {
                        path: (*path).to_owned(),
                        content: content.to_vec(),
                    })
                    .collect(),
                ..Default::default()
            },
            source_path,
        )
        .unwrap()
    }

    #[test]
    fn v1_is_told_from_the_format_in_any_case_and_padding_or_from_the_extension() {
        for format in ["xmi", "UML", " mdzip "] {
            assert!(is_v1(format), "{format:?}");
        }
        for format in ["", "sysml", "ttl", "xmi2"] {
            assert!(!is_v1(format), "{format:?}");
        }
        assert!(path_is_v1("Model.mdzip"));
        assert!(path_is_v1("dir/Model.XMI"));
        assert!(!path_is_v1("Model.sysml"));
        assert!(!path_is_v1("xmi"));
    }

    #[test]
    fn a_refusal_is_an_error_and_a_notice_is_filled_in() {
        let refused = migration_of(
            wire::MigrateResponse {
                error: "cannot read".to_owned(),
                ..Default::default()
            },
            None,
        )
        .unwrap_err();
        assert!(matches!(refused, Error::Migration { message } if message == "cannot read"));
        let migrated = migration(&[], None);
        assert!(migrated.experimental);
        assert_eq!(migrated.experimental_notice, MIGRATION_NOTICE);
        assert_eq!(migrated.report, MigrationReport::default());
    }

    #[test]
    fn the_report_selects_entries_by_verdict() {
        let report = report_of(Some(wire::MigrationReport {
            mapped: 1,
            unmapped: 1,
            entries: vec![
                wire::MigrationEntry {
                    id: "a".to_owned(),
                    verdict: VERDICT_MAPPED.to_owned(),
                    ..Default::default()
                },
                wire::MigrationEntry {
                    id: "b".to_owned(),
                    verdict: VERDICT_UNMAPPED.to_owned(),
                    ..Default::default()
                },
            ],
            ..Default::default()
        }));
        assert_eq!(report.by_verdict(VERDICT_UNMAPPED).len(), 1);
        assert_eq!(report.by_verdict(VERDICT_UNMAPPED)[0].id, "b");
        assert!(report.by_verdict(VERDICT_SKIPPED).is_empty());
    }

    #[test]
    fn write_puts_images_beside_the_model_however_deep() {
        let dir = scratch("deep");
        let deep = (0..70)
            .map(|index| format!("d{index}"))
            .collect::<Vec<_>>()
            .join("/")
            + "/deep.png";
        let migrated = migration(&[("images/a.png", b"a"), (deep.as_str(), b"deep")], None);
        let path = dir.join("Vehicle.sysml");
        assert_eq!(migrated.write(&path).unwrap(), path);
        assert_eq!(fs::read_to_string(&path).unwrap(), "package Vehicle;");
        assert_eq!(fs::read(dir.join("images/a.png")).unwrap(), b"a");
        assert_eq!(fs::read(dir.join(&deep)).unwrap(), b"deep");
    }

    #[test]
    fn write_refuses_the_source_and_images_that_escape_writing_nothing() {
        let dir = scratch("guard");
        let source = dir.join("Vehicle.xmi");
        fs::write(&source, b"<xmi/>").unwrap();
        let relative = {
            let cwd = std::env::current_dir().unwrap();
            source.strip_prefix(&cwd).map(Path::to_path_buf).ok()
        };
        let migrated = migration(&[], Some(source.clone()));
        let refused = migrated.write(&source).unwrap_err();
        assert!(
            matches!(refused, Error::Unwritable(ref m) if m.contains("names the model being migrated")),
            "{refused}"
        );
        if let Some(relative) = relative {
            assert!(migrated.write(relative).is_err());
        }
        assert_eq!(fs::read(&source).unwrap(), b"<xmi/>");

        let path = dir.join("Vehicle.sysml");
        for escaping in [
            "../escaped.png",
            "images/../../escaped.png",
            "/tmp/escaped.png",
            "images//x.png",
            "images\\x.png",
            "Vehicle.sysml",
            "Vehicle.xmi",
        ] {
            let refused = migration(&[(escaping, b"png")], Some(source.clone()))
                .write(&path)
                .unwrap_err();
            assert!(
                matches!(refused, Error::Unwritable(_)),
                "{escaping}: {refused}"
            );
            assert!(!path.exists(), "{escaping} wrote the model");
        }
        assert!(!dir.parent().unwrap().join("escaped.png").exists());

        fs::rename(&source, source.with_extension("bak")).unwrap();
        assert!(
            migrated.write(&source).is_ok(),
            "a vacated source path protects nothing"
        );
    }

    #[cfg(unix)]
    #[test]
    fn write_refuses_an_image_whose_directory_links_outside() {
        let dir = scratch("link");
        let outside = scratch("outside");
        std::os::unix::fs::symlink(&outside, dir.join("images")).unwrap();
        fs::create_dir(dir.join("dangling")).unwrap();
        std::os::unix::fs::symlink(outside.join("missing"), dir.join("dangling/dir")).unwrap();
        std::os::unix::fs::symlink(outside.join("file.png"), dir.join("dangling/file.png"))
            .unwrap();
        let path = dir.join("Vehicle.sysml");
        for linked in [
            "images/escaped.png",
            "dangling/dir/escaped.png",
            "dangling/file.png",
        ] {
            let refused = migration(&[(linked, b"png")], None)
                .write(&path)
                .unwrap_err();
            assert!(
                matches!(refused, Error::Unwritable(ref m) if m.contains("would land outside")),
                "{linked}: {refused}"
            );
        }
        assert!(!path.exists());
        assert!(fs::read_dir(&outside).unwrap().next().is_none());
    }

    #[cfg(unix)]
    #[test]
    fn write_refuses_an_image_whose_path_is_a_link() {
        let dir = scratch("alias");
        fs::create_dir(dir.join("linked")).unwrap();
        std::os::unix::fs::symlink(dir.join("linked/real.png"), dir.join("linked/alias.png"))
            .unwrap();
        let path = dir.join("Vehicle.sysml");
        let refused = migration(&[("linked/alias.png", b"png")], None)
            .write(&path)
            .unwrap_err();
        assert!(
            matches!(refused, Error::Unwritable(ref m) if m.contains("through a symbolic link")),
            "{refused}"
        );
        assert!(!path.exists());
        assert!(!dir.join("linked/real.png").exists());
    }
}
