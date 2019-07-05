# CR Conditions and Readiness Probe
Conditions are..
	   _the latest available observations of an object's state. They are
	   an extension mechanism intended to be used when the details of an
	   observation are not a priori known or would not apply to all
	   instances of a given Kind._

The HCO’s CR is a representation of the underlying component operators.  If the
object exists, then all components exist.  If the object doesn’t exist, all
components don’t exist.  However, the CR existence doesn’t help us with the
state where the operators exist, but are they healthy?  This is where conditions
on the HCO’s CR will answer this question by providing the observed health of
the underlying components.

## Condition List
We can use some of the CVO's [conditions](https://github.com/openshift/api/blob/master/config/v1/types_cluster_operator.go#L121-L133) to standardize across components.

#### OperatorAvailable
```
	OperatorAvailable ClusterStatusConditionType = "Available"
```
OperatorAvailable indicates that the binary maintained by the operator
(eg: openshift-apiserver for the openshift-apiserver-operator), is functional
and available in the cluster.

#### OperatorProgressing
```
	OperatorProgressing ClusterStatusConditionType = "Progressing"
```
Progressing indicates that the operator is actively making changes to the binary
maintained by the operator (eg: openshift-apiserver for the
openshift-apiserver-operator).

#### OperatorDegraded
```
	OperatorDegraded ClusterStatusConditionType = "Degraded"
```
Degraded indicates that the component operator is not functioning completely.
An example of a degraded state would be if there should be 5 copies of the
component running but only 4 are running. It may still be available, but it is
degraded.

#### Condition Matrix

| Condition        | Status           | Status  | Status  |
| :------------- |:-------------:|:-----:|:-----:|
| OperatorAvailable | True | True | True |
| OperatorProgressing | False | True | True |
| OperatorDegraded | False | False | True |
| Meaning | Component is 100% healthy and the Operator is idle | Component is functional but, either upgrading or healing | Component is functioning below capacity and an upgrade or heal is in progress |

| Condition        | Status           | Status  |
| :------------- |:-------------:|:-----:|
| OperatorAvailable | False | False |
| OperatorProgressing | False | True |
| OperatorDegraded | True | True |
| Meaning | Component and operator are in a failed state that requires human intervention.  Failed upgrade or failed heal | Component is in a failed state and an operator is healing |

| Condition        | Status           |
| :------------- |:-------------:|
| OperatorAvailable | False |
| OperatorProgressing | True |
| OperatorDegraded | False |
| Meaning | Operator is deploying the component |

## Readiness Probe
With a standardized set of conditions, the HCO should report the health of the
overall application back to OLM and the user.  This will be critial for sensitive
operations like upgrade, because OLM needs to know it shouldn't replace an
operator when it is in the middle of imporant work.

See this [issue](https://github.com/operator-framework/operator-lifecycle-manager/issues/922) for why we only want to report a readiness probe on the HCO
instead of on all component operators.
