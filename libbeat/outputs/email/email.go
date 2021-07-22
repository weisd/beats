package email

import (
	"bytes"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs"
	"github.com/elastic/beats/libbeat/outputs/codec"
	"github.com/elastic/beats/libbeat/publisher"
)

var _ outputs.Client = &Email{}

// Email Email
type Email struct {
	cfg      config
	beat     beat.Info
	observer outputs.Observer
	codec    codec.Codec
	buff     *bytes.Buffer
	ch       chan []byte
	mailAuth smtp.Auth
}

func (p *Email) proc() {
	go func() {
		for {
			select {
			case msg := <-p.ch:
				p.buff.Write(msg)
				p.buff.Write([]byte{'\n'})
				p.buff.Write([]byte{'\n'})
				continue
			default:
				if p.buff.Len() == 0 {
					select {
					case msg := <-p.ch:
						p.buff.Write(msg)
						p.buff.Write([]byte{'\n'})
						p.buff.Write([]byte{'\n'})
						continue
					}
				}
				logp.Debug("Email", "send msg", p.buff.Len())

				err := p.Send(p.buff.String())
				if err != nil {
					logp.Error(err)
					continue
				}
				// do
				p.buff.Reset()
			}
		}
	}()
}

// Close Close
func (p *Email) Close() error {
	return nil
}

// Publish Publish
func (p *Email) Publish(batch publisher.Batch) error {
	defer batch.ACK()

	st := p.observer
	events := batch.Events()
	st.NewBatch(len(events))

	dropped := 0
	for i := range events {
		event := &events[i]

		serializedEvent, err := p.codec.Encode(p.beat.Beat, &event.Content)
		if err != nil {
			if event.Guaranteed() {
				logp.Critical("Failed to serialize the event: %v", err)
			} else {
				logp.Warn("Failed to serialize the event: %v", err)
			}
			logp.Debug("Email", "Failed event: %v", event)

			dropped++
			continue
		}

		logp.Debug("Email", "get event: %v", string(serializedEvent))

		p.ch <- serializedEvent

		st.WriteBytes(len(serializedEvent))
	}

	st.Dropped(dropped)
	st.Acked(len(events) - dropped)

	return nil
}

// String String
func (p *Email) String() string {
	return "Email"
}

// SendToMail SendToMail
func (p *Email) Send(body string) error {
	if len(p.cfg.Receiver) == 0 {
		return nil
	}

	msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", p.cfg.Receiver[0], p.cfg.Subject, body)

	return smtp.SendMail(p.cfg.Host, p.mailAuth, p.cfg.User, p.cfg.Receiver, []byte(msg))

}

// NewFactory NewFactory
func NewFactory(im outputs.IndexManager, beat beat.Info, stats outputs.Observer, cfg *common.Config) (outputs.Group, error) {
	config := defaultConfig
	if err := cfg.Unpack(&config); err != nil {
		return outputs.Fail(err)
	}

	logp.Debug("Email", "config:%v", config)

	codec, err := codec.CreateEncoder(beat, config.Codec)
	if err != nil {
		return outputs.Fail(err)
	}

	hp := strings.Split(config.Host, ":")

	c := &Email{
		beat:     beat,
		observer: stats,
		cfg:      config,
		codec:    codec,
		ch:       make(chan []byte, 100),
		buff:     new(bytes.Buffer),
		mailAuth: smtp.PlainAuth("", config.User, config.Password, hp[0]),
	}

	go c.proc()

	return outputs.Success(-1, 0, c)
}

func init() {
	outputs.RegisterType("Email", NewFactory)
}
