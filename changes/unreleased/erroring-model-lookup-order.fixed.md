- **`Model.find` on a model with errors keeps preferring the outermost symbol of a shared short
  name.** A feature whose name comes from an unresolved redefinition has no effective name for the
  service's `Query` to see, so the index alone could answer with a deeper symbol of the same name
  when such an outer one existed. The tree is now walked breadth-first, down to the depth of the
  index's best answer, before that answer is accepted; a model without errors is still one query
  and one fetch.
