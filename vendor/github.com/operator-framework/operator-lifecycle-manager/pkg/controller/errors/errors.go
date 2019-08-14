package errors

import "fmt"

// MultipleExistingCRDOwnersError is an error that denotes multiple owners of a CRD exist
// simultaneously in the same namespace
type MultipleExistingCRDOwnersError struct {
	CSVNames  []string
	CRDName   string
	Namespace string
}

func (m MultipleExistingCRDOwnersError) Error() string {
	return fmt.Sprintf("Existing CSVs %v in namespace %s all claim to own CRD %s", m.CSVNames, m.Namespace, m.CRDName)
}

func NewMultipleExistingCRDOwnersError(csvNames []string, crdName string, namespace string) MultipleExistingCRDOwnersError {
	return MultipleExistingCRDOwnersError{
		CSVNames:  csvNames,
		CRDName:   crdName,
		Namespace: namespace,
	}
}

func IsMultipleExistingCRDOwnersError(err error) bool {
	switch err.(type) {
	case MultipleExistingCRDOwnersError:
		return true
	}

	return false
}
