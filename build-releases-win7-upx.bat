SET CGO_ENABLED=1
SET GOOS=windows
SET GOARCH=amd64
SET GIN_MODE=release
go build -ldflags="-X SamWaf/global.GWAF_RUNTIME_WIN7_VERSION=true -X SamWaf/global.GWAF_RELEASE=true -X SamWaf/global.GWAF_RELEASE_VERSION_NAME=20250407 -X SamWaf/global.GWAF_RELEASE_VERSION=v1.3.11 -s -w" -o %cd%/release/SamWaf64.exe main.go && %cd%/upx/win64/upx -9  %cd%/release/SamWaf64.exe