package requester

/*
Heavily borrowed from Apache 2.0 Licensed https://github.com/rakyll/hey.
*/
import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"
)

const maxIdleConn = 500

type Result struct {
	Request       *http.Request
	Err           error
	StatusCode    int
	Duration      time.Duration
	ConnDuration  time.Duration // connection setup(DNS lookup + Dial up) Duration
	DnsDuration   time.Duration // dns lookup Duration
	ReqDuration   time.Duration // request "write" Duration
	ResDuration   time.Duration // response "read" Duration
	DelayDuration time.Duration // delay between response and request
	Reused        bool          // whether connection was reused
	ContentLength int64
}

type Work struct {
	// Request is the request to be made.
	Request *http.Request

	RequestBody []byte

	// N is the total number of requests to make.
	N int

	// C is the concurrency level, the number of concurrent workers to run.
	C int

	// Timeout
	Timeout time.Duration

	// Qps is the rate limit in queries per second.
	QPS float64

	// DisableCompression is an option to disable compression in response
	DisableCompression bool

	// DisableKeepAlives is an option to prevents re-use of TCP connections between different HTTP requests
	DisableKeepAlives bool

	// DisableRedirects is an option to prevent the following of HTTP redirects
	DisableRedirects bool

	HandleResult func(*Result)
	stopCh       chan struct{}
}

// Run makes all the requests, prints the summary. It blocks until
// all work is done.
func (b *Work) Run() {
	b.stopCh = make(chan struct{}, b.C)
	b.runWorkers()
}

func (b *Work) Stop() {
	// Send stop signal so that workers can stop gracefully.
	for i := 0; i < b.C; i++ {
		b.stopCh <- struct{}{}
	}
}

func (b *Work) makeRequest(c *http.Client) {
	s := time.Now()
	var size int64
	var code int
	var dnsStart, connStart, resStart, reqStart, delayStart time.Time
	var dnsDuration, connDuration, resDuration, reqDuration, delayDuration time.Duration
	var reused bool
	req := cloneRequest(b.Request, b.RequestBody)
	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			dnsDuration = time.Now().Sub(dnsStart)
		},
		GetConn: func(h string) {
			connStart = time.Now()
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			reused = connInfo.Reused
			if !connInfo.Reused {
				connDuration = time.Now().Sub(connStart)
			}
			reqStart = time.Now()
		},
		WroteRequest: func(w httptrace.WroteRequestInfo) {
			reqDuration = time.Now().Sub(reqStart)
			delayStart = time.Now()
		},
		GotFirstResponseByte: func() {
			delayDuration = time.Now().Sub(delayStart)
			resStart = time.Now()
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := c.Do(req)
	if err == nil {
		size = resp.ContentLength
		code = resp.StatusCode
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
	t := time.Now()
	resDuration = t.Sub(resStart)
	finish := t.Sub(s)
	b.HandleResult(&Result{
		Request:       req,
		StatusCode:    code,
		Duration:      finish,
		Err:           err,
		ContentLength: size,
		ConnDuration:  connDuration,
		DnsDuration:   dnsDuration,
		ReqDuration:   reqDuration,
		ResDuration:   resDuration,
		DelayDuration: delayDuration,
		Reused:        reused,
	})
}

func (b *Work) runWorker(client *http.Client, n int) {
	var throttle <-chan time.Time
	if b.QPS > 0 {
		throttle = time.Tick(time.Duration(1e6/(b.QPS)) * time.Microsecond)
	}

	if b.DisableRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	for i := 0; i < n; i++ {
		// Check if application is stopped. Do not send into a closed channel.
		select {
		case <-b.stopCh:
			return
		default:
			if b.QPS > 0 {
				<-throttle
			}
			b.makeRequest(client)
		}
	}
}

func (b *Work) runWorkers() {
	var wg sync.WaitGroup
	wg.Add(b.C)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleConnsPerHost: min(b.C, maxIdleConn),
		DisableCompression:  b.DisableCompression,
		DisableKeepAlives:   b.DisableKeepAlives,
	}
	tr.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	client := &http.Client{Transport: tr, Timeout: b.Timeout}

	// Ignore the case where b.N % b.C != 0.
	for i := 0; i < b.C; i++ {
		go func() {
			b.runWorker(client, b.N/b.C)
			wg.Done()
		}()
	}
	wg.Wait()
}

func (b *Work) String() string {
	var gzBody bytes.Buffer
	if len(b.RequestBody) > 0 {
		zw := gzip.NewWriter(&gzBody)
		zw.Write(b.RequestBody)
		zw.Close()
	}
	return fmt.Sprintf("%s %s - %d (%d gz) bytes", b.Request.Method, b.Request.URL, len(b.RequestBody), len(gzBody.Bytes()))
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request, body []byte) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	if len(body) > 0 {
		r2.Body = ioutil.NopCloser(bytes.NewReader(body))
	}
	return r2
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
