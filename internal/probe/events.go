package probe

// Event is a probe lifecycle notification (Observer pattern).
type Event struct {
	Kind   string      `json:"kind"` // phase | step_start | step_done | done | error
	Step   int         `json:"step,omitempty"`
	Phase  string      `json:"phase,omitempty"`
	MS     float64     `json:"ms,omitempty"`
	URL    string      `json:"url,omitempty"`
	Result *StepResult `json:"result,omitempty"`
	Report *ProbeReport `json:"report,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// EventListener receives probe events (Observer).
type EventListener interface {
	OnEvent(Event)
}

// NoopEventListener is a Null Object.
type NoopEventListener struct{}

func (NoopEventListener) OnEvent(Event) {}

// MultiListener fans events out to many observers (Observer composite).
type MultiListener struct {
	listeners []EventListener
}

func NewMultiListener(listeners ...EventListener) *MultiListener {
	out := make([]EventListener, 0, len(listeners))
	for _, l := range listeners {
		if l != nil {
			out = append(out, l)
		}
	}
	return &MultiListener{listeners: out}
}

func (m *MultiListener) OnEvent(e Event) {
	for _, l := range m.listeners {
		l.OnEvent(e)
	}
}

// PhaseBridge adapts legacy PhaseListener to EventListener.
type PhaseBridge struct {
	Inner PhaseListener
}

func (b PhaseBridge) OnEvent(e Event) {
	if b.Inner == nil || e.Kind != "phase" {
		return
	}
	b.Inner.OnPhase(e.Phase, e.Step, e.MS)
}

// ChanListener pushes events onto a channel (used by WebSocket streaming).
type ChanListener struct {
	Ch chan Event
}

func NewChanListener(buffer int) *ChanListener {
	if buffer < 1 {
		buffer = 16
	}
	return &ChanListener{Ch: make(chan Event, buffer)}
}

func (c *ChanListener) OnEvent(e Event) {
	select {
	case c.Ch <- e:
	default:
		// Drop if consumer is slow; keep probe moving.
	}
}

func (c *ChanListener) Close() {
	close(c.Ch)
}
