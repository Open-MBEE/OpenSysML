- **Fixed literal FMI variable names that collide with trajectory selectors.** A variable
  whose archive name is `fmi:series:y` (or another reserved `fmi:` spelling) now reads its
  scalar value at `stopTime` instead of being mistaken for a sample request; one classifier
  shared by validation, request construction and reply binding resolves the name.
