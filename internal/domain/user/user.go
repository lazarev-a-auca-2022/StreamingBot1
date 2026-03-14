package user

import "time"

type User struct {
	ID        int64
	Username  string
	CreatedAt time.Time
	Banned    bool
}

func (u User) CanReceiveAccess() bool {
	return !u.Banned
}
