package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func ToJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func ParseJSON(raw []byte, v interface{}) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}

func ParseJSONBytes(raw []byte, v interface{}) error {
	return json.Unmarshal(raw, v)
}

func SetTimeField(ctx context.Context, repo interface{}, id uuid.UUID, field string, t time.Time) error {
	return fmt.Errorf("not implemented: use direct DB update")
}

func PointerString(s string) *string {
	return &s
}

func PointerTime(t time.Time) *time.Time {
	return &t
}
