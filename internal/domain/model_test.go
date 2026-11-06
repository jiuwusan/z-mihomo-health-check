package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInternalEventSequenceIsNotPartOfHTTPStatus(t *testing.T) {
	data, err := json.Marshal(Status{EventSequence: 42})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Sequence") || strings.Contains(string(data), "sequence") {
		t.Fatalf("internal sequence leaked: %s", data)
	}
}
