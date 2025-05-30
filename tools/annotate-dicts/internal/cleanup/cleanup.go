package cleanup

import (
	"bufio"
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

// CleanOutput removes unnecessary lines that was added by the yaml marshaller: the status, creationTimestamp, and
// metadata fields, and formats the schedule lines to be quoted strings, as in the original file.
//
// the object before the cleanup looks like this:
//
//	metadata:
//	  annotations:
//	    cdi.kubevirt.io/storage.bind.immediate.requested: "true"
//	    ssp.kubevirt.io/dict.architectures: amd64,arm64,s390x
//	  creationTimestamp: null                         # <<== this line will be removed
//	  labels:
//	    kubevirt.io/dynamic-credentials-support: "true"
//	  name: centos-stream10-image-cron
//	spec:
//	  garbageCollect: Outdated
//	  managedDataSource: centos-stream10
//	  schedule: 0 */12 * * *                          # <<== the schedule value will be changed to: "0 */12 * * *"
//	  template:
//	    metadata:                                     # <<== this line will be removed
//	      creationTimestamp: null                     # <<== this line will be removed
//	    spec:
//	      source:
//	        registry:
//	          pullMethod: node
//	          url: docker://quay.io/containerdisks/centos-stream:10
//	      storage:
//	        resources:
//	          requests:
//	            storage: 30Gi
//	    status: {}                                    # <<== this line will be removed
func CleanOutput(out []byte) []byte {
	buf := bytes.Buffer{}
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := scanner.Bytes()

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
