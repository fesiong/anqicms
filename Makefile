# 在linux或mac下运行
all: open

build: clean
	mkdir -p -v ./release/windows/cache
	cp -r ./doc ./release/windows/
	cp -r ./public ./release/windows/
	rm -rf ./release/windows/public/uploads
	rm -rf ./release/windows/public/sitemap.txt
	rm -rf ./release/windows/public/robots.txt
	rm -rf ./release/windows/public/uploads
	cp -r ./template ./release/windows/
	cp -r ./system ./release/windows/
	cp -r ./CHANGELOG.md ./release/windows/
	cp -r ./stop.bat ./release/windows/
	cp -r ./License ./release/windows/
	cp -r ./README.md ./release/windows/
	GOOS=windows GOARCH=amd64 go build -ldflags '-w -s -H=windowsgui' -o ./release/windows/anqicms.exe kandaoni.com/anqicms/main

	mkdir -p -v ./release/linux/cache
	cp -r ./doc ./release/linux/
	cp -r ./public ./release/linux/
	rm -rf ./release/linux/public/uploads
	rm -rf ./release/linux/public/sitemap.txt
	rm -rf ./release/linux/public/robots.txt
	cp -r ./template ./release/linux/
	cp -r ./system ./release/linux/
	cp -r ./CHANGELOG.md ./release/linux/
	cp -r ./start.sh ./release/linux/
	cp -r ./stop.sh ./release/linux/
	cp -r ./License ./release/linux/
	cp -r ./README.md ./release/linux/
	GOOS=linux GOARCH=amd64 go build -ldflags '-w -s' -o ./release/linux/anqicms kandaoni.com/anqicms/main

open: build
	open ./release

clean:
	rm -rf ./release/windows
	rm -rf ./release/linux

start:
	go run kandaoni.com/anqicms/main

vet:
	go vet $(shell glide nv)
