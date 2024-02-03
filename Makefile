APP=On-Call
APPDIR=dist/$(APP).app
EXECUTABLE=$(APPDIR)/Contents/MacOS/on-call
ICONFILE=$(APPDIR)/Contents/Resources/Icon.icns

build: $(EXECUTABLE) $(ICONFILE)

$(EXECUTABLE): src/*.go
	GOOS=darwin GOARCH=amd64 go build -o "$@_amd64" $^
	GOOS=darwin GOARCH=arm64 go build -o "$@_arm64" $^
	lipo -create -output "$@" "$@_amd64" "$@_arm64"

run: build
	open $(APPDIR)

test:
	go test -v src/*.go

lint:
	golangci-lint run

install:
	cp -r $(APPDIR) /Applications

icon-clear-cache:
	sudo rm -rfv /Library/Caches/com.apple.iconservices.store
	sudo find /private/var/folders/ \( -name com.apple.dock.iconcache -or -name com.apple.iconservices \) -exec rm -rfv {} \;
	sleep 3
	sudo touch /Applications/*
	killall Dock; killall Finder

$(ICONFILE): assets/icon.png
	rm -rf assets/icon.iconset
	mkdir -p assets/icon.iconset
	for size in 16 32 64 128 256 512 1024; do \
	   sips -z $$size $$size assets/icon.png --out assets/icon.iconset/icon_$${size}x$${size}.png; \
	done
	iconutil -c icns -o $(ICONFILE) assets/icon.iconset

clean:
	rm -rf package
	rm -rf assets/icon.iconset
	rm -f assets/icon.icns
	rm -f $(EXECUTABLE)
	rm -f $(ICONFILE)
	rm -f dist/Applications

dmg: build
	ln -fs /Applications dist
	hdiutil create -volname $(APP) -srcfolder ./dist -ov ${PACKAGE}

install-dependencies:
	go mod download