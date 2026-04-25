package prmark

import "regexp"

var markerRE = regexp.MustCompile(`<!--\s*squad-item:\s*([^\s>][^\s]*)\s*-->`)

func Extract(body string) string {
	m := markerRE.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func Format(itemID string) string {
	return "<!-- squad-item: " + itemID + " -->"
}
