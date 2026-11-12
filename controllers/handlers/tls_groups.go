package handlers

import openshiftv1 "github.com/openshift/api/config/v1"

func tlsGroupsToStrings(groups []openshiftv1.TLSGroup) []string {
	if len(groups) == 0 {
		return nil
	}

	groupStrings := make([]string, len(groups))
	for i, group := range groups {
		groupStrings[i] = string(group)
	}

	return groupStrings
}
