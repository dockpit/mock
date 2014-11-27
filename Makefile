#build docker image
build: 
	docker build -t dockpit/mock:`cat VERSION` .
	docker tag dockpit/mock:`cat VERSION` dockpit/mock:latest

#run all unit tests
test:
	go test ./... 