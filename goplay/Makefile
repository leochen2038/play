build:
	go build -ldflags="-s -w" goplay.go
	$(if $(shell command -v upx), upx goplay)
mac:
	GOOS=darwin go build -ldflags="-s -w" -o goplay-darwin goplay.go
	$(if $(shell command -v upx), upx goplay-darwin)
win:
	GOOS=windows go build -ldflags="-s -w" -o goplay.exe goplay.go
	$(if $(shell command -v upx), upx goplay.exe)
linux:
	GOOS=linux go build -ldflags="-s -w" -o goplay-linux goplay.go
	$(if $(shell command -v upx), upx goplay-linux)