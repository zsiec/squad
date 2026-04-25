package prmark

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	ItemID    string    `json:"item_id"`
	Branch    string    `json:"branch"`
	CreatedAt time.Time `json:"created_at"`
}

func ReadPending(path string) ([]Entry, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	var out []Entry
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func AppendPending(path string, e Entry) error {
	cur, err := ReadPending(path)
	if err != nil {
		return err
	}
	cur = append(cur, e)
	return writePending(path, cur)
}

func RemovePending(path, itemID string) error {
	cur, err := ReadPending(path)
	if err != nil {
		return err
	}
	if len(cur) == 0 {
		return nil
	}
	out := cur[:0]
	for _, e := range cur {
		if e.ItemID != itemID {
			out = append(out, e)
		}
	}
	return writePending(path, out)
}

func writePending(path string, entries []Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
