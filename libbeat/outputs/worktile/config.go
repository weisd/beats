package worktile

import (
	"github.com/elastic/beats/v7/libbeat/outputs/codec"
)

type config struct {
	URL   string       `config:"url"`
	Codec codec.Config `config:"codec"`
}

var (
	defaultConfig = config{}
)

func (c *config) Validate() error {
	return nil
}
