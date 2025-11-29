dist: deploy_linux deploy_windows
	mkdir -p dist
	cd deploy/linux && tar -czvf vnak_linux.tar.gz vnak
	mv deploy/linux/vnak_linux.tar.gz dist/
	rm -f deploy/windows/vnak_windows.zip
	cd deploy/windows && zip vnak_windows *.exe
	mv deploy/windows/vnak_windows.zip dist/

deploy_linux:
	go build -ldflags="-s -w"

deploy_windows:
	go build -ldflags="-s -w -H windowsgui" --tags=windowsqtstatic
