### Kubevirt HCO upgrade procedure

##### Summary

In order to tell the OLM to start the upgrade process, one needs to point the relevant operator subscription to the updated CSV (ClusterServiceVersion).

It may be as simple as updating the subscription's **channel**, inside the currently used **catalog**. Or, in other cases, it may require updating the **catalogsource** first, to point to the updated image, one that will contain a new catalog, with a CSV for this new channel.

Once OLM will recognize that the subscription points to a new CSV, it will attempt to peform an update. By default this happens automatically, but this behaviour can be modified for each operator, by modifying the relevant **subscription**, in case one wants to manually approve each update.

Steps 1 and 4 describe how to modify the subscription update policy to manual, and how to approve the update, once one becomes available. These steps are not required for the default, automatic update flow.

##### 1. Modify the subscription update policy to manual (ONLY for manual approval flow)

Using UI:
Go to Operators -> Installed Operators
Select kubevirt-hyperconverged project
Click on the KubeVirt HyperConverged Cluster Operator
Click on the Subscription
Click on the Automatic under Approval
Select Manual and click on the Save button

Using CLI:
	
	# kubectl edit subs hco-subscription

Add the following line under the spec: section:

    installPlanApproval: Manual

##### 2. Update the CatalogSource to point to the registry-bundle that contains a more recent version of KubeVirt HCO CSV (ONLY if 2 registry-bundle images are used)

NOTE: This step of course is only required if 2 different registry-bundle images are used, one that contains the initial version and a second image that includes an updated version.

	# kubectl edit catalogsources -n openshift-marketplace hco-catalogsource

	Edit the image to point to the updated one (image: under spec: section).
	I.e. to the docker.io/lveyde/hco-registry:CRMyRLkM

OR

    # kubectl patch catalogsource hco-catalogsource -n openshift-marketplace -p '{"spec":{"image": "docker.io/lveyde/hco-registry:CRMyRLkM"}}' --type merge

##### 3. Update the Subscription’s channel
	# kubectl edit subs hco-subscription

	Edit the subscription to point to the new channel (channel: under spec: section).
I.e. to the channel 0.0.2

OR

	# kubectl patch subs hco-subscription -p '{"spec":{"channel": "0.0.2"}}' --type merge

##### 4. Approve the update (ONLY for manual approval flow)

Using the UI:
Go to Operators -> Installed Operators
Select kubevirt-hyperconverged project
Click on the 3 dots and then click on the Edit Subscription from the popup menu
Click on Subscriptions
Click on the hco-subscription
Once the system gets aware of the update it will show a link which says: 1 requires approval
Click on it, then click on the Preview Install Plan button
Click on the Approve button

##### 5. Verify that the update/upgrade indeed happens. It may take 5-10 minutes.

    # kubectl get csv

Check that the version of the CSV has been indeed updated.

