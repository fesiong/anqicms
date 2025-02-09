# only for windows
echo "🔨 Building for current platform..."
mkdir -p -v ./release/windows/cache
mkdir -p -v ./release/windows/public
mkdir -p -v ./release/windows/source
cp -r ./doc ./release/windows/
cp -r ./public/static ./release/windows/public/
cp -r ./public/*.xsl ./release/windows/public/
cp -r ./template ./release/windows/
cp -r ./locales ./release/windows/
cp -r ./CHANGELOG.md ./release/windows/
cp -r ./License ./release/windows/
cp -r ./clientFiles ./release/windows/
cp -r ./README.md ./release/windows/
cp -r ./dictionary.txt ./release/windows/
cp -r ./source/cwebp_windows_amd64.exe ./release/windows/source/
find ./release/windows -name '.DS_Store' | xargs rm -f
go run ico/gen_syso.go
go build -trimpath -ldflags '-w -s -H=windowsgui' -o ./release/windows/anqicms.exe kandaoni.com/anqicms/main
rm -rf anqicms.syso