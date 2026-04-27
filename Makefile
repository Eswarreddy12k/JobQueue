APP_NAME       := mini-job-queue
AWS_REGION     := us-east-1
AWS_ACCOUNT_ID := $(shell aws sts get-caller-identity --query Account --output text 2>/dev/null)
ECR_REPO       := $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(APP_NAME)
IMAGE_TAG      := $(shell git rev-parse --short HEAD)

.PHONY: build test docker-build docker-push deploy

## Build all three Go binaries locally
build:
	CGO_ENABLED=0 go build -o bin/api ./cmd/api
	CGO_ENABLED=0 go build -o bin/worker ./cmd/worker
	CGO_ENABLED=0 go build -o bin/autoscaler ./cmd/autoscaler

## Run tests
test:
	go test ./...

## Build Docker image tagged with git SHA
docker-build:
	docker build -t $(APP_NAME):$(IMAGE_TAG) .
	docker tag $(APP_NAME):$(IMAGE_TAG) $(ECR_REPO):$(IMAGE_TAG)
	docker tag $(APP_NAME):$(IMAGE_TAG) $(ECR_REPO):latest

## Authenticate with ECR and push the image
docker-push:
	aws ecr get-login-password --region $(AWS_REGION) | \
		docker login --username AWS --password-stdin $(ECR_REPO)
	docker push $(ECR_REPO):$(IMAGE_TAG)
	docker push $(ECR_REPO):latest

## Rolling update of all three deployments to the current git SHA
deploy:
	kubectl set image deployment/api api=$(ECR_REPO):$(IMAGE_TAG) -n jobqueue
	kubectl set image deployment/worker worker=$(ECR_REPO):$(IMAGE_TAG) -n jobqueue
	kubectl set image deployment/autoscaler autoscaler=$(ECR_REPO):$(IMAGE_TAG) -n jobqueue
	kubectl rollout status deployment/api -n jobqueue --timeout=120s
	kubectl rollout status deployment/worker -n jobqueue --timeout=120s
	kubectl rollout status deployment/autoscaler -n jobqueue --timeout=120s
