package predicate_test

import (
	"context"
	"io/ioutil"
	"log"
	"sync"
	"testing"
	"time"

	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/apoydence/cf-canary-router/internal/predicate"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TP struct {
	*testing.T

	spyDataReader *spyDataReader
	p             *predicate.PromQL
}

func TestPromQLPredicate(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TP {
		spyDataReader := newSpyDataReader()
		ticker := make(chan time.Time, 1)
		ticker <- time.Now()

		return TP{
			T:             t,
			spyDataReader: spyDataReader,
			p: predicate.NewPromQL(
				`metric{source_id="some-id-1"} + metric{source_id="some-id-2"} > 5`,
				spyDataReader,
				ticker,
				log.New(ioutil.Discard, "", 0),
			),
		}
	})

	o.Spec("it returns true while the query returns true", func(t TP) {
		t.spyDataReader.readErrs = []error{nil, nil}
		t.spyDataReader.readResults = [][]*loggregator_v2.Envelope{
			{{
				SourceId:  "some-id-1",
				Timestamp: time.Now().UnixNano(),
				Message: &loggregator_v2.Envelope_Counter{
					Counter: &loggregator_v2.Counter{
						Name:  "metric",
						Total: 99,
					},
				},
			}},
			{{
				SourceId:  "some-id-2",
				Timestamp: time.Now().UnixNano(),
				Message: &loggregator_v2.Envelope_Counter{
					Counter: &loggregator_v2.Counter{
						Name:  "metric",
						Total: 99,
					},
				},
			}},
		}

		Expect(t, t.p.Predicate).To(ViaPolling(BeTrue()))
		Expect(t, t.spyDataReader.ReadSourceIDs).To(ViaPolling(
			Contain("some-id-1", "some-id-2"),
		))
	})

	o.Spec("it returns false while the query returns false", func(t TP) {
		Expect(t, t.p.Predicate()).To(BeFalse())
		Expect(t, t.spyDataReader.ReadSourceIDs).To(ViaPolling(
			Contain("some-id-1", "some-id-2"),
		))
	})
}

type spyDataReader struct {
	mu            sync.Mutex
	readSourceIDs []string
	readStarts    []time.Time

	readResults [][]*loggregator_v2.Envelope
	readErrs    []error
}

func newSpyDataReader() *spyDataReader {
	return &spyDataReader{}
}

func (s *spyDataReader) Read(
	ctx context.Context,
	sourceID string,
	start time.Time,
	opts ...logcache.ReadOption,
) ([]*loggregator_v2.Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readSourceIDs = append(s.readSourceIDs, sourceID)
	s.readStarts = append(s.readStarts, start)

	if len(s.readResults) != len(s.readErrs) {
		panic("readResults and readErrs are out of sync")
	}

	if len(s.readResults) == 0 {
		return nil, nil
	}

	r := s.readResults[0]
	err := s.readErrs[0]

	s.readResults = s.readResults[1:]
	s.readErrs = s.readErrs[1:]

	return r, err
}

func (s *spyDataReader) ReadSourceIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]string, len(s.readSourceIDs))
	copy(result, s.readSourceIDs)

	return result
}
