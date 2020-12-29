#!/bin/bash
# Usage:
#
#     git difftool -y --extcmd=hacks/diff-csv.sh
#
# currently used for make sanity
set -e

diff --unified=1 --ignore-matching-lines='^\s*createdAt:\|^\s*currentCSV:' $@

IGNORE_PATTERN='^\s*createdAt:\|^\s*olm.skipRange:\|^\s*name: kubevirt-hyperconverged-operator.v\|^\s*version:'
diff --unified=1 --ignore-matching-lines="${IGNORE_PATTERN}" /tmp/csv-olm-before.yaml /tmp/csv-olm-after.yaml
diff --unified=1 --ignore-matching-lines="${IGNORE_PATTERN}" /tmp/csv-index-image-before.yaml /tmp/csv-index-image-after.yaml
