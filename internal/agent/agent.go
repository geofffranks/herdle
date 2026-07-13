package agent

import "fmt"

type Name string

const (
	Claude    Name = "claude"
	Polytoken Name = "polytoken"
)

func Parse(values []string) ([]Name, error) {
	if len(values) == 0 {
		return []Name{Claude}, nil
	}
	seen := map[Name]bool{}
	out := make([]Name, 0, len(values))
	for _, raw := range values {
		n := Name(raw)
		if n != Claude && n != Polytoken {
			return nil, fmt.Errorf("unknown agent %q (expected claude or polytoken)", raw)
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out, nil
}
