//go:build js || wasip1

package route

func routeAll([]Shape, []Edge, *Box) ([]Route, error) { return nil, ErrUnavailable }
