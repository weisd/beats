package email

import (
	"github.com/elastic/beats/libbeat/outputs/codec"
)

type config struct {
	Receiver []string     `config:"receiver"`
	Host     string       `config:"host"`
	User     string       `config:"user"`
	Subject  string       `config:"subject"`
	Password string       `config:"password"`
	Codec    codec.Config `config:"codec"`
}

var (
	defaultConfig = config{}
)

func (c *config) Validate() error {
	return nil
}
