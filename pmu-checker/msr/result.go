package msr

import (
	"encoding/json"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type Result struct {
	PMUActive  int               `json:"active_pmus"`
	PMUDetails map[string]string `json:"details"`
}

func (r Result) String() string {
	js, err := json.MarshalIndent(r, "", "\t")
	if err != nil {
		log.Error(errors.Wrap(err, "result could not be converted to json"))
		return ""
	}
	return string(js)
}
