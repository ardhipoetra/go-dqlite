package client

import (
	"github.com/ardhipoetra/go-dqlite/internal/protocol"
)

// Node roles
const (
	Voter   = protocol.Voter
	StandBy = protocol.StandBy
	Spare   = protocol.Spare
)
