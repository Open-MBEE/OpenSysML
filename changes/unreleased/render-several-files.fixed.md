- **`sysml -render <view>` accepts several model files, loaded as one model.** It used to stop
  at the second file with `-render renders a view of one model; unexpected extra argument`, so
  a view that exposed elements a sibling file declares could only be rendered with `-render-all`
  or from the REPL. Every file named on the command line is now loaded together, as `-render-all`
  and `-render-document` already loaded theirs; `-render-form` and `-o` apply as before, and the
  `#tree` pseudo-view renders every file loaded. A single file renders exactly as it did.
