module github.com/darkliquid/keyshell

go 1.27.2

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/go-freedesktop/desktopentry v0.2.0
	github.com/godbus/dbus/v5 v5.2.4
	github.com/gogpu/gg v0.52.5
	github.com/gogpu/gogpu v0.54.0
	github.com/gogpu/gpucontext v0.31.3
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gputypes v0.8.0 // indirect
	github.com/gogpu/naga v0.19.0 // indirect
	github.com/gogpu/wgpu v0.34.2 // indirect
	github.com/rkoesters/xdg v0.0.1 // indirect
	golang.org/x/image v0.47.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

replace github.com/gogpu/gogpu => github.com/darkliquid/gogpu v0.54.1-0.20261019223216-43e5b4e065b1
