package beater

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/graphaelli/loadbeat/config"
	"github.com/graphaelli/loadbeat/requester"
)

type Loadbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
	logger *logp.Logger
	work   []*requester.Work

	mu       sync.Mutex
	stopping bool
}

func getWork(c config.Config, handleResult func(*requester.Result)) ([]*requester.Work, error) {
	var work []*requester.Work
	for _, baseUrl := range c.BaseUrls {
		for _, t := range c.Targets {
			url := baseUrl + t.Url
			req, err := http.NewRequest(t.Method, url, nil)
			if err != nil {
				panic(err)
			}

			for _, header := range t.Headers {
				parts := strings.SplitN(header, ":", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("bad header config in %s %s: %s", t.Method, t.Url, header)
				}
				req.Header.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}

			w := &requester.Work{
				Request:     req,
				RequestBody: []byte(t.Body), // TODO: support other body types

				DisableCompression: !c.Compression,
				DisableKeepAlives:  !c.Keepalives,
				DisableRedirects:   !c.Redirects,
				C:                  t.Concurrent,
				N:                  c.MaxRequests,
				QPS:                t.Qps,
				Timeout:            c.RequestTimeout,

				HandleResult: handleResult,
			}
			work = append(work, w)
		}
	}

	if len(work) == 0 {
		return nil, errors.New("no work to do")
	}

	return work, nil
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

	work, err := getWork(c, bt.handleResult)
	if err != nil {
		return nil, err
	}
	bt.work = work

	return bt, nil
}

func (bt *Loadbeat) handleResult(r *requester.Result) {
	var errStr *string
	if r.Err != nil {
		s := r.Err.Error()
		errStr = &s
		bt.logger.Error(r.Err)
	}

	bt.client.Publish(beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"method":   r.Request.Method,
			"url":      r.Request.URL.RequestURI(),
			"bodysize": r.ContentLength,
			"trace": common.MapStr{
				"connection": r.ConnDuration.Nanoseconds(),
				"dns":        r.DnsDuration.Nanoseconds(),
				"request":    r.ReqDuration.Nanoseconds(),
				"response":   r.ResDuration.Nanoseconds(),
				"server":     r.DelayDuration.Nanoseconds(),
				"reused":     r.Reused,
			},
			"code":     r.StatusCode,
			"duration": r.Duration,
			"complete": r.Err == nil,
			"err":      errStr,
		},
	})
}

func (bt *Loadbeat) annotate(message string) time.Time {
	now := time.Now()
	bt.client.Publish(beat.Event{
		Timestamp: now,
		Fields: common.MapStr{
			"annotation": message,
		},
	})
	return now
}

func (bt *Loadbeat) Run(b *beat.Beat) error {
	bt.logger.Info("loadbeat is running! Hit CTRL-C to stop it.")

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	descriptions := make([]string, len(bt.work))
	for i, w := range bt.work {
		descriptions[i] = w.String()
	}
	description := strings.Join(descriptions, "\n")
	startTime := bt.annotate("start\n" + description)

	// start load generation workers
	var wg sync.WaitGroup
	wg.Add(len(bt.work))
	for _, w := range bt.work {
		go func(w *requester.Work) {
			bt.logger.Info("starting worker for ", w.Request.URL)
			w.Run()
			wg.Done()
		}(w)
	}

	go func() {
		wg.Wait()
		bt.Stop()
	}()

	select {
	case <-bt.done:
		bt.Stop()
	case <-time.After(bt.config.RunTimeout):
		bt.Stop()
		<-bt.done
	}
	endTime := bt.annotate("stop\n" + description)
	bt.logger.Infof("run complete after %s", endTime.Sub(startTime))
	return nil
}

func (bt *Loadbeat) Stop() {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	if bt.stopping {
		return
	}
	bt.stopping = true

	// stop load generation workers
	var wg sync.WaitGroup
	wg.Add(len(bt.work))
	for _, w := range bt.work {
		go func(w *requester.Work) {
			bt.logger.Info("stopping worker for ", w.Request.URL)
			w.Stop()
			wg.Done()
		}(w)
	}
	stopped := make(chan struct{})
	go func() {
		wg.Wait()
		close(stopped)
	}()
	select {
	case <-time.After(30 * time.Second):
		bt.logger.Info("timed out waiting for workers to stop")
	case <-stopped:
	}

	bt.client.Close()
	close(bt.done)
}
