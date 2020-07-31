GO=go
GOTEST=$(GO) test -race
GOCOVER=$(GO) tool cover
COVEROUT=./cover/c.out

.PHONY: test

test:
	$(GOTEST) -cover -coverprofile=$(COVEROUT) . && $(GOCOVER) -html=$(COVEROUT)
minio:
	docker-compose -f docker-compose-minio.yml up -d --build --force-recreate
mc:
	docker-compose -f docker-compose-minio.yml exec minio-mc sh
down:
	docker-compose -f docker-compose-minio.yml down
	docker-compose -f docker-compose-minio.yml rm -fsv