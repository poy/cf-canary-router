package structuredlogs_test

import (
	"testing"

	"github.com/apoydence/cf-canary-router/internal/structuredlogs"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TS struct {
	*testing.T
}

func TestEvent(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TS {
		return TS{
			T: t,
		}
	})

	o.Spec("it marshals and unmarshals", func(t TS) {
		e := structuredlogs.Event{
			Code:    99,
			Message: "some-message",
		}
		data, err := e.Marshal()
		Expect(t, err).To(Not(HaveOccurred()))

		var ee structuredlogs.Event
		err = ee.Unmarshal(data)
		Expect(t, err).To(Not(HaveOccurred()))

		Expect(t, e.Code).To(Equal(99))
		Expect(t, e.Message).To(Equal("some-message"))
	})

	o.Spec("it returns an error while unmarshalling garbage", func(t TS) {
		var e structuredlogs.Event
		err := e.Unmarshal("invalid")
		Expect(t, err).To(HaveOccurred())
	})
}
