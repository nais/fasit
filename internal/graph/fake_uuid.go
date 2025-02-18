package graph

import (
	"strings"

	"github.com/google/uuid"
)

func fakeUUID(args ...string) uuid.UUID {
	return uuid.NewSHA1(uuid.Nil, []byte(strings.Join(args, "|")))
}
