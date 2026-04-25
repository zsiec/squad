package items

import (
	"fmt"
	"regexp"
	"strconv"
)

var idRe = regexp.MustCompile(`^([A-Z][A-Z0-9]*)-([0-9]+)$`)

func ParseID(id string) (prefix string, n int, err error) {
	m := idRe.FindStringSubmatch(id)
	if m == nil {
		return "", 0, fmt.Errorf("invalid item ID %q (want PREFIX-NUMBER)", id)
	}
	n, err = strconv.Atoi(m[2])
	if err != nil {
		return "", 0, fmt.Errorf("invalid number in %q: %w", id, err)
	}
	return m[1], n, nil
}

func ValidateID(id string, allowed []string) error {
	prefix, _, err := ParseID(id)
	if err != nil {
		return err
	}
	for _, p := range allowed {
		if p == prefix {
			return nil
		}
	}
	return fmt.Errorf("ID %q uses prefix %q; allowed: %v", id, prefix, allowed)
}
