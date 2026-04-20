package hyperconverged

import hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"

func newVersion(name, version string) hcov1.Version {
	return hcov1.Version{Name: name, Version: version}
}

func UpdateVersion(hcs *hcov1.HyperConvergedStatus, name, version string) {
	if hcs.Versions == nil {
		hcs.Versions = make([]hcov1.Version, 0, 1)
	}

	for i, v := range hcs.Versions {
		if v.Name == name {
			hcs.Versions[i].Version = version
			return
		}
	}
	hcs.Versions = append(hcs.Versions, newVersion(name, version))
}

func GetVersion(hcs *hcov1.HyperConvergedStatus, name string) (string, bool) {
	for _, v := range hcs.Versions {
		if v.Name == name {
			return v.Version, true
		}
	}
	return "", false
}
