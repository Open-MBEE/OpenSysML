package MultiLineLiterals {
    // STRING_VALUE and UNRESTRICTED_NAME admit a line break like any other
    // character, so a literal runs to its closing quote across lines.
    attribute s = "line one
    line two";
    attribute e = "
";
    attribute escaped = "a \"quoted\"
line and a \\ backslash";
    part def 'Two
Lines';
    doc /* a comment body spans lines too */
}
