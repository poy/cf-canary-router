package structuredlogs

import (
	"fmt"
	"io"
)

type EventStream struct {
	s      LineStream
	writer io.Writer
}

type LineStream func() string

func NewEventStream(s LineStream, writer io.Writer) *EventStream {
	return &EventStream{
		s:      s,
		writer: writer,
	}
}

func (s *EventStream) Write(e Event) {
	data, err := e.Marshal()
	if err != nil {
		return
	}

	fmt.Fprintf(s.writer, "%s\n", data)
}

func (s *EventStream) NextEvent() Event {
	for {
		var e Event
		if err := e.Unmarshal(s.s()); err != nil {
			continue
		}

		return e
	}
}
