//go:build js || wasip1

package route

func routeAll([]Shape, []Edge, *Box) ([][]Point, error) { return nil, ErrUnavailable }
