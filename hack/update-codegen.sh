#!/usr/bin/env bash

set -euxo pipefail

go run sigs.k8s.io/controller-tools/cmd/controller-gen object \
    paths=./pkg/apis/calculations/v1 \
    output:dir=./pkg/apis/calculations/v1
go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:allowDangerousTypes=true \
    paths=./pkg/apis/calculations/v1 \
    output:dir=./pkg/apis/calculations/v1


go run sigs.k8s.io/controller-tools/cmd/controller-gen object \
    paths=./pkg/apis/workers/v1 \
    output:dir=./pkg/apis/workers/v1
go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:allowDangerousTypes=true \
    paths=./pkg/apis/workers/v1 \
    output:dir=./pkg/apis/workers/v1

go run sigs.k8s.io/controller-tools/cmd/controller-gen object \
    paths=./pkg/apis/calculationbulk/v1 \
    output:dir=./pkg/apis/calculationbulk/v1
go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:allowDangerousTypes=true \
    paths=./pkg/apis/calculationbulk/v1 \
    output:dir=./pkg/apis/calculationbulk/v1


go run sigs.k8s.io/controller-tools/cmd/controller-gen object \
    paths=./pkg/apis/calculationbulkfactory/v1 \
    output:dir=./pkg/apis/calculationbulkfactory/v1
go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:allowDangerousTypes=true \
    paths=./pkg/apis/calculationbulkfactory/v1 \
    output:dir=./pkg/apis/calculationbulkfactory/v1
