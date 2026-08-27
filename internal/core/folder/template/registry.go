package template

import "errors"

var ErrNotFound = errors.New("template not found")

type Registry struct {
	templates map[string]Template
}

func NewRegistry(templates []Template) *Registry {
	items := make(map[string]Template, len(templates))

	for _, template := range templates {
		items[template.Key] = template
	}

	return &Registry{
		templates: items,
	}
}

func (r *Registry) Get(key string) (Template, error) {
	template, ok := r.templates[key]
	if !ok {
		return Template{}, ErrNotFound
	}

	return template, nil
}
