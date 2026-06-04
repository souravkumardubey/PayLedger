package engine

import "github.com/google/uuid"

func newID() string {
	return uuid.New().String()
}
