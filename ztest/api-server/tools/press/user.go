package press

import (
	"context"
)

type User struct {
	ID int64
}

func (u *User) Run(ctx context.Context) {}
func (u *User) login()                  {}
