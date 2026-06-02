package engine

type Idempotency struct{}

func NewIdempotency() *Idempotency {
	return &Idempotency{}
}
