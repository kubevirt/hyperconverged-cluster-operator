# CR Conditions and Readiness Probe
Conditions are..
	   _the latest available observations of an object's state. They are
	   an extension mechanism intended to be used when the details of an
	   observation are not a priori known or would not apply to all
	   instances of a given Kind._

Kubernetes conditions [documentation](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).

The HCO’s CR is a representation of the underlying component operators.  If the
object exists, then all components exist.  If the object doesn’t exist, all
components don’t exist.  However, the CR existence doesn’t help us with the
state where the operators exist, but are they healthy?  This is where conditions
on the HCO’s CR will answer this question by providing the observed health of
the underlying components.

## Condition Struct
We can use some of the CVO's [conditions](https://github.com/openshift/api/blob/master/config/v1/types_cluster_operator.go#L121-L133) to standardize across components.

Here's how the Condition struct will look...

```go
type OperatorStatusCondition struct {
   // type specifies the state of the operator's reconciliation functionality.
   Type ConditionType `json:"type"`

   // status of the condition, one either True or False.
   Status ConditionStatus `json:"status"`

   // lastTransitionTime is the time of the last update to the current status object.
   LastTransitionTime metav1.Time `json:"lastTransitionTime"`

   // reason is the reason for the condition's last transition.  Reasons are CamelCase
   Reason string `json:"reason,omitempty"`

   // message provides additional information about the current condition.
   // This is only to be consumed by humans.
   Message string `json:"message,omitempty"`
}
```

## ConditionType
`ConditionType` _specifies the state of the operator's reconciliation functionality_.
`ConditionType`s use `ConditionStatus` to report state.  The `ConditionStatus`es
we will use are either `True` or `False`.  The `ConditionStatus` object can also
be `Unknown`, but we won't use it because it's not clear what `Unknown` means in
terms of an application's lifecycle.

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

## Reason
`Reason` is _a one-word CamelCase reason for the condition's last transition_.

We'll use a series of lifecycle inspired prefixes paired with postfixes to
standardize values for `Reason`.

|         | -Failed  | -Succeeded | -Invalid | -InProgress |
| :------------- |:-------------:|:-----:|:-----:|
| Install- | InstallFailed | InstallSucceeded | InstallInvalid | InstallInProgress |
| Upgrade- | UpgradeFailed | UpgradeSucceded | UpgradeInvalid | UpgradeInProgress |
| Heal- | HealFailed | HealSucceeded | HealInvalid | HealInProgress |
| Configuration- | ConfigurationFailed | ConfigurationSucceeded | ConfigurationInvalid | ConfigurationInProgress |

|         | -Failed  | -Succeeded | -Invalid | -InProgress |
| :------------- |:-------------:|:-----:|:-----:|
| Meaning | The attempted operation **Failed** and the error is clear to the operator | The attempted operation **Succeeded** |  The attempted operation is missing something or is **Invalid** at this time | The attempted operation is **InProgress** |

## Message
`Message` is a _human-readable message indicating details about last transition_.

Explain why your CR has `Reason`.
