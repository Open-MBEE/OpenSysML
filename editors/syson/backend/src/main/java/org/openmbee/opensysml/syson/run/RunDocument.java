package org.openmbee.opensysml.syson.run;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;
import org.openmbee.opensysml.RenderedDocument;

public record RunDocument(RunDocumentForm form, String content, String fileName) {
    private static final int MAX_FILE_NAME = 255;
    private static final int TAG_BYTES = 8;
    private static final Set<String> DEVICE_NAMES = Set.of("CON", "PRN", "AUX", "NUL",
            "COM0", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "COM¹", "COM²", "COM³",
            "LPT0", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9", "LPT¹", "LPT²", "LPT³");

    /**
     * The rendered document under the file name the service reports (which carries the tag the service
     * adds when two documents' names collide), or, against a service that reports none, under the
     * name the service's rendered sets and links use.
     */
    public static RunDocument of(RunDocumentForm form, RenderedDocument rendered, String target) {
        String fileName = rendered.fileName().isEmpty()
                ? fileName(target, "." + form.extension())
                : rendered.fileName();
        return new RunDocument(form, rendered.content(), fileName);
    }

    // The file a rendered set writes this document to, which its links to other documents name:
    // `::` as `-`, every byte outside ASCII letters, digits and `_` as `.XX`, a Windows device name
    // or leading `.` made safe, and a stem too long for a file name cut and tagged with its hash.
    static String fileName(String qualifiedName, String extension) {
        List<String> segments = new ArrayList<>();
        for (String segment : splitQualifiedName(qualifiedName)) {
            StringBuilder encoded = new StringBuilder();
            for (byte b : segment.getBytes(StandardCharsets.UTF_8)) {
                char ch = (char) (b & 0xFF);
                boolean plain = ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_';
                if (plain) encoded.append(ch);
                else encoded.append('.').append(HexFormat.of().withUpperCase().toHexDigits(b));
            }
            segments.add(encoded.toString());
        }
        String stem = String.join("-", segments);
        if (isDeviceStem(stem)) {
            stem = "." + HexFormat.of().withUpperCase().toHexDigits((byte) stem.charAt(0)) + stem.substring(1);
        }
        if (stem.startsWith(".")) stem = "_" + stem;
        if (stem.length() + extension.length() > MAX_FILE_NAME) {
            String tag = "~" + HexFormat.of().formatHex(sha256(stem), 0, TAG_BYTES);
            stem = cut(stem, MAX_FILE_NAME - extension.length() - tag.length()) + tag;
        }
        return stem + extension;
    }

    // Segments of the raw qualified name the service splits, as a rendered set does: delimiting quotes
    // removed, escapes kept as the parser keeps them, and every `::`, a quoted one included, a separator.
    static List<String> splitQualifiedName(String qualifiedName) {
        StringBuilder raw = new StringBuilder();
        boolean quoted = false;
        int i = 0;
        while (i < qualifiedName.length()) {
            char ch = qualifiedName.charAt(i++);
            if (ch == '\'') {
                quoted = !quoted;
            } else if (quoted && ch == '\\' && i < qualifiedName.length()) {
                raw.append(ch).append(qualifiedName.charAt(i++));
            } else {
                raw.append(ch);
            }
        }
        return List.of(raw.toString().split("::", -1));
    }

    private static boolean isDeviceStem(String stem) {
        int dot = stem.indexOf('.');
        String head = dot < 0 ? stem : stem.substring(0, dot);
        int end = head.length();
        while (end > 0 && head.charAt(end - 1) == ' ') end--;
        return DEVICE_NAMES.contains(head.substring(0, end).toUpperCase());
    }

    // Cuts an ASCII stem to n bytes, keeping a `.XX` escape whole.
    private static String cut(String stem, int n) {
        n = Math.min(n, stem.length());
        int escape = stem.lastIndexOf('.', n - 1);
        if (escape >= 0 && escape > n - 3) n = escape;
        return stem.substring(0, n);
    }

    private static byte[] sha256(String text) {
        try {
            return MessageDigest.getInstance("SHA-256").digest(text.getBytes(StandardCharsets.UTF_8));
        } catch (NoSuchAlgorithmException exception) {
            throw new IllegalStateException(exception);
        }
    }
}
