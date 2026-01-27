package tlssecprofile

import (
	"reflect"
	"sync"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
)

type cachedTLSSecurityProfile struct {
	profile *openshiftconfigv1.TLSSecurityProfile
	lock    *sync.RWMutex
}

var (
	cachedAPIServerProfile      = newSecurityProfileCache()
	hyperConvergedServerProfile = newSecurityProfileCache()
)

func newSecurityProfileCache() *cachedTLSSecurityProfile {
	return &cachedTLSSecurityProfile{
		lock: &sync.RWMutex{},
	}
}

func (c *cachedTLSSecurityProfile) get() *openshiftconfigv1.TLSSecurityProfile {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.profile == nil {
		return nil
	}

	return c.profile.DeepCopy()
}

func (c *cachedTLSSecurityProfile) set(profile *openshiftconfigv1.TLSSecurityProfile) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if reflect.DeepEqual(c.profile, profile) {
		return false
	}

	c.profile = profile
	return true
}

func getAPIServerProfile() *openshiftconfigv1.TLSSecurityProfile {
	return cachedAPIServerProfile.get()
}

func setAPIServerProfile(profile *openshiftconfigv1.TLSSecurityProfile) bool {
	return cachedAPIServerProfile.set(profile)
}

func getHyperConvergedProfile() *openshiftconfigv1.TLSSecurityProfile {
	return hyperConvergedServerProfile.get()
}

func setHyperConvergedProfile(profile *openshiftconfigv1.TLSSecurityProfile) bool {
	return hyperConvergedServerProfile.set(profile)
}
