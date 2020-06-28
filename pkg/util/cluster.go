package util

import (
	"context"
	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterInfo interface {
	CheckRunningInOpenshift(ctx context.Context) error
	IsOpenshift() bool
}

type ClusterInfoImp struct {
	client             client.Reader
	firstTime          bool
	runningInOpenshift bool
}

func NewClusterInfo(c client.Reader) ClusterInfo {
	return &ClusterInfoImp{
		client:             c,
		firstTime:          true,
		runningInOpenshift: false,
	}
}

func (c ClusterInfoImp) CheckRunningInOpenshift(ctx context.Context) error {
	if !c.firstTime {
		return nil
	}

	cvList := &configv1.ClusterVersionList{}
	err := c.client.List(ctx, cvList)
	if err != nil {
		if meta.IsNoMatchError(err) {
			c.runningInOpenshift = false
		} else {
			return err
		}
	} else {
		c.runningInOpenshift = true
	}

	c.firstTime = false

	return nil

}

func (c ClusterInfoImp) IsOpenshift() bool {
	return c.runningInOpenshift
}
