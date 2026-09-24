package analytics

import "context"

// Emitter is a zero-value-safe publisher dependency. Configure it only at
// composition time, before serving requests; it is immutable thereafter.
type Emitter struct{ publisher Publisher }

func (e *Emitter) SetPublisher(p Publisher) {
	if p == nil {
		p = NoopPublisher{}
	}
	e.publisher = p
}
func (e *Emitter) Publish(ctx context.Context, event Event) {
	if e.publisher != nil {
		e.publisher.Publish(ctx, event)
	}
}
