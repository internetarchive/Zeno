package control

// Event represents events that are handled by the control package.
type Event int

const (
	// PauseEvent is sent when Zeno is paused.
	PauseEvent Event = iota
	// ResumeEvent is sent when Zeno is resumed.
	ResumeEvent
)
