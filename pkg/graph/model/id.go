package model

import (
	"fmt"

	"github.com/google/uuid"
)

type ID uuid.UUID

func NewID() ID {
	return ID(uuid.New())
}

func (i ID) DecodeSpanner(input interface{}) error {
	if input == nil {
		return nil
	}
	if input, ok := input.([]byte); ok {
		u, err := uuid.FromBytes(input)
		if err != nil {
			return err
		}

		i = ID(u)
		return nil
	}
	return fmt.Errorf("unable to decode id: %v", input)
}

func (i ID) EncodeSpanner() (interface{}, error) {
	return i[:], nil
}
