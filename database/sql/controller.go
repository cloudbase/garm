package sql

import (
	runnerErrors "garm/errors"
	"garm/params"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

func (s *sqlDatabase) ControllerInfo() (params.ControllerInfo, error) {
	var info ControllerInfo
	q := s.conn.Model(&ControllerInfo{}).First(&info)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.ControllerInfo{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching controller info")
		}
		return params.ControllerInfo{}, errors.Wrap(q.Error, "fetching controller info")
	}
	return params.ControllerInfo{
		ControllerID: info.ControllerID,
	}, nil
}

func (s *sqlDatabase) InitController() (params.ControllerInfo, error) {
	if _, err := s.ControllerInfo(); err == nil {
		return params.ControllerInfo{}, runnerErrors.NewConflictError("controller already initialized")
	}

	newID, err := uuid.NewV4()
	if err != nil {
		return params.ControllerInfo{}, errors.Wrap(err, "generating UUID")
	}

	newInfo := ControllerInfo{
		ControllerID: newID,
	}

	q := s.conn.Save(&newInfo)
	if q.Error != nil {
		return params.ControllerInfo{}, errors.Wrap(q.Error, "saving controller info")
	}

	return params.ControllerInfo{
		ControllerID: newInfo.ControllerID,
	}, nil
}
