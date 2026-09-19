// An enumeration literal is a value: an enum-typed default materializes as the
// identity of the literal it names, and compares equal to that literal alone.
package test {
    enum def Color { red; green; blue; }

    enum def Level { low; high; }

    part def Car {
        attribute c : Color = Color::red;
        attribute isRed = c == Color::red;
        attribute isGreen = c == Color::green;
        attribute notGreen = c != Color::green;
        attribute sameAsOtherEnum = c == Level::low;
        attribute sameLiteralTwice = Color::blue == Color::blue;
    }
}
