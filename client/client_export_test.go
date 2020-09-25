package client

import (
	"github.com/ardhipoetra/go-dqlite/internal/protocol"
)

func (c *Client) Protocol() *protocol.Protocol {
	return c.protocol
}
