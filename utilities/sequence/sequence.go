package sequence

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// Sequence a sequence generator.
// A quick note on the statistics here: we're trying to calculate the chance that
// two randomly generated base62 prefixes will collide. We use the formula from
// http://en.wikipedia.org/wiki/Birthday_problem
//
// P[m, n] \approx 1 - e^{-m^2/2n}
//
// We ballpark an upper bound for $m$ by imagining (for whatever reason) a server
// that restarts every second over 10 years, for $m = 86400 * 365 * 10 = 315360000$
//
// For a $k$ character base-62 identifier, we have $n(k) = 62^k$
//
// Plugging this in, we find $P[m, n(10)] \approx 5.75%$, which is good enough for
// our purposes, and is surely more than anyone would ever need in practice -- a
// process that is rebooted a handful of times a day for a hundred years has less
// than a millionth of a percent chance of generating two colliding IDs.
type Sequence struct {
	prefix     string
	sequenceId uint64
}

// New new sequence with prefix '{hostname}-{pid}-{init-rand-value}-' and zero sequence id.
func New() *Sequence {
	hostname, err := os.Hostname()
	if hostname == "" || err != nil {
		hostname = "localhost"
	}
	var buf [20]byte
	var b64 string
	for len(b64) < 16 {
		_, _ = rand.Read(buf[:])
		b64 = base64.StdEncoding.EncodeToString(buf[:])
		b64 = strings.NewReplacer("+", "", "/", "").Replace(b64)
	}

	return &Sequence{
		prefix:     fmt.Sprintf("%s-%d-%s-", hostname, os.Getpid(), b64[:16]),
		sequenceId: 0,
	}
}

// NewSequence generates the next sequence id format {hostname}-{pid}-{init-rand-value}-{sequence}.
// A sequence id is a string where "random" is a base62 random string
// that uniquely identifies this go process, and where the last number
// is an atomically incremented request counter.
func (s *Sequence) NewSequence() string {
	return fmt.Sprintf("%s%012d", s.prefix, atomic.AddUint64(&s.sequenceId, 1))
}
