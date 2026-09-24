package models

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Tag struct {
	Object  string
	Type    ObjectType
	Name    string
	Tagger  string
	Email   string
	Date    int64
	Message string
}

func (t *Tag) Serialize() []byte {
	date := t.Date
	if date == 0 {
		date = time.Now().Unix()
	}
	objType := t.Type
	if objType == "" {
		objType = CommitObject
	}

	body := fmt.Sprintf(
		"object %s\n"+
			"type %s\n"+
			"tag %s\n"+
			"tagger %s <%s> %d\n\n"+
			"%s\n",
		t.Object,
		objType,
		t.Name,
		t.Tagger,
		t.Email,
		date,
		t.Message,
	)

	header := fmt.Sprintf("tag %d\x00", len(body))
	return append([]byte(header), []byte(body)...)
}

func ParseTag(data []byte) (*Tag, error) {
	idx := bytes.IndexByte(data, 0)
	if idx >= 0 {
		data = data[idx+1:]
	}

	tag := &Tag{}
	parts := bytes.SplitN(data, []byte("\n\n"), 2)
	headerLines := bytes.Split(parts[0], []byte("\n"))
	for _, line := range headerLines {
		lineStr := string(line)
		if strings.HasPrefix(lineStr, "object ") {
			tag.Object = strings.TrimSpace(strings.TrimPrefix(lineStr, "object "))
		} else if strings.HasPrefix(lineStr, "type ") {
			tag.Type = ObjectType(strings.TrimSpace(strings.TrimPrefix(lineStr, "type ")))
		} else if strings.HasPrefix(lineStr, "tag ") {
			tag.Name = strings.TrimSpace(strings.TrimPrefix(lineStr, "tag "))
		} else if strings.HasPrefix(lineStr, "tagger ") {
			taggerLine := strings.TrimPrefix(lineStr, "tagger ")
			leftAngle := strings.Index(taggerLine, "<")
			rightAngle := strings.Index(taggerLine, ">")
			if leftAngle != -1 && rightAngle != -1 && rightAngle > leftAngle {
				tag.Tagger = strings.TrimSpace(taggerLine[:leftAngle])
				tag.Email = strings.TrimSpace(taggerLine[leftAngle+1 : rightAngle])
				timeStr := strings.TrimSpace(taggerLine[rightAngle+1:])
				if ts, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
					tag.Date = ts
				}
			} else {
				tag.Tagger = taggerLine
			}
		}
	}
	if len(parts) > 1 {
		tag.Message = strings.TrimSpace(string(parts[1]))
	}
	return tag, nil
}
