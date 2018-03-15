package requester

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

type Report struct {
	sync.RWMutex
	statusCodeDist map[int]int
	first          time.Time
	last           time.Time
}

func NewReport() *Report {
	return &Report{
		first:          time.Now(), // not perfect
		statusCodeDist: make(map[int]int),
	}
}

func (r *Report) Update(res *Result) {
	r.Lock()
	r.last = time.Now()
	r.statusCodeDist[res.StatusCode]++
	r.Unlock()
}

// sortedTotal sorts the keys and sums the values of the input map
func sortedTotal(m map[int]int) ([]int, int) {
	keys := make([]int, len(m))
	i := 0
	total := 0
	for k, v := range m {
		keys[i] = k
		total += v
		i++
	}
	sort.Ints(keys)
	return keys, total
}

func (r *Report) Summarize(w io.Writer) {
	r.Lock()
	defer r.Unlock()
	if r.last.IsZero() {
		r.last = time.Now()
	}
	codes, total := sortedTotal(r.statusCodeDist)
	div := float64(total)
	for _, code := range codes {
		cnt := r.statusCodeDist[code]
		fmt.Fprintf(w, "  [%d]\t%d responses (%.2f%%) \n", code, cnt, 100*float64(cnt)/div)
	}
	dur := r.last.Sub(r.first)
	fmt.Fprintf(w, "  total\t%d responses (%.2f rps) in %s [%s-%s]\n", total, div/dur.Seconds(), dur, r.first, r.last)
}
