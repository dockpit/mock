#build docker image
build: 
	docker build -t dockpit/mock:`cat VERSION` .
	docker tag -f dockpit/mock:`cat VERSION` dockpit/mock:latest

#push image to docker registry
push: build
	docker push dockpit/mock

#run all unit tests
test:
	go test ./... 