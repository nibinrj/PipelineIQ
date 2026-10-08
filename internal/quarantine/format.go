package quarantine

import (
	"sort"
	"strings"
)

// Ref is one quarantined method. The class name is the Surefire fully qualified name.
type Ref struct {
	ClassName  string
	MethodName string
}

const excludesHeader = "# pipelineiq quarantine — generated, do not edit\n"

// ExcludesFile is the surefire.excludesFile syntax checked in the P1 note.
// An empty list is only the comment, which Surefire ignores.
func ExcludesFile(tests []Ref) string {
	var b strings.Builder
	b.WriteString(excludesHeader)
	for _, test := range sorted(tests) {
		b.WriteString(test.ClassName)
		b.WriteByte('#')
		b.WriteString(test.MethodName)
		b.WriteByte('\n')
	}
	return b.String()
}

// OnlyArg is the -Dtest value for the quarantine stage. An empty list is empty,
// and the stage is skipped.
func OnlyArg(tests []Ref) string {
	grouped := map[string][]string{}
	var classes []string
	for _, test := range sorted(tests) {
		if _, ok := grouped[test.ClassName]; !ok {
			classes = append(classes, test.ClassName)
		}
		grouped[test.ClassName] = append(grouped[test.ClassName], test.MethodName)
	}
	parts := make([]string, 0, len(classes))
	for _, className := range classes {
		parts = append(parts, className+"#"+strings.Join(grouped[className], "+"))
	}
	return strings.Join(parts, ",")
}

func sorted(tests []Ref) []Ref {
	out := append([]Ref(nil), tests...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].ClassName == out[j].ClassName {
			return out[i].MethodName < out[j].MethodName
		}
		return out[i].ClassName < out[j].ClassName
	})
	return out
}
