- **An inline `entry`, `do` or `exit action` body of a state that states successions now
  executes.** A body such as `do action ops { first start; then action a : A; then action b : B;
  then done; }` or `do action ops { action a : A; action b : B; first a then b; }` was rejected
  at instantiation with `statement not executable: … *ast.InitialNode in a body is not
  executable`, while the same body as a standalone `action def` ran. The body is now lowered to
  the same token flow a standalone action's body is and runs through the action executor, so
  successions, `first … then …`, guards, forks, joins, `then done` and action nodes with a flow
  of their own behave as they do in an action, and the attributes the body declares are the
  performance's own; a dangling or unstartable succession, or a node declaring `return`, is
  reported as a typed error before any node runs. A body stating no flow still runs its
  statements in declaration order.
