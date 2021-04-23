build:
	go build -o . ./cmd/...
	docker build -f images/dispatcher/Dockerfile -t dispatcher .
	docker build -f images/worker/Dockerfile -t worker .
	docker build -f images/janitor/Dockerfile -t janitor .
	docker build -f images/result-collector/Dockerfile -t result-collector .
	docker build -f images/apiserver/Dockerfile -t apiserver .
.PHONY: all build


namespace:
	oc create --dry-run -f ./cluster/vega-namespace --dry-run -o yaml | oc apply -f -
.PHONY: namespace

dispatcher:
	oc create --dry-run -f ./cluster/dispatcher --dry-run -o yaml | oc apply -f -
.PHONY: dispatcher

worker:
	oc create --dry-run -f ./cluster/worker --dry-run -o yaml | oc apply -f -
.PHONY: worker

result-collector:
	oc create -f ./cluster/result-collector --dry-run -o yaml | oc apply -f - 
.PHONY: result-collector

janitor:
	oc create -f ./cluster/janitor --dry-run -o yaml | oc apply -f - 
.PHONY: janitor

apiserver:
	oc process -f ./cluster/apiserver | oc apply -f -
.PHONY: apiserver

redis:
	oc process -f ./cluster/redis | oc apply -f -
.PHONY: redis

storage:
ifdef NFS_SERVER_IP
	oc process -f ./cluster/storage/nfs-storage-template.yaml NFS_SEVER_IP=${NFS_SERVER_IP} | oc apply -f -
else
	@echo "NFS_SERVER_IP variable must be specified."
endif	
.PHONY: storage

deploy: storage dispatcher worker result-collector janitor apiserver redis
