GO=go
GOTEST=$(GO) test -race
GOCOVER=$(GO) tool cover
COVEROUT=./cover/c.out

.PHONY: test

test:
	$(GOTEST) -cover -coverprofile=$(COVEROUT) . && $(GOCOVER) -html=$(COVEROUT)
minio:
	docker-compose -f docker-compose-minio.yml up -d
mc:
	docker-compose -f docker-compose-minio.yml run --entrypoint="/bin/sh" minio-mc
down:
	docker-compose -f docker-compose-minio.yml down