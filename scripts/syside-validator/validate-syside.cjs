/*
 * Validate SysML v2 / KerML files with Sensmetry SysIDE (sysml-2ls), printing
 * GNU-format diagnostics on stderr so tools/referee/diff can read them the same way
 * it reads the two pilot validators. Provisioned by scripts/download-syside.sh.
 *
 * Usage: validate-syside --library <sysml.library> [--root <dir>] <file>...
 *
 * Exit status: 0 clean, 1 the batch has error diagnostics, 2 the tool failed.
 */
"use strict";

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

// SysIDE truncates nothing; a chevrotain parse error can carry kilobytes of
// expected-token dumps, which would swamp the report.
const MESSAGE_LIMIT = 200;

function usage(message) {
	process.stderr.write(`Error: ${message}\n`);
	process.stderr.write("Usage: validate-syside --library <sysml.library> [--root <dir>] <file>...\n");
	process.exit(2);
}

function parseArgs(argv) {
	const options = { library: "", root: "", files: [] };
	for (let i = 0; i < argv.length; i++) {
		const arg = argv[i];
		switch (arg) {
			case "--library":
			case "--root": {
				const value = argv[++i];
				if (!value) {
					usage(`${arg} needs a directory`);
				}
				options[arg === "--library" ? "library" : "root"] = path.resolve(value);
				break;
			}
			default:
				if (arg.startsWith("--")) {
					usage(`unknown option: ${arg}`);
				}
				options.files.push(path.resolve(arg));
		}
	}
	if (!options.library) {
		usage("--library is required");
	}
	if (options.files.length === 0) {
		usage("no input files");
	}
	if (!options.root) {
		options.root = process.cwd();
	}
	return options;
}

function requireSyside(home) {
	const server = path.join(home, "packages", "syside-languageserver", "lib");
	for (const entry of [path.join(server, "index.js"), path.join(server, "node", "index.js")]) {
		if (!fs.existsSync(entry)) {
			process.stderr.write(`Error: SysIDE is not built: ${entry} is missing\n`);
			process.exit(2);
		}
	}
	return {
		languageserver: require(path.join(server, "index.js")),
		node: require(path.join(server, "node", "index.js")),
		URI: require(path.join(home, "packages", "syside-languageserver", "node_modules", "vscode-uri")).URI,
	};
}

// SysIDE validates a workspace folder, and loads every model file under it. The
// batch is copied into a scratch folder so the workspace is exactly the files
// asked for, never a sibling corpus the caller excluded.
function stageWorkspace(root, files) {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), "syside-batch-"));
	const staged = [];
	try {
		for (const file of files) {
			const rel = path.relative(root, file);
			if (rel.startsWith("..") || path.isAbsolute(rel)) {
				throw new Error(`${file} is outside the workspace root ${root}`);
			}
			const target = path.join(dir, rel);
			fs.mkdirSync(path.dirname(target), { recursive: true });
			fs.copyFileSync(file, target);
			staged.push({ rel, target });
		}
	} catch (error) {
		fs.rmSync(dir, { recursive: true, force: true });
		process.stderr.write(`Error: ${error?.message ?? error}\n`);
		process.exit(2);
	}
	return { dir, staged };
}

const SEVERITIES = ["error", "warning", "info", "info"];

function format(rel, diagnostic) {
	const severity = SEVERITIES[(diagnostic.severity ?? 1) - 1] ?? "info";
	const code = diagnostic.code ? `[${diagnostic.code}] ` : "";
	let message = String(diagnostic.message ?? "").replace(/\s+/g, " ").trim();
	if (message.length > MESSAGE_LIMIT) {
		message = `${message.slice(0, MESSAGE_LIMIT)}...`;
	}
	const start = diagnostic.range?.start ?? { line: 0, character: 0 };
	return `${rel}:${start.line + 1}:${start.character + 1}: ${severity}: ${code}${message}\n`;
}

async function main() {
	const home = process.env.SYSIDE_HOME;
	if (!home) {
		usage("SYSIDE_HOME is not set");
	}
	const options = parseArgs(process.argv.slice(2));
	if (!fs.existsSync(options.library)) {
		process.stderr.write(`Error: SysML library not found at ${options.library}\n`);
		process.exit(2);
	}

	const syside = requireSyside(home);
	const { dir, staged } = stageWorkspace(options.root, options.files);
	let status;
	try {
		const services = syside.languageserver.createSysMLServices(syside.node.SysMLNodeFileSystem, {
			standardLibraryPath: options.library,
			standardLibrary: true,
			logStatistics: false,
		});
		const shared = services.shared;
		await shared.workspace.WorkspaceManager.initializeWorkspace([
			{ name: path.basename(dir), uri: syside.URI.file(dir).toString() },
		]);

		const documents = staged.map(({ rel, target }) => ({
			rel,
			document: shared.workspace.LangiumDocuments.getOrCreateDocument(syside.URI.file(target)),
		}));
		await shared.workspace.DocumentBuilder.build(
			documents.map(({ document }) => document),
			{ validationChecks: "all" }
		);

		let errors = 0;
		let out = "";
		for (const { rel, document } of documents) {
			for (const diagnostic of document.diagnostics ?? []) {
				if ((diagnostic.severity ?? 1) === 1) {
					errors++;
				}
				out += format(rel, diagnostic);
			}
		}
		process.stderr.write(out);
		status = errors > 0 ? 1 : 0;
	} finally {
		fs.rmSync(dir, { recursive: true, force: true });
	}
	// exit rather than return: the language server leaves handles that would keep
	// the event loop alive.
	process.exit(status);
}

main().catch((error) => {
	process.stderr.write(`Error: SysIDE failed: ${error?.stack ?? error}\n`);
	process.exit(2);
});
