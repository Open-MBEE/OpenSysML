package Q {
attribute t = "a" +
"b";
attribute u = 1 +
2 +
3;
attribute v = 1
+ 2;
attribute w = max(
1,
2);
// a broken expression stays broken across a comment
attribute x = "a" + // why
"b";
part def Wheel {
doc /* a body ends its own declaration */
attribute diameter;
}
comment about Wheel /* and so does this one */
}
