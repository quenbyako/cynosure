package entities

type EventsReader[T any] interface {
	Synchronized() bool
	PendingEvents() []T
}

type pendingEvents[T any] []T

func (e *pendingEvents[T]) ClearEvents()       { *e = (*e)[:0:0] }
func (e *pendingEvents[T]) PendingEvents() []T { return *e }
func (e *pendingEvents[T]) Synchronized() bool { return len(*e) == 0 }
