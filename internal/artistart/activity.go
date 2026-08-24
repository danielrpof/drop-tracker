package artistart

// ActivityGate is a RED-phase placeholder: its methods exist so the package
// and activity_test.go compile, but Active always reports false regardless
// of any Begin() call, so every positive-activity test fails until GREEN
// implements the real counter.
type ActivityGate struct{}

// NewActivityGate returns a ready-to-use zero-count gate.
func NewActivityGate() *ActivityGate {
	return &ActivityGate{}
}

// Begin is a RED-phase placeholder that returns a no-op end func without
// incrementing any counter.
func (g *ActivityGate) Begin() (end func()) {
	return func() {}
}

// Active is a RED-phase placeholder that always reports false.
func (g *ActivityGate) Active() bool {
	return false
}
