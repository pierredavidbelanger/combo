build:
	go build

install: build
	install combo /usr/local/bin
