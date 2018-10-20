package proxy_test

import (
	"io/ioutil"
	"log"
	"testing"
	"time"

	"github.com/poy/cf-canary-router/internal/proxy"
	"github.com/poy/cf-canary-router/internal/structuredlogs"
	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
)

type TR struct {
	*testing.T
	p              *proxy.RoutePlanner
	spyEventWriter *spyEventWriter
	spyPredicate   *spyPredicate
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

		spyEventWriter := newSpyEventWriter()

		return TR{
			T:              t,
			spyPredicate:   spyPredicate,
			spyEventWriter: spyEventWriter,
			p: proxy.NewRoutePlanner(
				plan,
				spyPredicate.Predicate,
				spyEventWriter,
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

		Expect(t, t.spyEventWriter.events).To(Contain(structuredlogs.Event{
			Code:    proxy.NextPlanStep,
			Message: "starting next step: {Percentage:10 Duration:100ms}",
		}))

		Expect(t, t.spyEventWriter.events).To(Contain(structuredlogs.Event{
			Code:    proxy.FinishedPlanSteps,
			Message: "finished steps",
		}))
	})

	o.Spec("it aborts and returns 0 if the predicate fails", func(t TR) {
		t.spyPredicate.result = false
		for i := 0; i < 100; i++ {
			Expect(t, t.p.CurrentPercentage()).To(Equal(0))
		}

		Expect(t, t.spyEventWriter.events).To(HaveLen(100))
		Expect(t, t.spyEventWriter.events[0].Code).To(Equal(proxy.Abort))
	})

	o.Spec("it survives the race detector", func(t TR) {
		go func() {
			for i := 0; i < 100; i++ {
				t.p.CurrentPercentage()
			}
		}()

		for i := 0; i < 100; i++ {
			t.p.CurrentPercentage()
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

type spyEventWriter struct {
	events []structuredlogs.Event
}

func newSpyEventWriter() *spyEventWriter {
	return &spyEventWriter{}
}

func (s *spyEventWriter) Write(e structuredlogs.Event) {
	s.events = append(s.events, e)
}
