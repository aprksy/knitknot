package types

import "sync"

// VerbRegistry stores semantic definitions for edge kinds
type VerbRegistry struct {
	mu    sync.RWMutex
	verbs map[string]Verb
}

func NewVerbRegistry() *VerbRegistry {
	vr := &VerbRegistry{
		verbs: make(map[string]Verb),
	}
	return vr
}

func (vr *VerbRegistry) Register(name string, def Verb) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.verbs[name] = def
}

func (vr *VerbRegistry) Lookup(name string) (Verb, bool) {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	v, ok := vr.verbs[name]
	return v, ok
}

func (vr *VerbRegistry) All() map[string]Verb {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	cp := make(map[string]Verb, len(vr.verbs))
	for k, v := range vr.verbs {
		cp[k] = v
	}
	return cp
}
