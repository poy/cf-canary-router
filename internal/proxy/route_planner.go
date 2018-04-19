package proxy

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/apoydence/cf-canary-router/internal/structuredlogs"
)

type RoutePlanner struct {
	// currentPlan
	current unsafe.Pointer

	w   EventWriter
	log *log.Logger

	plan      Plan
	predicate Predicate
}

type currentPlan struct {
	idx  int64
	last time.Time
}

type PlanStep struct {
	// Percentage of requests to route to new route.
	Percentage int

	// Duration is the length of time that the step takes place before moving
	// on to the next. If this is the last step in the plan, then the planner
	// will recommend all the traffic go to the new route.
	Duration time.Duration
}

type Plan []PlanStep

type Predicate func() bool

// Codes are used to relay information from the application to the CLI about
// what actions are being taken.
const (
	NextPlanStep      = 10
	FinishedPlanSteps = 20
	Abort             = 30
)

type EventWriter interface {
	Write(structuredlogs.Event)
}

func NewRoutePlanner(plan Plan, p Predicate, w EventWriter, log *log.Logger) *RoutePlanner {
	current := &currentPlan{
		idx: -1,
	}

	return &RoutePlanner{
		plan:      plan,
		predicate: p,
		w:         w,
		log:       log,
		current:   unsafe.Pointer(current),
	}
}

func (p *RoutePlanner) CurrentPercentage() int {
	if !p.predicate() {
		p.w.Write(structuredlogs.Event{
			Code:    Abort,
			Message: "predicate failed. Directing traffic to previous route...",
		})
		return 0
	}

	current := (*currentPlan)(atomic.LoadPointer(&p.current))

	if current.idx >= int64(len(p.plan)) {
		p.w.Write(structuredlogs.Event{
			Code:    FinishedPlanSteps,
			Message: "finished steps",
		})
		return 100
	}

	if current.last.IsZero() || time.Since(current.last) >= p.plan[current.idx].Duration {
		updated := &currentPlan{
			last: time.Now(),
			idx:  current.idx + 1,
		}

		if !atomic.CompareAndSwapPointer(
			&p.current,
			unsafe.Pointer(current),
			unsafe.Pointer(updated),
		) {
			return p.CurrentPercentage()
		}
		current = updated

		if current.idx >= int64(len(p.plan)) {
			p.w.Write(structuredlogs.Event{
				Code:    FinishedPlanSteps,
				Message: "finished steps",
			})
			return p.CurrentPercentage()
		}

		p.w.Write(structuredlogs.Event{
			Code:    NextPlanStep,
			Message: fmt.Sprintf("starting next step: %+v", p.plan[current.idx]),
		})
		return p.CurrentPercentage()
	}

	return p.plan[current.idx].Percentage
}
