- The gRPC object lookup resolves a name a declaration inherits or reaches through its type, as the
  REPL does. While another object was held and `V::car` was not, it answered `no instance of
  "V::car"` for any name under `car`; it now reports an unknown name inside an inherited package as
  not found (`symbol not found: V::car::P::Nope`), names the inherited declaration that lacks an
  object (`no instance of "V::Car::engine"`), and reaches an object held under the declared name.
  The REPL and gRPC both read a quoted name holding `::` (`V::car::'drive::motor'`) as the one member
  it is rather than as a path through `drive`.
