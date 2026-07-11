//go:build windows

package notify

import (
	"git.sr.ht/~jackmordaunt/go-toast/v2"
)

func ShowToast(title, message string) {
	n := toast.Notification{
		AppID: "ClickRAX",
		Title: title,
		Body:  message,
	}
	_ = n.Push()
}
