#!/usr/bin/env bats -p

@test "prepare test files" {
    run sed -i "s/PRTAG/${PRTAG}/g;s/PRPULLPOLICY/${PRPULLPOLICY}/g;s/PRDEVICEPATTERN/${PRDEVICEPATTERN}/g" /files/*
    [ "$status" -eq 0 ]
}

@test "create namespace ${PRTAG}" {
    run kubectl apply -f /files/namespace.yaml
    [ "$status" -eq 0 ]
}

@test "deploy csi-lvm-controller" {
    run kubectl apply -f /files/controller.yaml
    [ "$status" -eq 0 ]
}

@test "csi-lvm-controller running" {
    run kubectl wait -n ${PRTAG} --for=condition=Available deployment/csi-lvm-controller-${PRTAG} --timeout=120s
    [ "$status" -eq 0 ]
    [ $(expr "$output" : "csi-lvm-controller.*Running") ]
}

@test "create pvc" {
    run kubectl apply -f /files/pvc.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "persistentvolumeclaim/lvm-pvc-block created" ]
    [ "${lines[1]}" = "persistentvolumeclaim/lvm-pvc-linear created" ]
}

@test "deploy linear pod" {
    run kubectl apply -f /files/linear.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "pod/volume-test created" ]
} 

@test "deploy block pod" {
    run sleep 30
    run kubectl apply -f /files/block.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "pod/volume-test-block created" ]
} 

@test "linear pvc bound" {
    run kubectl wait -n ${PRTAG} --for=condition=ready pod/volume-test --timeout=80s
    run kubectl get pvc -n ${PRTAG} lvm-pvc-linear -o jsonpath="{.metadata.name},{.status.phase}" 
    [ "$status" -eq 0 ]
    [ "$output" = "lvm-pvc-linear,Bound" ]
} 

@test "linear pod running" {
    run kubectl get pods -n ${PRTAG} volume-test -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "volume-test,Running" ]
}

@test "block pvc bound" {
    run kubectl wait -n ${PRTAG} --for=condition=ready pod/volume-test-block --timeout=80s
    run kubectl get pvc -n ${PRTAG} lvm-pvc-block -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "lvm-pvc-block,Bound" ]
} 

@test "block pod running" {
    run kubectl get pods -n ${PRTAG} volume-test-block -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "volume-test-block,Running" ]
}

