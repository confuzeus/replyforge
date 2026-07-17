package model

import (
	"fmt"

	"github.com/sqids/sqids-go"
)

type DisplayIDGenerator struct {
	s *sqids.Sqids
}

func NewDisplayIDGenerator() *DisplayIDGenerator {
	s, err := sqids.New(sqids.Options{
		MinLength: 6,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to create sqids instance: %v", err))
	}
	return &DisplayIDGenerator{s: s}
}

func (g *DisplayIDGenerator) Generate(id int64) (string, error) {
	encoded, err := g.s.Encode([]uint64{uint64(id)})
	if err != nil {
		return "", fmt.Errorf("encoding display id: %w", err)
	}
	return encoded, nil
}
