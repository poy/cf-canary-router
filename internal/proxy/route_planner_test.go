package proxy_test

import (
	"io/ioutil"
	"log"
	"testing"
	"time"

	"github.com/apoydence/cf-canary-router/internal/proxy"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TR struct {
	*testing.T
	p            *proxy.RoutePlanner
	spyPredicate *spyPredicate
}

func TestPlanner(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TR {
		plan := proxy.Plan{
			{Percentage: 5, Duration: 100 * time.Millisecond},
			{Percentage: 10, Duration: 100 * time.Millisecond},
		}

		spyPredicate := newSpyPredicate()
		spyPredicate.result = true

		return TR{
			T:            t,
			spyPredicate: spyPredicate,
			p: proxy.NewRoutePlanner(
				plan,
				spyPredicate.Predicate,
				log.New(ioutil.Discard, "", 0),
			),
		}
	})

	o.Spec("it returns the plan over time", func(t TR) {
		for i := 0; i < 100; i++ {
			Expect(t, t.p.CurrentPercentage()).To(Equal(5))
		}

		time.Sleep(100 * time.Millisecond)

		for i := 0; i < 100; i++ {
			Expect(t, t.p.CurrentPercentage()).To(Equal(10))
		}

		time.Sleep(100 * time.Millisecond)

		for i := 0; i < 100; i++ {
			Expect(t, t.p.CurrentPercentage()).To(Equal(100))
		}
	})

	o.Spec("it aborts and returns 0 if the predicate fails", func(t TR) {
		t.spyPredicate.result = false
		for i := 0; i < 100; i++ {
			Expect(t, t.p.CurrentPercentage()).To(Equal(0))
		}
	})
}

type spyPredicate struct {
	result bool
}

func newSpyPredicate() *spyPredicate {
	return &spyPredicate{}
}

func (s *spyPredicate) Predicate() bool {
	return s.result
}
