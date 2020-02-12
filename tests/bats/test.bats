#!/usr/bin/env bats 

@test "deploy csi-lvm-controller" {
    run kubectl apply -f /files/controller.yaml
    [ "$status" -eq 0 ]
}

@test "csi-lvm-controller running" {
    run sleep 2 && kubectl get pods -n csi-lvm
    [ "$status" -eq 0 ]
    [ $(expr "$output" : "csi-lvm-controller.*Running") ]
}

@test "deploy linear pod" {
    run kubectl apply -f /files/linear.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "persistentvolumeclaim/lvm-pvc-linear created" ]
    [ "${lines[1]}" = "pod/volume-test created" ]
} 

@test "deploy block pod" {
    run kubectl apply -f /files/block.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "persistentvolumeclaim/lvm-pvc-block created" ]
    [ "${lines[1]}" = "pod/volume-test-block created" ]
} 

@test "linear pvc bound" {
    run sleep 40
    run kubectl get pvc lvm-pvc-linear -o jsonpath="{.metadata.name},{.status.phase}" 
    [ "$status" -eq 0 ]
    [ "$output" = "lvm-pvc-linear,Bound" ]
} 

@test "linear pod running" {
    run sleep 20 
    run kubectl get pods volume-test -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "volume-test,Running" ]
}

@test "block pvc bound" {
    run kubectl get pvc lvm-pvc-block -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "lvm-pvc-block,Bound" ]
} 

@test "block pod running" {
    run kubectl get pods volume-test-block -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "volume-test-block,Running" ]
}

@test "delete linear pod" {
    run kubectl delete -f /files/linear.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "persistentvolumeclaim \"lvm-pvc-linear\" deleted" ]
    [ "${lines[1]}" = "pod \"volume-test\" deleted" ]
} 

@test "delete block pod" {
    run kubectl delete -f /files/block.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "persistentvolumeclaim \"lvm-pvc-block\" deleted" ]
    [ "${lines[1]}" = "pod \"volume-test-block\" deleted" ]
} 

@test "linear pvc deleted" {
    run sleep 5
    run kubectl get pvc lvm-pvc-linear -o jsonpath="{.metadata.name},{.status.phase}" 
    [ "$status" -ne 0 ]
} 

@test "block pvc deleted" {
    run kubectl get pvc lvm-pvc-block -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -ne 0 ]
} 
