package org.openmbee.opensysml.syson.run;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;

public record RunDocument(RunDocumentForm form, String content, String fileName) {
    private static final int MAX_FILE_NAME = 255;
    private static final int TAG_BYTES = 8;
    private static final Set<String> DEVICE_NAMES = Set.of("CON", "PRN", "AUX", "NUL",
            "COM0", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "COM¹", "COM²", "COM³",
            "LPT0", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9", "LPT¹", "LPT²", "LPT³");

    public static RunDocument of(RunDocumentForm form, String content, String target) {
        return new RunDocument(form, content, fileName(target, "." + form.extension()));
    }

    // The file a rendered set writes this document to, which its links to other documents name:
    // `::` as `-`, every byte outside ASCII letters, digits and `_` as `.XX`, a Windows device name
    // or leading `.` made safe, and a stem too long for a file name cut and tagged with its hash.
    public static String fileName(String qualifiedName, String extension) {
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

    // Segments of a qualified name, quotes removed; a quoted segment may hold `::`.
    static List<String> splitQualifiedName(String qualifiedName) {
        List<String> segments = new ArrayList<>();
        StringBuilder current = new StringBuilder();
        boolean quoted = false;
        for (int i = 0; i < qualifiedName.length(); i++) {
            char ch = qualifiedName.charAt(i);
            if (ch == '\'') {
                quoted = !quoted;
            } else if (!quoted && ch == ':' && i + 1 < qualifiedName.length() && qualifiedName.charAt(i + 1) == ':') {
                segments.add(current.toString());
                current.setLength(0);
                i++;
            } else {
                current.append(ch);
            }
        }
        segments.add(current.toString());
        return segments;
    }

    private static boolean isDeviceStem(String stem) {
        int dot = stem.indexOf('.');
        String name = (dot < 0 ? stem : stem.substring(0, dot)).replaceAll(" +$", "").toUpperCase();
        return DEVICE_NAMES.contains(name);
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
