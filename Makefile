# 在linux下运行
all: open

build: clean
	mkdir -p -v ./release/linux/cache
	cp -r ./doc ./release/linux/
	cp -r ./public ./release/linux/
	rm -rf ./release/linux/public/uploads
	rm -rf ./release/linux/public/*.txt
	rm -rf ./release/linux/public/*.xml
	cp -r ./template ./release/linux/
	cp -r ./system ./release/linux/
	cp -r ./language ./release/linux/
	cp -r ./CHANGELOG.md ./release/linux/
	find ./release/linux -name '.DS_Store' | xargs rm -f
	cp -r ./start.sh ./release/linux/
	cp -r ./stop.sh ./release/linux/
	cp -r ./License ./release/linux/
	cp -r ./clientFiles ./release/linux/
	cp -r ./README.md ./release/linux/
	cp -r ./dictionary.txt ./release/linux/
	dos2unix ./release/linux/start.sh
	dos2unix ./release/linux/stop.sh
	GOOS=linux GOARCH=amd64 go build -ldflags '-w -s' -o ./release/linux/anqicms kandaoni.com/anqicms/main

open: build

clean:
	rm -rf ./release/linux

start:
	go run kandaoni.com/anqicms/main

vet:
	go vet $(shell glide nv)
