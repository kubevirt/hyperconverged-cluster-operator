package tests

import (
	"os"
)

const (
	KubevirtCfgMap = "kubevirt-config"
)

//GetJobTypeEnvVar returns "JOB_TYPE" enviroment varibale
func GetJobTypeEnvVar() string {
	return (os.Getenv("JOB_TYPE"))
}
