#!/usr/bin/env bash

set -euxo pipefail

go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen object \
    paths=./pkg/apis/calculations/v1 \
    output:dir=./pkg/apis/calculations/v1


go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen crd:allowDangerousTypes=true \
    paths=./pkg/apis/calculations/v1 \
    output:dir=./pkg/apis/calculations/v1
