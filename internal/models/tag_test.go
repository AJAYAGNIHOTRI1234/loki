package models

import (
	"bytes"
	"testing"
)

func TestTagSerializeAndParse(t *testing.T) {
	tag := &Tag{
		Object:  "0123456789abcdef0123456789abcdef01234567",
		Type:    CommitObject,
		Name:    "v1.0.0",
		Tagger:  "Loki Tester",
		Email:   "tester@loki.dev",
		Date:    1700000000,
		Message: "First official release",
	}

	serialized := tag.Serialize()
	if !bytes.HasPrefix(serialized, []byte("tag ")) {
		t.Fatalf("Expected serialized tag to start with 'tag ', got: %s", string(serialized))
	}

	parsed, err := ParseTag(serialized)
	if err != nil {
		t.Fatalf("ParseTag returned unexpected error: %v", err)
	}

	if parsed.Object != tag.Object {
		t.Errorf("Expected Object %s, got %s", tag.Object, parsed.Object)
	}
	if parsed.Type != tag.Type {
		t.Errorf("Expected Type %s, got %s", tag.Type, parsed.Type)
	}
	if parsed.Name != tag.Name {
		t.Errorf("Expected Name %s, got %s", tag.Name, parsed.Name)
	}
	if parsed.Tagger != tag.Tagger {
		t.Errorf("Expected Tagger %s, got %s", tag.Tagger, parsed.Tagger)
	}
	if parsed.Email != tag.Email {
		t.Errorf("Expected Email %s, got %s", tag.Email, parsed.Email)
	}
	if parsed.Date != tag.Date {
		t.Errorf("Expected Date %d, got %d", tag.Date, parsed.Date)
	}
	if parsed.Message != tag.Message {
		t.Errorf("Expected Message %q, got %q", tag.Message, parsed.Message)
	}
}

func TestTagSerialize_Defaults(t *testing.T) {
	tag := &Tag{
		Object:  "abcdef0123456789abcdef0123456789abcdef01",
		Name:    "v0.1",
		Tagger:  "User",
		Email:   "user@test",
		Message: "Initial test tag",
	}

	serialized := tag.Serialize()
	parsed, err := ParseTag(serialized)
	if err != nil {
		t.Fatalf("ParseTag error: %v", err)
	}

	if parsed.Type != CommitObject {
		t.Errorf("Expected default Type to be CommitObject, got %s", parsed.Type)
	}
	if parsed.Date == 0 {
		t.Errorf("Expected Date to default to non-zero timestamp")
	}
}
