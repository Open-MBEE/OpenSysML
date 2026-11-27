- A sequence rendering keeps a fork in declaration order between two sent messages: with
  `succession first a then b; succession first a then c; succession first b then d;` and messages
  sent from `b` and from `d`, `a then c` was drawn below `b then d` and the second message.
