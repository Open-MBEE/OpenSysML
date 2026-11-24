- The Emacs package's `sysml-diagram-image` draws the labels of a diagram: Mermaid is asked for
  labels as SVG text rather than HTML in `foreignObject`, which librsvg, the SVG library Emacs
  draws with, leaves blank, and to keep the spaces between their words and wrap them at 400
  pixels instead of mid-word at 200. The colons, semicolons, `#` and `"` the server escapes for
  Mermaid show as themselves, not as `&#58;` and the like.
