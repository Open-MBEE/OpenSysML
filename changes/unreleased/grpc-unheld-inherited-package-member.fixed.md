- The gRPC object lookup reports an unknown name inside a package that a declaration inherits or
  reaches through its type as not found (`symbol not found: V::car::P::Nope`), as the REPL does.
  While another object was held and `V::car` was not, it answered `no instance of "V::car"`.
