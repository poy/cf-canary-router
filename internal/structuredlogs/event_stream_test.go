package structuredlogs_test

import (
	"strings"
	"testing"

	"github.com/poy/cf-canary-router/internal/structuredlogs"
	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
)

type TE struct {
	*testing.T
	s          *structuredlogs.EventStream
	lines      chan<- string
	stubWriter *stubWriter
}

func TestEventStream(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TE {
		lines := make(chan string, 10)
		stubWriter := newStubWriter()
		return TE{
			T:          t,
			lines:      lines,
			stubWriter: stubWriter,
			s: structuredlogs.NewEventStream(func() string {
				return <-lines
			}, stubWriter),
		}
	})

	o.Spec("it reads events from lines", func(t TE) {
		e1 := structuredlogs.Event{Code: 99}
		data, err := e1.Marshal()
		Expect(t, err).To(Not(HaveOccurred()))
		t.lines <- data

		e2 := structuredlogs.Event{Code: 101}
		data, err = e2.Marshal()
		Expect(t, err).To(Not(HaveOccurred()))
		t.lines <- data

		Expect(t, t.s.NextEvent().Code).To(Equal(99))
		Expect(t, t.s.NextEvent().Code).To(Equal(101))
	})

	o.Spec("it disregards non-event lines", func(t TE) {
		t.lines <- "invalid"

		e := structuredlogs.Event{Code: 99}
		data, err := e.Marshal()
		Expect(t, err).To(Not(HaveOccurred()))
		t.lines <- data

		Expect(t, t.s.NextEvent().Code).To(Equal(99))
	})

	o.Spec("it writes the event to the writer", func(t TE) {
		t.s.Write(structuredlogs.Event{Code: 99})
		t.s.Write(structuredlogs.Event{Code: 101})

		Expect(t, t.stubWriter.data).To(HaveLen(2))

		var e1 structuredlogs.Event
		Expect(t, e1.Unmarshal(string(t.stubWriter.data[0]))).To(
			Not(HaveOccurred()),
		)
		Expect(t, e1.Code).To(Equal(99))
		Expect(t, strings.HasSuffix(t.stubWriter.data[0], "\n")).To(BeTrue())

		var e2 structuredlogs.Event
		Expect(t, e2.Unmarshal(string(t.stubWriter.data[1]))).To(
			Not(HaveOccurred()),
		)
		Expect(t, e2.Code).To(Equal(101))
		Expect(t, strings.HasSuffix(t.stubWriter.data[1], "\n")).To(BeTrue())
	})
}

type stubWriter struct {
	data []string
}

func newStubWriter() *stubWriter {
	return &stubWriter{}
}

func (s *stubWriter) Write(p []byte) (n int, err error) {
	s.data = append(s.data, string(p))

	return len(s.data), nil
}
