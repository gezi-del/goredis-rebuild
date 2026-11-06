package database

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"
)

// Snapshotter provides a read-only view without exposing the storage map.
type Snapshotter interface {
	Snapshot(context.Context) ([]SnapshotEntry, error)
}

type SnapshotEntry struct {
	Key       string     `json:"key"`
	Type      string     `json:"type"`
	Value     any        `json:"value"`
	Size      int        `json:"size"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

type ScoredMember struct {
	Member string `json:"member"`
	Score  int64  `json:"score"`
}

type snapshotRequest struct {
	reply chan []SnapshotEntry
}

func (e *DBExecutor) Snapshot(ctx context.Context) ([]SnapshotEntry, error) {
	request := snapshotRequest{reply: make(chan []SnapshotEntry, 1)}
	select {
	case e.snapshots <- request:
	case <-e.ctx.Done():
		return nil, errors.New("database is closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case entries := <-request.reply:
		return entries, nil
	case <-e.ctx.Done():
		return nil, errors.New("database is closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Called only by run(): ForEach and ToCmd execute in the same goroutine as writes.
func (e *DBExecutor) snapshot() []SnapshotEntry {
	entries := make([]SnapshotEntry, 0)
	e.dataStore.ForEach(func(key string, adapter CmdAdapter, expiresAt *time.Time) {
		if adapter == nil {
			return
		}
		cmd := adapter.ToCmd()
		if len(cmd) < 2 {
			return
		}
		entry := SnapshotEntry{Key: key}
		if expiresAt != nil {
			copied := *expiresAt
			entry.ExpiresAt = &copied
		}
		values := make([]string, 0, len(cmd)-2)
		for _, value := range cmd[2:] {
			values = append(values, string(value))
		}
		switch CmdType(cmd[0]) {
		case CmdTypeSet:
			entry.Type = "string"
			if len(values) > 0 {
				entry.Value, entry.Size = values[0], len(values[0])
			}
		case CmdTypeRPush:
			entry.Type, entry.Value, entry.Size = "list", values, len(values)
		case CmdTypeSAdd:
			sort.Strings(values)
			entry.Type, entry.Value, entry.Size = "set", values, len(values)
		case CmdTypeHSet:
			fields := make(map[string]string)
			for i := 0; i+1 < len(values); i += 2 {
				fields[values[i]] = values[i+1]
			}
			entry.Type, entry.Value, entry.Size = "hash", fields, len(fields)
		case CmdTypeZAdd:
			members := make([]ScoredMember, 0, len(values)/2)
			for i := 0; i+1 < len(values); i += 2 {
				score, _ := strconv.ParseInt(values[i], 10, 64)
				members = append(members, ScoredMember{Member: values[i+1], Score: score})
			}
			sort.Slice(members, func(i, j int) bool {
				if members[i].Score == members[j].Score {
					return members[i].Member < members[j].Member
				}
				return members[i].Score < members[j].Score
			})
			entry.Type, entry.Value, entry.Size = "zset", members, len(members)
		default:
			return
		}
		entries = append(entries, entry)
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries
}
