package proxy

import (
	"log"
	"time"
)

type RoutePlanner struct {
	idx  int
	last time.Time

	log *log.Logger

	plan      Plan
	predicate Predicate
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

func NewRoutePlanner(plan Plan, p Predicate, log *log.Logger) *RoutePlanner {
	return &RoutePlanner{
		plan:      plan,
		predicate: p,
		log:       log,
	}
}

func (p *RoutePlanner) CurrentPercentage() int {
	if !p.predicate() {
		return 0
	}

	if p.last.IsZero() {
		p.last = time.Now()
	}

	if p.idx >= len(p.plan) {
		return 100
	}

	if time.Since(p.last) >= p.plan[p.idx].Duration {
		p.last = time.Now()
		p.idx++
		return p.CurrentPercentage()
	}

	return p.plan[p.idx].Percentage
}
