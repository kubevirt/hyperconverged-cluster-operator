package cleanup

import (
	"bytes"
	"fmt"
	"regexp"
)

var (
	statusRegex    = regexp.MustCompile(`^\s*status:\s+\{}$`)
	timestampRegex = regexp.MustCompile(`^\s*creationTimestamp:\s+null$`)
	metadataRegex  = regexp.MustCompile(`^\s*metadata:$`)
	cronRegex      = regexp.MustCompile(`^(\s*schedule:\s+)(.*)$`)
)

func CleanOutput(out []byte) []byte {
	buf := bytes.Buffer{}
	lines := bytes.Split(out, []byte("\n"))

	for _, line := range lines {
		if statusRegex.Match(line) ||
			timestampRegex.Match(line) ||
			metadataRegex.Match(line) {
			continue
		}

		submatch := cronRegex.FindAllStringSubmatch(string(line), -1)
		if len(submatch) > 0 {
			line = []byte(fmt.Sprintf("%s%q", submatch[0][1], submatch[0][2]))
		}

		buf.Write(line)
		buf.WriteByte('\n')
	}

	return buf.Bytes()
}

/*
todo: use this code after moving to go 1.24, instead of the cleanOutput function above
var (
	statusRegex    = regexp.MustCompile(`^\s*status:\s*\{}\n$`)
	timestampRegex = regexp.MustCompile(`^\s*creationTimestamp:\s*null\n$`)
	metadataRegex  = regexp.MustCompile(`^\s*metadata:\n$`)
	cronRegex      = regexp.MustCompile(`^(\s*schedule:\s+)(.*)\n$`)
)

func CleanOutput(out []byte) []byte {
	lines := filter(bytes.Lines(out), func(line []byte) bool {
		return !statusRegex.Match(line) &&
			!timestampRegex.Match(line) &&
			!metadataRegex.Match(line)
	})

	buf := bytes.Buffer{}

	for line := range lines {
		submatch := cronRegex.FindAllStringSubmatch(string(line), -1)
		if len(submatch) > 0 {
			line = []byte(fmt.Sprintf("%s%q\n", submatch[0][1], submatch[0][2]))
		}

		buf.Write(line)
	}

	return buf.Bytes()
}

func filter[V any](seq iter.Seq[V], f func(V) bool) iter.Seq[V] {
	return func(yield func(V) bool) {
		for v := range seq {
			if !f(v) {
				continue
			}

			if !yield(v) {
				return
			}
		}
	}
}
*/
