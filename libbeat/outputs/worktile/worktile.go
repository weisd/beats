package worktile

import (
	"bytes"
	"net/http"
	"net/url"

	"github.com/elastic/beats/libbeat/v7/beat"
	"github.com/elastic/beats/libbeat/v7/common"
	"github.com/elastic/beats/libbeat/v7/logp"
	"github.com/elastic/beats/libbeat/v7/outputs"
	"github.com/elastic/beats/libbeat/v7/outputs/codec"
	"github.com/elastic/beats/libbeat/v7/publisher"
)

var _ outputs.Client = &Worktile{}

// Worktile Worktile
type Worktile struct {
	cfg      config
	beat     beat.Info
	observer outputs.Observer
	codec    codec.Codec
	buff     *bytes.Buffer
	ch       chan []byte
	client   *http.Client
}

func (p *Worktile) proc() {
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
				logp.Debug("worktile", "send msg", p.buff.Len())

				val := url.Values{}
				val.Set("text", p.buff.String())
				resp, err := p.client.PostForm(p.cfg.URL, val)
				if err != nil {
					logp.Error(err)
					continue
				}

				resp.Body.Close()

				// do
				p.buff.Reset()
			}
		}
	}()
}

// Close Close
func (p *Worktile) Close() error {
	return nil
}

// Publish Publish
func (p *Worktile) Publish(batch publisher.Batch) error {
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
			logp.Debug("worktile", "Failed event: %v", event)

			dropped++
			continue
		}

		logp.Debug("worktile", "get event: %v", string(serializedEvent))

		p.ch <- serializedEvent

		st.WriteBytes(len(serializedEvent))
	}

	st.Dropped(dropped)
	st.Acked(len(events) - dropped)

	return nil
}

// String String
func (p *Worktile) String() string {
	return "worktile"
}

// NewFactory NewFactory
func NewFactory(im outputs.IndexManager, beat beat.Info, stats outputs.Observer, cfg *common.Config) (outputs.Group, error) {
	config := defaultConfig
	if err := cfg.Unpack(&config); err != nil {
		return outputs.Fail(err)
	}

	logp.Debug("worktile", "config:%v", config)

	codec, err := codec.CreateEncoder(beat, config.Codec)
	if err != nil {
		return outputs.Fail(err)
	}

	c := &Worktile{
		beat:     beat,
		observer: stats,
		cfg:      config,
		codec:    codec,
		ch:       make(chan []byte, 100),
		buff:     new(bytes.Buffer),
		client:   &http.Client{},
	}

	go c.proc()

	return outputs.Success(-1, 0, c)
}

func init() {
	outputs.RegisterType("worktile", NewFactory)
}
