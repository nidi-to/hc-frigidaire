clean:
	rm -rf dist

build:
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=$(VERSION)" -o dist/hc-frigidaire-linux-amd64
	GOOS=linux GOARCH=arm go build -ldflags "-s -w -X main.version=$(VERSION)" -o dist/hc-frigidaire-linux-arm
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X main.version=$(VERSION)" -o dist/hc-frigidaire-darwin-amd64

package: build
	upx -9 dist/hc-frigidaire-*
	tar cfz dist/hc-frigidaire-linux-amd64.tgz -C dist ./hc-frigidaire-linux-amd64
	tar cfz dist/hc-frigidaire-linux-arm.tgz -C dist ./hc-frigidaire-linux-arm
	tar cfz dist/hc-frigidaire-darwin-amd64.tgz -C dist ./hc-frigidaire-darwin-amd64

release: clean package
