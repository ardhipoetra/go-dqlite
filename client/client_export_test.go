package client

import (
	"github.com/ardhipoetra/go-dqlite/protocol"
)

func (c *Client) Protocol() *protocol.Protocol {
	return c.protocol
}
