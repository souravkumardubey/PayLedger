package engine

type Locking struct{}

func NewLocking() *Locking {
	return &Locking{}
}
