package widget

import (
	"go.angaros.io/internal/dbutil"
)

type Action struct {
	Name string
	Func func(v dbutil.Saveable, refreshChan chan<- struct{}) func()
}
