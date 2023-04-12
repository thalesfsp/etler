// Package state provides the state of something.
package state

// State represents the state of a processor.
type State int

const (
	// Canceled is the state of a canceled processor.
	Canceled State = iota

	// Finished is the state of a finished processor.
	Finished

	// Paused is the state of a paused processor.
	Paused

	// Running is the state of a running processor.
	Running

	// Stopped is the state of a stopped processor.
	Stopped
)

// String returns the string representation of a state.
func (s State) String() string {
	switch s {
	case Canceled:
		return "Canceled"
	case Finished:
		return "Finished"
	case Paused:
		return "Paused"
	case Running:
		return "Running"
	case Stopped:
		return "Stopped"
	default:
		return "Invalid"
	}
}
