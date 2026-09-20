package io.opensysml.fuml;

import java.io.File;
import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.AtomicMoveNotSupportedException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardCopyOption;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.IdentityHashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.concurrent.atomic.AtomicReference;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.zip.ZipEntry;
import java.util.zip.ZipFile;

import javax.xml.parsers.DocumentBuilder;
import javax.xml.parsers.DocumentBuilderFactory;
import javax.xml.parsers.ParserConfigurationException;

import org.apache.log4j.AppenderSkeleton;
import org.apache.log4j.ConsoleAppender;
import org.apache.log4j.Level;
import org.apache.log4j.Logger;
import org.apache.log4j.PatternLayout;
import org.apache.log4j.spi.LoggingEvent;
import org.modeldriven.fuml.Fuml;
import org.modeldriven.fuml.environment.Environment;
import org.modeldriven.fuml.environment.ExecutionEnvironment;
import org.w3c.dom.Document;
import org.w3c.dom.Element;
import org.w3c.dom.Node;
import org.xml.sax.SAXException;

import fuml.semantics.commonbehavior.ParameterValue;
import fuml.semantics.commonbehavior.ParameterValueList;
import fuml.semantics.simpleclassifiers.BooleanValue;
import fuml.semantics.simpleclassifiers.DataValue;
import fuml.semantics.simpleclassifiers.EnumerationValue;
import fuml.semantics.simpleclassifiers.FeatureValue;
import fuml.semantics.simpleclassifiers.FeatureValueList;
import fuml.semantics.simpleclassifiers.IntegerValue;
import fuml.semantics.simpleclassifiers.RealValue;
import fuml.semantics.simpleclassifiers.SignalInstance;
import fuml.semantics.simpleclassifiers.StringValue;
import fuml.semantics.simpleclassifiers.StructuredValue;
import fuml.semantics.simpleclassifiers.UnlimitedNaturalValue;
import fuml.semantics.structuredclassifiers.ExtensionalValue;
import fuml.semantics.structuredclassifiers.Link;
import fuml.semantics.structuredclassifiers.Object_;
import fuml.semantics.structuredclassifiers.Reference;
import fuml.semantics.values.Value;
import fuml.semantics.values.ValueList;
import fuml.syntax.classification.Classifier;
import fuml.syntax.classification.ClassifierList;
import fuml.syntax.classification.Parameter;
import fuml.syntax.commonbehavior.Behavior;

/**
 * Runs every packaged activity of the pinned fUML test models, selected by XMI id, through
 * the reference implementation and writes the record the Go referee consumes.
 * Usage: {@code --model FILE=URI... --jar JAR --ri-tag TAG --ri-commit SHA --out FILE}.
 */
public final class FumlExpected {
	private static final PrintStream STDERR = new PrintStream(new FileOutputStream(FileDescriptor.err), true,
			StandardCharsets.UTF_8);
	private static final String XMI_NS = "http://www.omg.org/spec/XMI/20131001";
	private static final long ACTIVITY_TIMEOUT_SECONDS = 120;
	private static final int VALUE_DEPTH = 4;
	private static final String EXECUTED = "executed";
	private static final String VALUE = "value";
	// The foundational model library the implementation loads is the copy inside its jar.
	private static final String LIBRARY_RESOURCE = "org/modeldriven/fuml/library/fUML_Library.xmi";

	// Locus-relative object identifiers are Java hash codes; they are aliased per
	// activity in order of first appearance so the record is byte-stable.
	private static final Pattern OBJECT_ID = Pattern.compile("\\b[0-9a-f]+#[0-9a-f]+\\b");

	private FumlExpected() {
	}

	/** One model file and the namespace URI it is loaded under. */
	private static final class ModelArg {
		final File file;
		final String uri;

		ModelArg(File file, String uri) {
			this.file = file;
			this.uri = uri;
		}
	}

	/** One activity as the model file declares it. */
	private static final class ActivityDecl {
		final String model;
		final String id;
		final String name;
		final String ownerKind;
		final String ownerName;
		/** Every node the activity owns, at any nesting depth, by name: the ids declared under it. */
		final Map<String, List<String>> nodeIds = new LinkedHashMap<>();

		ActivityDecl(String model, String id, String name, String ownerKind, String ownerName) {
			this.model = model;
			this.id = id;
			this.name = name;
			this.ownerKind = ownerKind;
			this.ownerName = ownerName;
		}

		boolean packaged() {
			return "packagedElement".equals(ownerKind);
		}
	}

	// The implementation announces an activity execution's end only as a debug line.
	private static final Pattern COMPLETED = Pattern.compile("\\[execute\\] Activity (.*) completed\\.");

	/** Captures the thread's {@code [event]} lines, plus each completion debug line as a Complete event. */
	private static final class EventAppender extends AppenderSkeleton {
		final AtomicReference<Thread> owner = new AtomicReference<>();
		final List<String> events = new ArrayList<>();

		@Override
		protected void append(LoggingEvent event) {
			if (Thread.currentThread() != owner.get()) {
				return;
			}
			// Compared by rank: commons-logging hands log4j its own Priority instances.
			String line = String.valueOf(event.getMessage());
			if (event.getLevel().toInt() == Level.DEBUG_INT) {
				Matcher m = COMPLETED.matcher(line);
				if (!m.matches()) {
					return;
				}
				line = "Complete activity=" + m.group(1);
			} else if (event.getLevel().toInt() != Level.INFO_INT) {
				return;
			}
			synchronized (events) {
				events.add(line);
			}
		}

		List<String> drain() {
			synchronized (events) {
				List<String> out = new ArrayList<>(events);
				events.clear();
				return out;
			}
		}

		@Override
		public void close() {
			// The events list is the only state; the driver drains it itself.
		}

		@Override
		public boolean requiresLayout() {
			return false;
		}
	}

	/** How one packaged activity's execution ended. */
	private enum Outcome {
		COMPLETED, FAILED, TIMED_OUT
	}

	/** Executes packaged activities one at a time on the thread the appender follows. */
	private static final class ActivityRunner {
		private final Environment environment;
		private final EventAppender events;
		private final Map<String, ActivityDecl> byName;
		private final ExecutorService executor = Executors.newSingleThreadExecutor(r -> {
			Thread t = new Thread(r, "fuml-activity");
			t.setDaemon(true);
			return t;
		});

		ActivityRunner(Environment environment, EventAppender events, Map<String, ActivityDecl> byName) {
			this.environment = environment;
			this.events = events;
			this.byName = byName;
		}

		Outcome run(ActivityDecl decl, Map<String, Object> rec) throws InterruptedException {
			Behavior behavior = behaviorOf(decl);
			// The JUnit suite clears the locus between tests; extents must not leak.
			environment.locus.extensionalValues.clear();
			events.drain();
			Map<String, String> aliases = new LinkedHashMap<>();
			Future<ParameterValueList> future = executor.submit(() -> {
				events.owner.set(Thread.currentThread());
				return new ExecutionEnvironment(environment).execute(behavior);
			});
			rec.put(EXECUTED, true);
			Outcome outcome = Outcome.COMPLETED;
			try {
				ParameterValueList outputs = future.get(ACTIVITY_TIMEOUT_SECONDS, TimeUnit.SECONDS);
				rec.put("parameters", parameters(behavior));
				rec.put("outputs", outputs(outputs, aliases));
			} catch (TimeoutException e) {
				outcome = Outcome.TIMED_OUT;
				rec.put("error", "timed out after " + ACTIVITY_TIMEOUT_SECONDS + "s");
				events.owner.set(null);
				report(decl.name + " timed out; no later activity is run");
				executor.shutdownNow();
			} catch (ExecutionException e) {
				outcome = Outcome.FAILED;
				Throwable cause = e.getCause() == null ? e : e.getCause();
				rec.put("error", cause.getClass().getName() + ": " + cause.getMessage());
				report(decl.name + " failed: " + cause);
			}
			rec.put("events", eventRecords(events.drain(), aliases, byName));
			return outcome;
		}

		private Behavior behaviorOf(ActivityDecl decl) {
			fuml.syntax.commonstructure.Element element = environment.findElementById(decl.id);
			if (!(element instanceof Behavior)) {
				fail("xmi id " + decl.id + " (" + decl.name + ") is not a Behavior in the loaded model: "
						+ (element == null ? "null" : element.getClass().getName()));
			}
			return (Behavior) element;
		}

		void shutdown() {
			executor.shutdownNow();
		}
		/** The behavior's parameters as the implementation loaded them. */
		private static List<Object> parameters(Behavior behavior) {
			List<Object> out = new ArrayList<>();
			for (Parameter p : behavior.ownedParameter) {
				Map<String, Object> m = new LinkedHashMap<>();
				m.put("name", p.name);
				m.put("direction", String.valueOf(p.direction));
				m.put("type", p.type == null ? null : p.type.name);
				m.put("lower", p.multiplicityElement.lower);
				m.put("upper", p.multiplicityElement.upper.naturalValue < 0 ? "*"
						: String.valueOf(p.multiplicityElement.upper.naturalValue));
				m.put("isOrdered", p.multiplicityElement.isOrdered);
				m.put("isUnique", p.multiplicityElement.isUnique);
				out.add(m);
			}
			return out;
		}
		private static List<Object> outputs(ParameterValueList outputs, Map<String, String> aliases) {
			List<Object> out = new ArrayList<>();
			if (outputs == null) {
				return out;
			}
			for (ParameterValue pv : outputs) {
				Map<String, Object> m = new LinkedHashMap<>();
				m.put("parameter", pv.parameter == null ? null : pv.parameter.name);
				m.put("values", values(pv.values, aliases, new IdentityHashMap<>(), VALUE_DEPTH));
				out.add(m);
			}
			return out;
		}
		/** Parses "Kind key=value ..." lines (a final "value=" takes the rest); adds the element's id where unique. */
		private static List<Object> eventRecords(List<String> lines, Map<String, String> aliases,
				Map<String, ActivityDecl> byName) {
			List<Object> out = new ArrayList<>();
			for (String line : lines) {
				Map<String, Object> m = new LinkedHashMap<>();
				int sp = line.indexOf(' ');
				String kind = sp < 0 ? line : line.substring(0, sp);
				m.put("kind", kind);
				String rest = sp < 0 ? "" : line.substring(sp + 1);
				int valueAt = rest.indexOf(" value=");
				String value = null;
				if (valueAt >= 0) {
					value = rest.substring(valueAt + " value=".length());
					rest = rest.substring(0, valueAt);
				} else if (rest.startsWith("value=")) {
					value = rest.substring("value=".length());
					rest = "";
				}
				String[] keys = { "activity=", "action=", "parameter=" };
				// Names may contain spaces, so each field runs to the next known key.
				int pos = 0;
				while (pos < rest.length()) {
					int keyEnd = rest.indexOf('=', pos);
					if (keyEnd < 0) {
						break;
					}
					String key = rest.substring(pos, keyEnd + 1);
					int next = rest.length();
					for (String k : keys) {
						int at = rest.indexOf(" " + k, keyEnd);
						if (at >= 0 && at < next) {
							next = at;
						}
					}
					m.put(key.substring(0, key.length() - 1), rest.substring(keyEnd + 1, next));
					pos = next + 1;
				}
				if ("Fire".equals(kind) || "Execute".equals(kind) || "Complete".equals(kind)) {
					String id = elementId(byName, (String) m.get("activity"), (String) m.get("action"));
					if (id != null) {
						m.put("id", id);
					}
				}
				if (value != null) {
					m.put(VALUE, alias(value, aliases));
				}
				out.add(m);
			}
			return out;
		}

	}

	public static void main(String[] args) throws Exception {
		List<ModelArg> models = new ArrayList<>();
		String jar = null;
		String riTag = null;
		String riCommit = null;
		String out = null;
		for (int i = 0; i < args.length; i += 2) {
			String a = args[i];
			if (i + 1 >= args.length) {
				usage("missing value for " + a);
			}
			String v = args[i + 1];
			switch (a) {
			case "--model": {
				int eq = v.indexOf('=');
				if (eq <= 0) {
					usage("--model wants FILE=URI, got " + v);
				}
				models.add(new ModelArg(new File(v.substring(0, eq)), v.substring(eq + 1)));
				break;
			}
			case "--jar":
				jar = v;
				break;
			case "--ri-tag":
				riTag = v;
				break;
			case "--ri-commit":
				riCommit = v;
				break;
			case "--out":
				out = v;
				break;
			default:
				usage("unknown option " + a);
			}
		}
		if (models.isEmpty() || jar == null || riTag == null || riCommit == null || out == null) {
			usage("--model, --jar, --ri-tag, --ri-commit and --out are all required");
		}
		for (ModelArg m : models) {
			if (!m.file.isFile()) {
				fail("model not found: " + m.file);
			}
		}
		if (!new File(jar).isFile()) {
			fail("jar not found: " + jar);
		}

		EventAppender events = configureLogging();

		Map<String, Object> provenance = new LinkedHashMap<>();
		provenance.put("riTag", riTag);
		provenance.put("riCommit", riCommit);
		provenance.put("jarDigest", sha256(Paths.get(jar)));
		provenance.put("libraryResource", LIBRARY_RESOURCE);
		provenance.put("libraryDigest", sha256InJar(jar, LIBRARY_RESOURCE));
		List<Object> modelProvenance = new ArrayList<>();
		List<ActivityDecl> activities = new ArrayList<>();
		Environment environment = Environment.getInstance();
		for (ModelArg m : models) {
			Map<String, Object> mp = new LinkedHashMap<>();
			mp.put("file", m.file.getName());
			mp.put("uri", m.uri);
			mp.put("digest", sha256(m.file.toPath()));
			modelProvenance.add(mp);
			activities.addAll(declaredActivities(m.file));
			Fuml.load(m.file, m.uri);
		}
		provenance.put("models", modelProvenance);
		Map<String, ActivityDecl> byName = activitiesByName(activities);

		int packaged = 0;
		int failed = 0;
		// A timed-out activity may still be running on the shared locus, so nothing runs after it.
		String timedOut = null;
		List<Object> records = new ArrayList<>();
		ActivityRunner runner = new ActivityRunner(environment, events, byName);
		try {
			for (ActivityDecl decl : activities) {
				Map<String, Object> rec = new LinkedHashMap<>();
				rec.put("model", decl.model);
				rec.put("id", decl.id);
				rec.put("name", decl.name);
				records.add(rec);
				String skipped = skipReason(decl, timedOut);
				if (skipped != null) {
					rec.put(EXECUTED, false);
					rec.put("skipped", skipped);
					continue;
				}
				packaged++;
				Outcome outcome = runner.run(decl, rec);
				if (outcome == Outcome.TIMED_OUT) {
					timedOut = decl.name;
				}
				if (outcome != Outcome.COMPLETED) {
					failed++;
				}
			}
		} finally {
			runner.shutdown();
		}

		Map<String, Object> document = new LinkedHashMap<>();
		document.put("meaning", "The fUML reference implementation's outputs and event trace per test activity,"
				+ " selected by XMI id; the referee compares against these and never runs Java itself.");
		document.put("provenance", provenance);
		document.put("activities", records);
		StringBuilder sb = new StringBuilder();
		Json.write(sb, document, 0);
		sb.append('\n');
		Path outPath = Paths.get(out).toAbsolutePath();
		Files.createDirectories(outPath.getParent());
		if (failed > 0) {
			// An incomplete record is not truth: keep the committed one and leave the
			// partial output beside it for diagnosis.
			Path partial = outPath.resolveSibling(outPath.getFileName() + ".failed");
			Files.write(partial, sb.toString().getBytes(StandardCharsets.UTF_8));
			fail(failed + " of " + packaged + " activities failed; " + out + " left unchanged, partial record at " + partial);
		}
		Path staged = Files.createTempFile(outPath.getParent(), outPath.getFileName() + ".", ".tmp");
		Files.write(staged, sb.toString().getBytes(StandardCharsets.UTF_8));
		move(staged, outPath);
		STDERR.println("Wrote " + out + ": " + activities.size() + " activities, " + packaged + " executed");
	}

	/** Why the activity is recorded without running, or null when it runs. */
	private static String skipReason(ActivityDecl decl, String timedOut) {
		if (!decl.packaged()) {
			return decl.ownerKind + " of " + decl.ownerName + ": runs only as part of its owner, as in the JUnit suite";
		}
		if (timedOut != null) {
			return "not run: " + timedOut + " timed out and may still be executing";
		}
		return null;
	}

	private static void move(Path from, Path to) throws IOException {
		try {
			Files.move(from, to, StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING);
		} catch (AtomicMoveNotSupportedException e) {
			Files.move(from, to, StandardCopyOption.REPLACE_EXISTING);
		}
	}

	/** Routes the implementation's event lines to the appender and its noise to stderr. */
	private static EventAppender configureLogging() {
		Logger root = Logger.getRootLogger();
		root.removeAllAppenders();
		root.setLevel(Level.WARN);
		root.addAppender(new ConsoleAppender(new PatternLayout("%-5p %c{1} %m%n"), ConsoleAppender.SYSTEM_ERR));
		EventAppender appender = new EventAppender();
		Logger debug = Logger.getLogger("fuml.Debug");
		debug.setLevel(Level.DEBUG);
		debug.setAdditivity(false);
		debug.addAppender(appender);
		return appender;
	}

	/** Every uml:Activity the file declares, in document order, with the element that owns it. */
	private static List<ActivityDecl> declaredActivities(File file)
			throws IOException, ParserConfigurationException, SAXException {
		DocumentBuilderFactory f = DocumentBuilderFactory.newInstance();
		f.setNamespaceAware(true);
		f.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);
		DocumentBuilder b = f.newDocumentBuilder();
		Document doc;
		try (InputStream in = Files.newInputStream(file.toPath())) {
			doc = b.parse(in);
		}
		List<ActivityDecl> out = new ArrayList<>();
		collectActivities(file.getName(), doc.getDocumentElement(), out);
		return out;
	}

	private static void collectActivities(String model, Element el, List<ActivityDecl> out) {
		// An href is a cross-reference into another model, not a declaration.
		if ("uml:Activity".equals(el.getAttributeNS(XMI_NS, "type")) && !el.hasAttribute("href")) {
			Node owner = el.getParentNode();
			String ownerName = owner instanceof Element ? ((Element) owner).getAttribute("name") : "";
			ActivityDecl decl = new ActivityDecl(model, el.getAttributeNS(XMI_NS, "id"), el.getAttribute("name"),
					el.getLocalName(), ownerName);
			collectNodes(el, decl.nodeIds);
			out.add(decl);
		}
		for (Node n = el.getFirstChild(); n != null; n = n.getNextSibling()) {
			if (n instanceof Element) {
				collectActivities(model, (Element) n, out);
			}
		}
	}

	/** Indexes the nodes under an activity, structured nodes and their contents included. */
	private static void collectNodes(Element el, Map<String, List<String>> nodeIds) {
		for (Node n = el.getFirstChild(); n != null; n = n.getNextSibling()) {
			// An activity an action owns declares its own nodes.
			if (n instanceof Element && !"uml:Activity".equals(((Element) n).getAttributeNS(XMI_NS, "type"))) {
				collectNode((Element) n, nodeIds);
			}
		}
	}

	private static void collectNode(Element child, Map<String, List<String>> nodeIds) {
		String role = child.getLocalName();
		if ("node".equals(role) || "structuredNode".equals(role)) {
			nodeIds.computeIfAbsent(child.getAttribute("name"), k -> new ArrayList<>())
					.add(child.getAttributeNS(XMI_NS, "id"));
		}
		collectNodes(child, nodeIds);
	}

	/** The declared activities by name; a name two activities share resolves nothing. */
	private static Map<String, ActivityDecl> activitiesByName(List<ActivityDecl> activities) {
		Map<String, ActivityDecl> byName = new LinkedHashMap<>();
		Set<String> shared = new HashSet<>();
		for (ActivityDecl decl : activities) {
			if (byName.put(decl.name, decl) != null) {
				shared.add(decl.name);
			}
		}
		byName.keySet().removeAll(shared);
		return byName;
	}

	/** The XMI id of the named activity, or of its one node with the action's name; null if shared. */
	private static String elementId(Map<String, ActivityDecl> byName, String activity, String action) {
		ActivityDecl decl = activity == null ? null : byName.get(activity);
		if (decl == null) {
			return null;
		}
		if (action == null) {
			return decl.id;
		}
		List<String> ids = decl.nodeIds.get(action);
		return ids != null && ids.size() == 1 ? ids.get(0) : null;
	}

	private static List<Object> values(ValueList values, Map<String, String> aliases,
			IdentityHashMap<Object, Boolean> visiting, int depth) {
		List<Object> out = new ArrayList<>();
		for (Value v : values) {
			out.add(value(v, aliases, visiting, depth));
		}
		return out;
	}

	/** A JSON shape per value kind; structured values recurse to a bounded depth. */
	private static Object value(Value v, Map<String, String> aliases, IdentityHashMap<Object, Boolean> visiting,
			int depth) {
		Map<String, Object> m = new LinkedHashMap<>();
		if (v instanceof IntegerValue) {
			m.put("kind", "Integer");
			m.put(VALUE, ((IntegerValue) v).value);
		} else if (v instanceof BooleanValue) {
			m.put("kind", "Boolean");
			m.put(VALUE, ((BooleanValue) v).value);
		} else if (v instanceof StringValue) {
			m.put("kind", "String");
			m.put(VALUE, ((StringValue) v).value);
		} else if (v instanceof RealValue) {
			m.put("kind", "Real");
			m.put(VALUE, Float.toString(((RealValue) v).value));
		} else if (v instanceof UnlimitedNaturalValue) {
			int n = ((UnlimitedNaturalValue) v).value.naturalValue;
			m.put("kind", "UnlimitedNatural");
			m.put(VALUE, n < 0 ? "*" : String.valueOf(n));
		} else if (v instanceof EnumerationValue) {
			EnumerationValue ev = (EnumerationValue) v;
			m.put("kind", "Enumeration");
			m.put("type", ev.type == null ? null : ev.type.name);
			m.put(VALUE, ev.literal == null ? null : ev.literal.name);
		} else if (v instanceof Reference) {
			Object_ referent = ((Reference) v).referent;
			m.put("kind", "Reference");
			m.put("referent", referent == null ? null : value(referent, aliases, visiting, depth));
		} else if (v instanceof StructuredValue) {
			structuredValue((StructuredValue) v, m, aliases, visiting, depth);
		} else {
			m.put("kind", v.getClass().getSimpleName());
			m.put(VALUE, alias(v.toString(), aliases));
		}
		return m;
	}

	private static void structuredValue(StructuredValue v, Map<String, Object> m, Map<String, String> aliases,
			IdentityHashMap<Object, Boolean> visiting, int depth) {
		m.put("kind", structuredKind(v));
		if (v instanceof ExtensionalValue) {
			m.put("id", alias(((ExtensionalValue) v).identifier, aliases));
		}
		m.put("types", typeNames(v.getTypes()));
		if (depth <= 0 || visiting.containsKey(v)) {
			m.put("truncated", true);
			return;
		}
		visiting.put(v, Boolean.TRUE);
		List<Object> features = new ArrayList<>();
		FeatureValueList fvs = v.getFeatureValues();
		for (FeatureValue fv : fvs) {
			Map<String, Object> fm = new LinkedHashMap<>();
			fm.put("feature", fv.feature == null ? null : fv.feature.name);
			fm.put("values", values(fv.values, aliases, visiting, depth - 1));
			features.add(fm);
		}
		visiting.remove(v);
		m.put("features", features);
	}

	private static String structuredKind(StructuredValue v) {
		if (v instanceof Link) {
			return "Link";
		}
		if (v instanceof SignalInstance) {
			return "Signal";
		}
		return v instanceof DataValue ? "DataValue" : "Object";
	}

	private static List<Object> typeNames(ClassifierList types) {
		List<Object> out = new ArrayList<>();
		for (Classifier c : types) {
			out.add(c.name);
		}
		return out;
	}

	private static String alias(String s, Map<String, String> aliases) {
		Matcher m = OBJECT_ID.matcher(s);
		StringBuilder sb = new StringBuilder();
		int last = 0;
		while (m.find()) {
			sb.append(s, last, m.start());
			sb.append(aliases.computeIfAbsent(m.group(), k -> "obj" + (aliases.size() + 1)));
			last = m.end();
		}
		sb.append(s.substring(last));
		return sb.toString();
	}

	private static String sha256(Path p) throws IOException {
		return sha256(Files.readAllBytes(p));
	}

	private static String sha256InJar(String jar, String entry) throws IOException {
		try (ZipFile zip = new ZipFile(jar)) {
			ZipEntry e = zip.getEntry(entry);
			if (e == null) {
				fail(jar + " has no entry " + entry);
			}
			try (InputStream in = zip.getInputStream(e)) {
				return sha256(in.readAllBytes());
			}
		}
	}

	private static String sha256(byte[] content) {
		try {
			MessageDigest md = MessageDigest.getInstance("SHA-256");
			byte[] sum = md.digest(content);
			StringBuilder sb = new StringBuilder();
			for (byte b : sum) {
				sb.append(String.format("%02x", b));
			}
			return sb.toString();
		} catch (NoSuchAlgorithmException e) {
			throw new IllegalStateException(e);
		}
	}

	private static void report(String message) {
		STDERR.println("error: " + message);
	}

	private static void usage(String message) {
		report(message);
		STDERR.println("usage: FumlExpected --model FILE=URI [--model FILE=URI]... --jar JAR"
				+ " --ri-tag TAG --ri-commit SHA --out FILE");
		System.exit(2);
	}

	private static void fail(String message) {
		report(message);
		System.exit(1);
	}

	/** A minimal JSON writer: two-space indentation, insertion-ordered maps, no dependencies. */
	private static final class Json {
		private Json() {
		}

		static void write(StringBuilder sb, Object o, int indent) {
			if (o == null) {
				sb.append("null");
			} else if (o instanceof String) {
				string(sb, (String) o);
			} else if (o instanceof Boolean || o instanceof Integer || o instanceof Long) {
				sb.append(o);
			} else if (o instanceof Map) {
				Map<?, ?> m = (Map<?, ?>) o;
				if (m.isEmpty()) {
					sb.append("{}");
					return;
				}
				sb.append("{\n");
				boolean first = true;
				for (Map.Entry<?, ?> e : m.entrySet()) {
					if (!first) {
						sb.append(",\n");
					}
					first = false;
					pad(sb, indent + 1);
					string(sb, String.valueOf(e.getKey()));
					sb.append(": ");
					write(sb, e.getValue(), indent + 1);
				}
				sb.append('\n');
				pad(sb, indent);
				sb.append('}');
			} else if (o instanceof List) {
				List<?> l = (List<?>) o;
				if (l.isEmpty()) {
					sb.append("[]");
					return;
				}
				sb.append("[\n");
				boolean first = true;
				for (Object e : l) {
					if (!first) {
						sb.append(",\n");
					}
					first = false;
					pad(sb, indent + 1);
					write(sb, e, indent + 1);
				}
				sb.append('\n');
				pad(sb, indent);
				sb.append(']');
			} else {
				string(sb, String.valueOf(o));
			}
		}

		private static void pad(StringBuilder sb, int n) {
			for (int i = 0; i < n; i++) {
				sb.append("  ");
			}
		}

		private static void string(StringBuilder sb, String s) {
			sb.append('"');
			for (int i = 0; i < s.length(); i++) {
				char c = s.charAt(i);
				switch (c) {
				case '"':
					sb.append("\\\"");
					break;
				case '\\':
					sb.append("\\\\");
					break;
				case '\n':
					sb.append("\\n");
					break;
				case '\r':
					sb.append("\\r");
					break;
				case '\t':
					sb.append("\\t");
					break;
				default:
					if (c < 0x20) {
						sb.append(String.format("\\u%04x", (int) c));
					} else {
						sb.append(c);
					}
				}
			}
			sb.append('"');
		}
	}
}
