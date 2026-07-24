package utils

import "github.com/google/uuid"

func NewUUID() uuid.UUID {
	return uuid.New()
}

func NewUUIDString() string {
	return uuid.New().String()
}

func MustParse(s string) uuid.UUID {
	return uuid.MustParse(s)
}

func Parse(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
