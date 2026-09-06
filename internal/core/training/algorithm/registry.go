package algorithm

import "fmt"

type identity struct {
	key     string
	version int
}

type Registry struct{ algorithms map[identity]Algorithm }

func NewRegistry(algorithms ...Algorithm) (*Registry, error) {
	r := &Registry{algorithms: make(map[identity]Algorithm, len(algorithms))}
	for _, a := range algorithms {
		if a == nil || a.Key() == "" || a.Version() < 1 {
			return nil, fmt.Errorf("invalid algorithm registration")
		}
		id := identity{a.Key(), a.Version()}
		if _, ok := r.algorithms[id]; ok {
			return nil, fmt.Errorf("duplicate algorithm %s:v%d", id.key, id.version)
		}
		r.algorithms[id] = a
	}
	return r, nil
}

func (r *Registry) Get(key string, version int) (Algorithm, error) {
	if a, ok := r.algorithms[identity{key, version}]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("algorithm %s:v%d is not registered", key, version)
}
