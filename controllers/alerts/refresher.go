package alerts

import (
	"sync"
)

type Refresher interface {
	setShouldRefresh()
	refresh(f func() error) error
}

type refresher struct {
	*sync.Mutex
	upToDate bool
}

func NewRefresher() Refresher {
	return &refresher{
		Mutex:    &sync.Mutex{},
		upToDate: true,
	}
}

func (r *refresher) setShouldRefresh() {
	r.Lock()
	defer r.Unlock()

	r.upToDate = false
}

func (r *refresher) refresh(f func() error) error {
	r.Lock()
	defer r.Unlock()

	if r.upToDate {
		return nil
	}

	if err := f(); err != nil {
		return err
	}

	r.upToDate = true

	return nil
}
