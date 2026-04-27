package handler

import "time"

func durationFromOptionalSeconds(v *int) time.Duration {
	if v == nil || *v <= 0 {
		return 0
	}
	return time.Duration(*v) * time.Second
}
