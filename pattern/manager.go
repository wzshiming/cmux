package pattern

var std = newManager()

func Register(name string, pattern string) {
	std.Register(name, pattern)
}

func Get(name string) (string, bool) {
	return std.Get(name)
}

func List() []string {
	return std.List()
}

type manager struct {
	patterns map[string]string
}

func newManager() *manager {
	return &manager{
		patterns: map[string]string{},
	}
}

func (h *manager) Register(name string, pattern string) {
	h.patterns[name] = pattern
}

func (h *manager) Get(name string) (string, bool) {
	pattern, ok := h.patterns[name]
	return pattern, ok
}

func (h *manager) List() []string {
	list := make([]string, 0, len(h.patterns))
	for name := range h.patterns {
		list = append(list, name)
	}
	return list
}
