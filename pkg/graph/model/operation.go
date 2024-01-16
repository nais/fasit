package model

import "time"

type Operation struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Type      string    `json:"type"`
	Detail    string    `json:"detail"`
	StartTime time.Time `json:"startTime"`
	Progress  *Progress `json:"progress"`
}
