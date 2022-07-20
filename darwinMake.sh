rm -rf ./release/darwin
mkdir -p -v ./release/darwin/cache
cp -r ./doc ./release/darwin/
cp -r ./public ./release/darwin/
rm -rf ./release/darwin/public/uploads
rm -rf ./release/darwin/public/*.txt
rm -rf ./release/darwin/public/*.xml
cp -r ./template ./release/darwin/
cp -r ./system ./release/darwin/
cp -r ./language ./release/darwin/
cp -r ./CHANGELOG.md ./release/darwin/
find ./release/darwin -name '.DS_Store' | xargs rm -f
cp -r ./start.sh ./release/darwin/
cp -r ./stop.sh ./release/darwin/
cp -r ./License ./release/darwin/
cp -r ./clientFiles ./release/darwin/
cp -r ./README.md ./release/darwin/
GOOS=darwin GOARCH=amd64 go build -ldflags '-w -s' -o ./release/darwin/anqicms kandaoni.com/anqicms/main