package io.opensysml.pilot;

import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.PrintStream;
import java.io.Writer;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;
import org.eclipse.emf.common.util.TreeIterator;
import org.eclipse.emf.ecore.EObject;
import org.eclipse.emf.ecore.resource.Resource;
import org.omg.kerml.xtext.KerMLStandaloneSetup;
import org.omg.sysml.io.SysMLUtil;
import org.omg.sysml.lang.sysml.Element;
import org.omg.sysml.lang.sysml.Relationship;
import org.omg.sysml.util.SysMLLibraryUtil;
import org.omg.sysml.util.traversal.Traversal;
import org.omg.sysml.util.traversal.facade.impl.JsonElementProcessingFacade;
import org.omg.sysml.xtext.SysMLStandaloneSetup;
import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;

/**
 * Headless driver for the pilot's API element JSON export.
 *
 * <p>Every input file of one corpus root is read into a shared resource set with the standard
 * library, so cross-file imports resolve. Each input resource's root elements are then visited
 * by the same {@link JsonElementProcessingFacade} the pilot's {@code %export} magic uses
 * ({@code SysMLInteractive.export}): derived properties included, and the tree serialized with
 * {@code toJsonTree(true)}. One JSON document per input file is written under the output
 * directory, mirroring the file's path relative to the root:
 *
 * <pre>{"file": "...", "elements": [...], "keys": {"&lt;elementId&gt;": "&lt;key&gt;"}}</pre>
 *
 * {@code keys} resolves every {@code @id} the elements mention, library elements included, to a
 * stable key (see {@link #keyOf}), so a reader can compare references by key rather than by the
 * pilot's per-run ids.
 * Nulls are serialized as the facade emits them.
 */
public final class ExportJSON extends SysMLUtil {
    private static final PrintStream STDERR =
            new PrintStream(new FileOutputStream(FileDescriptor.err), true, StandardCharsets.UTF_8);
    private static final String KERML_EXTENSION = ".kerml";
    private static final String SYSML_EXTENSION = ".sysml";
    private static final String KERNEL_LIBRARIES_DIRECTORY = "Kernel Libraries";
    private static final String SYSTEMS_LIBRARY_DIRECTORY = "Systems Library";
    private static final String DOMAIN_LIBRARIES_DIRECTORY = "Domain Libraries";

    private static final class Input {
        private final Path file;
        private final Resource resource;

        private Input(Path file, Resource resource) {
            this.file = file;
            this.resource = resource;
        }
    }

    private final Path root;
    private final Path out;
    private final List<Input> inputs = new ArrayList<>();
    private final Map<Resource, String> resourceNames = new HashMap<>();
    private final Gson gson = new GsonBuilder().setPrettyPrinting().serializeNulls().create();
    private Map<String, String> elementKeys;
    private boolean hasErrors;

    private ExportJSON(Path root, Path out) {
        super();
        this.root = root;
        this.out = out;
        setVerbose(false);
    }

    /** Load the standard library the way the pilot's own interactive session does. */
    private void loadLibrary(Path library) {
        SysMLLibraryUtil.setModelLibraryDirectory(library.toString());
        readAll(library.resolve(KERNEL_LIBRARIES_DIRECTORY).toString(), false, KERML_EXTENSION);
        readAll(library.resolve(SYSTEMS_LIBRARY_DIRECTORY).toString(), false, SYSML_EXTENSION);
        readAll(library.resolve(DOMAIN_LIBRARIES_DIRECTORY).toString(), false, SYSML_EXTENSION);
    }

    /** Read every input file into the shared resource set and the Xtext index. */
    private void readInputs(List<Path> files) {
        for (Path file : files) {
            try {
                Resource resource = readResource(file.toString());
                addInputResource(resource);
                inputs.add(new Input(file, resource));
                resourceNames.put(resource, display(file));
            } catch (RuntimeException e) {
                fail(file, "read", e);
            }
        }
    }

    private void exportInputs() throws IOException {
        // Run the pilot's model transformation first, as its validation does before an export,
        // without adding implied relationships as elements; the traversal otherwise adds them
        // while it iterates.
        transformAll(false);
        for (Input input : inputs) {
            String rel = display(input.file);
            JsonElement elements;
            try {
                JsonElementProcessingFacade facade = new JsonElementProcessingFacade();
                facade.setIsIncludeDerived(true);
                facade.setTraversal(new Traversal(facade));
                for (EObject object : input.resource.getContents()) {
                    if (object instanceof Element) {
                        facade.getTraversal().visit((Element) object);
                    }
                }
                elements = facade.toJsonTree(true);
            } catch (RuntimeException e) {
                fail(input.file, "export", e);
                continue;
            }
            reportUnkeyed(rel, elements);
            JsonObject document = new JsonObject();
            document.addProperty("file", rel);
            document.add("elements", elements);
            document.add("keys", keys(elements));
            Path target = out.resolve(rel + ".json");
            Files.createDirectories(target.getParent());
            try (Writer writer = Files.newBufferedWriter(target, StandardCharsets.UTF_8)) {
                gson.toJson(document, writer);
                writer.write('\n');
            }
        }
    }

    /** Name on stderr the exported elements no key addresses, by metaclass. */
    private void reportUnkeyed(String rel, JsonElement tree) {
        if (!tree.isJsonArray()) {
            return;
        }
        Map<String, String> table = elementKeys();
        TreeMap<String, Integer> counts = new TreeMap<>();
        for (JsonElement item : (JsonArray) tree) {
            JsonElement payload = item.isJsonObject() ? item.getAsJsonObject().get("payload") : null;
            if (payload == null || !payload.isJsonObject()) {
                continue;
            }
            JsonObject object = payload.getAsJsonObject();
            String id = object.has("@id") ? object.get("@id").getAsString() : null;
            if (id != null && !table.containsKey(id)) {
                String type = object.has("@type") ? object.get("@type").getAsString() : "?";
                counts.merge(type, 1, Integer::sum);
            }
        }
        if (!counts.isEmpty()) {
            STDERR.println(rel + ": unkeyed: " + counts);
        }
    }

    /** The stable key of every element the tree refers to by id, sorted by id. */
    private JsonObject keys(JsonElement tree) {
        Map<String, String> table = elementKeys();
        TreeMap<String, String> found = new TreeMap<>();
        collectIds(tree, table, found);
        JsonObject keys = new JsonObject();
        for (Map.Entry<String, String> entry : found.entrySet()) {
            keys.addProperty(entry.getKey(), entry.getValue());
        }
        return keys;
    }

    private static void collectIds(JsonElement node, Map<String, String> table, Map<String, String> found) {
        if (node.isJsonObject()) {
            JsonObject object = node.getAsJsonObject();
            for (Map.Entry<String, JsonElement> member : object.entrySet()) {
                if (member.getKey().equals("@id") && member.getValue().isJsonPrimitive()) {
                    String id = member.getValue().getAsString();
                    String name = table.get(id);
                    if (name != null) {
                        found.put(id, name);
                    }
                } else {
                    collectIds(member.getValue(), table, found);
                }
            }
        } else if (node.isJsonArray()) {
            for (JsonElement item : (JsonArray) node) {
                collectIds(item, table, found);
            }
        }
    }

    /** Element id to stable key over every resource loaded, library included. */
    private Map<String, String> elementKeys() {
        if (elementKeys == null) {
            elementKeys = new HashMap<>();
            Map<Element, String> memo = new HashMap<>();
            for (Resource resource : new ArrayList<>(getResourceSet().getResources())) {
                TreeIterator<EObject> contents = resource.getAllContents();
                while (contents.hasNext()) {
                    EObject object = contents.next();
                    if (object instanceof Element) {
                        Element element = (Element) object;
                        String key = keyOf(element, memo);
                        if (key != null && element.getElementId() != null) {
                            elementKeys.put(element.getElementId(), key);
                        }
                    }
                }
            }
        }
        return elementKeys;
    }

    /**
     * A named element's key is its qualified name. An unnamed one is addressed from its owner
     * (a relationship's owningRelatedElement): {@code <ownerKey>/<metaclass>[<i>]}, i counting
     * the owner's owned relationships (for a relationship) or owned elements (otherwise) of the
     * same metaclass that precede it. A document's root elements are
     * {@code /<file>/<metaclass>#<i>}.
     */
    private String keyOf(Element element, Map<Element, String> memo) {
        if (memo.containsKey(element)) {
            return memo.get(element);
        }
        String key = null;
        if (element.getQualifiedName() != null) {
            key = element.getQualifiedName();
        } else {
            Element owner = element instanceof Relationship
                    ? ((Relationship) element).getOwningRelatedElement()
                    : element.getOwner();
            if (owner == null) {
                // A document's root elements are addressed by the document; any other
                // ownerless unnamed element (a synthesized connector) stays unkeyed.
                Resource resource = element.eResource();
                int index = resource == null ? -1 : resource.getContents().indexOf(element);
                String file = resource == null ? null : resourceNames.get(resource);
                if (index >= 0 && file != null) {
                    int ordinal = 0;
                    for (EObject sibling : resource.getContents().subList(0, index)) {
                        if (sibling.eClass() == element.eClass()) {
                            ordinal++;
                        }
                    }
                    key = "/" + file + "/" + element.eClass().getName() + "#" + ordinal;
                }
            } else {
                String ownerKey = keyOf(owner, memo);
                if (ownerKey != null) {
                    List<? extends Element> siblings = element instanceof Relationship
                            ? owner.getOwnedRelationship()
                            : owner.getOwnedElement();
                    int index = 0;
                    boolean found = false;
                    for (Element sibling : siblings) {
                        if (sibling == element) {
                            found = true;
                            break;
                        }
                        if (sibling.eClass() == element.eClass()) {
                            index++;
                        }
                    }
                    if (found) {
                        key = ownerKey + "/" + element.eClass().getName() + "[" + index + "]";
                    }
                }
            }
        }
        memo.put(element, key);
        return key;
    }

    private void fail(Path file, String stage, RuntimeException e) {
        String message = e.getMessage() == null ? e.getClass().getName() : e.getMessage();
        STDERR.println(display(file) + ": " + stage + ": " + message.replaceAll("\\s+", " ").trim());
        hasErrors = true;
    }

    private String display(Path file) {
        Path absolute = file.toAbsolutePath().normalize();
        if (root != null && absolute.startsWith(root)) {
            return root.relativize(absolute).toString().replace('\\', '/');
        }
        return absolute.getFileName().toString();
    }

    public static void main(String[] args) {
        try {
            Path library = null;
            Path root = null;
            Path out = null;
            List<Path> files = new ArrayList<>();
            for (int i = 0; i < args.length; i++) {
                switch (args[i]) {
                    case "--library":
                        library = value(args, ++i, "--library");
                        break;
                    case "--root":
                        root = value(args, ++i, "--root");
                        break;
                    case "--out":
                        out = value(args, ++i, "--out");
                        break;
                    default:
                        if (args[i].startsWith("--")) {
                            throw new IllegalArgumentException("unknown option: " + args[i]);
                        }
                        files.add(Paths.get(args[i]).toAbsolutePath().normalize());
                }
            }
            if (library == null || out == null) {
                STDERR.println("Usage: export-json --library <sysml.library> --out <dir> [--root <dir>] <file>...");
                System.exit(2);
            }
            if (files.isEmpty()) {
                STDERR.println("Warning: no input files");
                System.exit(0);
            }
            // The KerML injector reads .kerml inputs; registering SysML too reads .sysml ones
            // and the library's SysML parts.
            new KerMLStandaloneSetup().createInjectorAndDoEMFRegistration();
            SysMLStandaloneSetup.doSetup();

            ExportJSON instance = new ExportJSON(root, out);
            instance.loadLibrary(library);
            instance.readInputs(files);
            instance.exportInputs();
            System.exit(instance.hasErrors ? 1 : 0);
        } catch (Throwable e) {
            STDERR.println("Error: " + (e.getMessage() == null ? e.toString() : e.getMessage()));
            System.exit(3);
        }
    }

    private static Path value(String[] args, int index, String option) {
        if (index >= args.length || args[index].startsWith("--")) {
            throw new IllegalArgumentException(option + " requires a value");
        }
        return Paths.get(args[index]).toAbsolutePath().normalize();
    }
}
