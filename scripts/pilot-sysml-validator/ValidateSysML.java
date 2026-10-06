package io.opensysml.pilot;

import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Stream;

import org.eclipse.emf.common.util.URI;
import org.eclipse.emf.ecore.EPackage;
import org.eclipse.emf.ecore.resource.Resource;
import org.eclipse.xtext.diagnostics.Severity;
import org.eclipse.xtext.resource.IResourceServiceProvider;
import org.eclipse.xtext.util.CancelIndicator;
import org.eclipse.xtext.validation.CheckMode;
import org.eclipse.xtext.validation.IResourceValidator;
import org.eclipse.xtext.validation.Issue;
import org.omg.kerml.xtext.KerMLStandaloneSetup;
import org.omg.kerml.xtext.xmi.KerMLxStandaloneSetup;
import org.omg.sysml.io.SysMLUtil;
import org.omg.sysml.lang.sysml.SysMLPackage;
import org.omg.sysml.util.SysMLLibraryUtil;
import org.omg.sysml.xtext.SysMLStandaloneSetup;
import org.omg.sysml.xtext.xmi.SysMLxStandaloneSetup;

import com.google.inject.Injector;

/**
 * Loads .sysml and .kerml files into one resource set and validates each file with its own
 * language's pilot validator; .kerml files use KerMLValidator as in ValidateKerML.
 *
 * <p>This is a genuine batch load: every file is read before any is validated, unlike the
 * accumulating SysMLInteractive session the DeciSym CLI drives one file at a time.
 *
 * <p>Reports Xtext issues on stderr in GNU format: file:line:column: severity: message.
 */
public final class ValidateSysML extends SysMLUtil {

    private static final PrintStream STDERR =
            new PrintStream(new FileOutputStream(FileDescriptor.err), true, StandardCharsets.UTF_8);

    private static final String KERML_EXTENSION = ".kerml";
    private static final String SYSML_EXTENSION = ".sysml";

    private static final String LIBRARY_DIRECTORY_NAME = "sysml.library";
    private static final String KERNEL_LIBRARIES_DIRECTORY = "Kernel Libraries";
    private static final String SYSTEMS_LIBRARY_DIRECTORY = "Systems Library";
    private static final String DOMAIN_LIBRARIES_DIRECTORY = "Domain Libraries";

    private static final Pattern OBJECT_REFERENCE =
            Pattern.compile("@[0-9a-f]++\\{file:([^#{}]*+)#([^{}]*+)\\}");

    private final IResourceValidator validator;
    private final Path root;
    private final List<Input> inputs = new ArrayList<>();
    private boolean hasErrors = false;

    private static final class Input {
        private final Path file;
        private final Resource resource;

        private Input(Path file, Resource resource) {
            this.file = file;
            this.resource = resource;
        }
    }

    private ValidateSysML(IResourceValidator validator, Path root) {
        super();
        this.validator = validator;
        this.root = root;
        setVerbose(false);
    }

    /** Load the standard library the way the pilot's own interactive session does. */
    private void loadLibrary(Path library) {
        SysMLLibraryUtil.setModelLibraryDirectory(library.toString());
        readAll(library.resolve(KERNEL_LIBRARIES_DIRECTORY).toString(), false, KERML_EXTENSION);
        readAll(library.resolve(SYSTEMS_LIBRARY_DIRECTORY).toString(), false, SYSML_EXTENSION);
        readAll(library.resolve(DOMAIN_LIBRARIES_DIRECTORY).toString(), false, SYSML_EXTENSION);
    }

    /**
     * Load a library directory beyond the standard one, such as the OpenSysML libraries, the same
     * way: its files are resolved against but never validated, so a model that imports them is
     * compared on its own diagnostics.
     */
    private void loadExtensionLibrary(Path library) {
        readAll(library.toString(), false, KERML_EXTENSION);
        readAll(library.toString(), false, SYSML_EXTENSION);
    }

    /** Read every input file into the shared resource set and the Xtext index. */
    private void readInputs(List<Path> files) {
        for (Path file : files) {
            try {
                Resource resource = readResource(file.toString());
                addInputResource(resource);
                inputs.add(new Input(file, resource));
            } catch (RuntimeException e) {
                report(file, 1, 1, Severity.ERROR, message(e));
            }
        }
    }

    private void validateInputs() {
        for (Input input : inputs) {
            try {
                IResourceServiceProvider resourceServiceProvider =
                        IResourceServiceProvider.Registry.INSTANCE
                                .getResourceServiceProvider(input.resource.getURI());
                IResourceValidator resourceValidator = resourceServiceProvider == null
                        ? validator
                        : resourceServiceProvider.getResourceValidator();
                for (Issue issue : resourceValidator.validate(
                        input.resource, CheckMode.ALL, CancelIndicator.NullImpl)) {
                    int line = issue.getLineNumber() == null ? 1 : issue.getLineNumber();
                    int column = issue.getColumn() == null ? 1 : issue.getColumn();
                    report(input.file, line, column, issue.getSeverity(), issue.getMessage());
                }
            } catch (RuntimeException e) {
                report(input.file, 1, 1, Severity.ERROR, message(e));
            }
        }
    }

    /** Print one diagnostic in GNU format, collapsing embedded newlines. */
    private void report(Path file, int line, int column, Severity severity, String message) {
        String name = display(file);
        String text = message == null ? "" : normalize(message.replaceAll("\\s+", " ").trim());
        STDERR.println(name + ":" + line + ":" + column + ": "
                + severity.toString().toLowerCase() + ": " + text);
        if (severity == Severity.ERROR) {
            hasErrors = true;
        }
    }

    /** Drop identity hash codes and absolute URIs from EMF object references. */
    private String normalize(String message) {
        Matcher matcher = OBJECT_REFERENCE.matcher(message);
        StringBuilder result = new StringBuilder();
        while (matcher.find()) {
            String path = URI.createURI("file:" + matcher.group(1)).toFileString();
            String name = path == null ? matcher.group(1) : display(Paths.get(path));
            String replacement = "{" + name + "#" + matcher.group(2) + "}";
            matcher.appendReplacement(result, Matcher.quoteReplacement(replacement));
        }
        matcher.appendTail(result);
        return result.toString();
    }

    /** Root-relative path where possible, else a library- or name-relative one. */
    private String display(Path file) {
        if (root != null && file.startsWith(root)) {
            return root.relativize(file).toString();
        }
        int library = file.getNameCount();
        for (int i = 0; i < file.getNameCount(); i++) {
            if (file.getName(i).toString().equals(LIBRARY_DIRECTORY_NAME)) {
                library = i;
                break;
            }
        }
        if (library < file.getNameCount()) {
            return file.subpath(library, file.getNameCount()).toString();
        }
        return file.getFileName().toString();
    }

    private static String message(Throwable e) {
        String text = e.getMessage();
        return text == null ? e.getClass().getName() : text;
    }

    private static List<Path> collect(List<String> arguments) throws IOException {
        List<Path> files = new ArrayList<>();
        for (String argument : arguments) {
            Path path = Paths.get(argument).toAbsolutePath().normalize();
            if (Files.isDirectory(path)) {
                try (Stream<Path> walk = Files.walk(path)) {
                    walk.filter(Files::isRegularFile)
                            .filter(p -> p.toString().endsWith(SYSML_EXTENSION)
                                    || p.toString().endsWith(KERML_EXTENSION))
                            .sorted()
                            .forEach(files::add);
                }
            } else {
                files.add(path);
            }
        }
        return files;
    }

    private static void usage() {
        STDERR.println("usage: validate-sysml --library DIR [--extension-library DIR]..."
                + " [--root DIR] FILE|DIR...");
        STDERR.println("FILE may end in .sysml or .kerml; DIR is walked for both.");
        STDERR.println("--extension-library DIR is loaded like the standard library:"
                + " resolved against, never validated.");
    }

    public static void main(String[] args) {
        try {
            Path library = null;
            List<Path> extensions = new ArrayList<>();
            Path root = null;
            List<String> inputs = new ArrayList<>();
            int index = 0;
            while (index < args.length) {
                String argument = args[index++];
                switch (argument) {
                    case "--library" -> library = value(args, index++, "--library");
                    case "--extension-library" ->
                            extensions.add(value(args, index++, "--extension-library"));
                    case "--root" -> root = value(args, index++, "--root");
                    case "-h", "--help" -> {
                        usage();
                        System.exit(0);
                    }
                    default -> inputs.add(argument);
                }
            }

            if (library == null) {
                String property = System.getProperty(LIBRARY_DIRECTORY_NAME);
                library = property == null ? null : Paths.get(property).toAbsolutePath().normalize();
            }
            if (library == null || !Files.isDirectory(library)) {
                STDERR.println("Error: SysML library not found: " + library);
                usage();
                System.exit(2);
            }
            for (Path extension : extensions) {
                if (!Files.isDirectory(extension)) {
                    STDERR.println("Error: extension library not found: " + extension);
                    System.exit(2);
                }
            }
            if (inputs.isEmpty()) {
                usage();
                System.exit(2);
            }

            List<Path> files = collect(inputs);
            for (Path file : files) {
                if (!file.toString().endsWith(SYSML_EXTENSION)
                        && !file.toString().endsWith(KERML_EXTENSION)) {
                    STDERR.println("Error: File must have .sysml or .kerml extension: " + file);
                    System.exit(2);
                }
                if (!Files.isRegularFile(file)) {
                    STDERR.println("Error: File not found: " + file);
                    System.exit(2);
                }
            }
            if (files.isEmpty()) {
                STDERR.println("Warning: No .sysml or .kerml files found");
                System.exit(0);
            }

            // The same registration sequence SysMLInteractive.createInstance performs, so the
            // only difference from the interactive path is how resources are loaded.
            EPackage.Registry.INSTANCE.put(
                    "https://www.omg.org/spec/SysML/20250201", SysMLPackage.eINSTANCE);
            KerMLStandaloneSetup.doSetup();
            KerMLxStandaloneSetup.doSetup();
            SysMLxStandaloneSetup.doSetup();
            Injector injector = new SysMLStandaloneSetup().createInjectorAndDoEMFRegistration();
            IResourceValidator validator = injector.getInstance(IResourceValidator.class);

            ValidateSysML instance = new ValidateSysML(validator, root);
            instance.loadLibrary(library);
            for (Path extension : extensions) {
                instance.loadExtensionLibrary(extension);
            }
            instance.readInputs(files);
            instance.validateInputs();
            System.exit(instance.hasErrors ? 1 : 0);
        } catch (Throwable e) {
            STDERR.println("Error: " + message(e));
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
