package watcher_test

import (
	"time"

	"github.com/cloudbase/garm/database/common"
)

func waitForPayload(ch <-chan common.ChangePayload, timeout time.Duration) *common.ChangePayload {
	select {
	case payload := <-ch:
		return &payload
	case <-time.After(timeout):
		return nil
	}
}
