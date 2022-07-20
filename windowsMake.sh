rm -rf ./release/windows
mkdir -p -v ./release/windows/cache
cp -r ./doc ./release/windows/
cp -r ./public ./release/windows/
rm -rf ./release/windows/public/uploads
rm -rf ./release/windows/public/*.txt
rm -rf ./release/windows/public/*.xml
cp -r ./template ./release/windows/
cp -r ./system ./release/windows/
cp -r ./language ./release/windows/
find ./release/windows -name '.DS_Store' | xargs rm -f
cp -r ./CHANGELOG.md ./release/windows/
cp -r ./stop.bat ./release/windows/
cp -r ./License ./release/windows/
cp -r ./clientFiles ./release/windows/
cp -r ./README.md ./release/windows/
GOOS=windows GOARCH=amd64 go build -ldflags '-w -s -H=windowsgui' -o ./release/windows/anqicms.exe kandaoni.com/anqicms/main
