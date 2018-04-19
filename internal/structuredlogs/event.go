package structuredlogs

import "encoding/json"

type Event struct {
	Code    int
	Message string
}

func (e Event) Marshal() (string, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (e *Event) Unmarshal(data string) error {
	return json.Unmarshal([]byte(data), e)
}
