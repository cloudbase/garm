package pool

import "log"

func (r *basePoolManager) log(msg string, args ...interface{}) {
	msgArgs := []interface{}{
		r.helper.String(),
	}
	msgArgs = append(msgArgs, args...)
	log.Printf("[Pool mgr %s] "+msg, msgArgs...)
}
