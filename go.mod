module pbs-win-backup

go 1.26

require (
	git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3
	github.com/cornelk/hashmap v1.0.8
	github.com/danieljoos/wincred v1.2.3
	github.com/dchest/siphash v1.2.3
	github.com/google/uuid v1.6.0
	github.com/hirochachacha/go-smb2 v1.1.0
	github.com/jlaffaye/ftp v0.2.1
	github.com/wailsapp/wails/v2 v2.13.0
	golang.org/x/net v0.54.0
	golang.org/x/sys v0.44.0
	pbscommon v0.0.0
	snapshot v0.0.0
)

require (
	github.com/alphadose/haxmap v1.4.1 // indirect
	github.com/bep/debounce v1.2.1 // indirect
	github.com/geoffgarside/ber v1.1.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jchv/go-winloader v0.0.0-20210711035445-715c2860da7e // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/labstack/echo/v4 v4.13.3 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/leaanthony/go-ansi-parser v1.6.1 // indirect
	github.com/leaanthony/gosod v1.0.4 // indirect
	github.com/leaanthony/slicer v1.6.0 // indirect
	github.com/leaanthony/u v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/samber/lo v1.49.1 // indirect
	github.com/st-matskevich/go-vss v0.3.3 // indirect
	github.com/tkrajina/go-reflector v0.5.8 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/wailsapp/go-webview2 v1.0.22 // indirect
	github.com/wailsapp/mimetype v1.4.1 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/exp v0.0.0-20221031165847-c99f073a8326 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace (
	clientcommon => ./third_party/proxmoxbackupclient_go-master/clientcommon
	pbscommon => ./third_party/proxmoxbackupclient_go-master/pbscommon
	snapshot => ./third_party/proxmoxbackupclient_go-master/snapshot
)
