package core

import (
	"encoding/json"
	"time"

	"github.com/oklog/ulid/v2"
)

type OpType string

const (
	OpTaskCreate    OpType = "task.create"
	OpTaskSet       OpType = "task.set"
	OpTagAdd        OpType = "tag.add"
	OpTagRemove     OpType = "tag.remove"
	OpProjectCreate OpType = "project.create"
	OpProjectSet    OpType = "project.set"
)

// Op is one line of an op log. Target is the task or project ID.
type Op struct {
	ID      string          `json:"id"`
	TS      time.Time       `json:"ts"`
	Host    string          `json:"host,omitempty"`
	Type    OpType          `json:"type"`
	Target  string          `json:"target"`
	Field   string          `json:"field,omitempty"`
	Value   json.RawMessage `json:"value,omitempty"`
	Tag     string          `json:"tag,omitempty"`
	Task    *TaskCreate     `json:"task,omitempty"`
	Project *ProjectCreate  `json:"project,omitempty"`
}

type TaskCreate struct {
	Title       string     `json:"title"`
	Category    Category   `json:"category"`
	Project     string     `json:"project,omitempty"`
	Status      Status     `json:"status"`
	Note        string     `json:"note,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	OnWeek      bool       `json:"on_week"`
	WeekAddedAt *time.Time `json:"week_added_at,omitempty"`
}

type ProjectCreate struct {
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	Description string   `json:"description,omitempty"`
}

// Less is the replay order: timestamp, then op ULID.
func (o Op) Less(p Op) bool {
	if !o.TS.Equal(p.TS) {
		return o.TS.Before(p.TS)
	}
	return o.ID < p.ID
}

func NewID() string { return ulid.Make().String() }

func newOp(now time.Time, typ OpType, target string) Op {
	return Op{ID: NewID(), TS: now.UTC(), Type: typ, Target: target}
}

func setOp(now time.Time, typ OpType, target, field string, v any) Op {
	op := newOp(now, typ, target)
	op.Field = field
	op.Value, _ = json.Marshal(v)
	return op
}

func setTask(now time.Time, id, field string, v any) Op {
	return setOp(now, OpTaskSet, id, field, v)
}

func setProject(now time.Time, id, field string, v any) Op {
	return setOp(now, OpProjectSet, id, field, v)
}

func tagOp(now time.Time, typ OpType, id, tag string) Op {
	op := newOp(now, typ, id)
	op.Tag = tag
	return op
}
