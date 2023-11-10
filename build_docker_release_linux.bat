docker run --rm -v "$PWD":/media/sf_SamWaf -w /media/sf_SamWaf -e CGO_ENABLED=1 -e GOPROXY=https://goproxy.cn,direct golang:1.19 go build -v -ldflags="-X SamWaf/global.GWAF_RELEASE=true -X SamWaf/global.GWAF_RELEASE_VERSION_NAME=20231109 -X SamWaf/global.GWAF_RELEASE_VERSION=v1.0.124 -s -w -extldflags "-static"" -o /media/sf_SamWaf/release/SamWafLinux64 main.go