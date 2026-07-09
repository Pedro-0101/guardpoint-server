package timeutil

import "time"

var BRT *time.Location

func init() {
	var err error
	BRT, err = time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		BRT = time.FixedZone("BRT", -3*60*60)
	}
}

func NowBRT() time.Time {
	return time.Now().In(BRT)
}
