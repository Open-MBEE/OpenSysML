- The pilot-rejection corpus and the type-tier tests pin that a `message` typed by an item
  definition (`message m : Note from a.emit to b.receive;`) is rejected with
  `A flow usage must be typed by flow definitions.`, in every spelling of the
  declaration, exactly as the `flow` form is: a message declaration is an abstract flow usage
  (SysML v2.1 Beta 2 §8.4.12.2), and `message m of Note` names the payload type rather than a
  typing. The constraint census records the message form under `validateFlowUsageType`.
