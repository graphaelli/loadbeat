package beater

import (
	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/graphaelli/loadbeat/config"
)

type Loadbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
	logger *logp.Logger
}

// Creates beater
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, errors.Wrap(err, "error reading config file")
	}

	bt := &Loadbeat{
		done:   make(chan struct{}),
		config: c,
		logger: logp.NewLogger("loadbeat"),
	}
	return bt, nil
}

func (bt *Loadbeat) Run(b *beat.Beat) error {
	bt.logger.Info("loadbeat is running! Hit CTRL-C to stop it.")

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	bt.logger.Infof("%+v", bt.config)

	<-bt.done
	return nil
}

func (bt *Loadbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
