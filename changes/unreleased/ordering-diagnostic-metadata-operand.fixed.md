- **An ordering operator over `x.metadata` is refused as a sequence, not as a metadata instance.**
  `tagged.metadata < 1` over `#Tag part tagged : Widget;` was reported as `operator '<' is not
  defined for an instance and an Integer; … an instance of the metadata def Tag is none`, but the
  left operand is no single instance: a metadata access expression evaluates to the element's
  metadata annotations followed by its reflective metaobject (KerML 1.0 §8.3.4.8.15, §8.4.4.9.7;
  `MetadataAccessEvaluation` returns `Metaobject[1..*]`), so `tagged.metadata` is a two-element
  sequence. It is now reported as `operator '<' is not defined for a sequence and an Integer;
  DataFunctions::'<' takes one DataValue per operand`, the same refusal as any other sequence. A
  single metadata instance selected with a meta-cast, `(tagged meta Tag) < 1`, still reports
  `an instance of the metadata def Tag is none`.
