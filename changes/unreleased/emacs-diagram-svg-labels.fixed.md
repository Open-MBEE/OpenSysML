- The Emacs package's `sysml-diagram-image` draws the labels of a diagram: Mermaid is asked for
  labels as SVG text rather than HTML in `foreignObject`, which librsvg, the SVG library Emacs
  draws with, leaves blank, and to keep the spaces between their words and wrap them at 400
  pixels instead of mid-word at 200. The colons, semicolons, `#` and `"` the server escapes for
  Mermaid show as themselves, not as `&#58;` and the like. Mermaid is also asked for its classic
  look: the drop shadows of the neo look it defaults to took librsvg minutes to draw a large
  view, during which the image buffer stayed empty.
