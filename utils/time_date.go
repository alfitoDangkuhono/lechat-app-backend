package utils

import "time"

func GetNowTimeAndDateStrings() string {
	return time.Now().Format("2017-09-07 17:06:06")
}

func GetNowTimeAndDate() time.Time {
	return time.Now()
}
