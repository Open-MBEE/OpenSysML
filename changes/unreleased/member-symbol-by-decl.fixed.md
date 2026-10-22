- The runtime reads the member a declaration registered from its scope's declaration index
  instead of collecting every member of the scope and scanning for it, which the lookup of a
  nested requirement's redefining default did for each member of each instantiated type:
  instantiation and satisfaction allocate as they did before a nested requirement's subject
  was bound to its enclosing part.
