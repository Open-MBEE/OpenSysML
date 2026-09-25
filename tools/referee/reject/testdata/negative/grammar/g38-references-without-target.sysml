// Invalid: References requires a referenced feature after ::> (SysML.xtext:413 References).
package G38ReferencesWithoutTarget {
    part def D {
        attribute a;
        attribute b ::> ;
    }
}
