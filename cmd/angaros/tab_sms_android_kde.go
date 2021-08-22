package main

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/godbus/dbus/v5"

	"go.angaros.io/internal/dbutil"
	container2 "go.angaros.io/internal/fyneutil/container"
	widget2 "go.angaros.io/internal/fyneutil/widget"
	"go.angaros.io/internal/gateway/sms/android"
	"go.angaros.io/internal/gateway/sms/android/kde"
)

func tabSmsAndroidKde(w fyne.Window) *container.TabItem {
	refreshChan := make(chan struct{}, 1)
	tablePage := container2.NewTable(
		w,
		refreshChan,
		[]widget2.TableAttribute{
			{Name: "Actions", Actions: true},
			{Name: "Android ID", Field: "AndroidID", Width: 175},
			{Name: "Name", Field: "Name", Width: 175},
			{Name: "Reachable", Field: "Reachable", Width: 125},
			{Name: "SMS Permission", Field: "PermissionSMS", Width: 125},
		},
		[]widget2.Action{
			{
				Name: "Save",
				Func: func(v dbutil.Saveable, refreshChan chan<- struct{}) func() {
					return func() {
						d := v.(*kde.Device)
						err := dbutil.UpsertSaveable(db, android.FromDeviceable(d))
						if err != nil {
							logAndShowError(fmt.Errorf("database error: %s", err), w)
						}
					}
				},
			},
		},
		func(refreshChan2 <-chan struct{}, t *widget2.Table, noticeLabel *widget.Label) {
			// renamed parameter to refreshChan2 because kde.GetDevices() needs refreshChan which is writtable. TODO: fix code smell
			devs := make(kde.Devices)
			var ctx context.Context
			var cancel context.CancelFunc
			var conn *dbus.Conn
			for range refreshChan2 {
				if cancel != nil {
					cancel()
					loggerInfo.Println("[kdeConnect] context cancelled")
				}
				if conn != nil {
					conn.Close()
					loggerInfo.Println("[kdeConnect] connection closed")
				}
				ctx, cancel = context.WithCancel(context.Background())
				var err error
				conn, err = kde.GetDevices(ctx, refreshChan, devs)
				if err != nil {
					loggerInfo.Println(err)
					noticeLabel.SetText(err.Error())
				} else {
					noticeLabel.SetText("")
				}
				t.UpdateAndRefresh(devs.ToSliceOfSaveables())
			}
			if cancel != nil {
				cancel()
				loggerInfo.Println("[kdeConnect] context cancelled")
			}
			if conn != nil {
				conn.Close()
				loggerInfo.Println("[kdeConnect] connection closed")
			}
		},
	)
	refreshChan <- struct{}{}
	return container.NewTabItemWithIcon("Connected Devices (KDE Connect)", theme.ComputerIcon(), tablePage)
}
