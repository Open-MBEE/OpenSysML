// Invalid: A string admits only the escapes \b \t \n \f \r \" \' \\ (KerMLExpressions.xtext:514 STRING_VALUE).
package G75InvalidStringEscape {
    attribute b = "bad \q escape";
}
