#!/usr/bin/env bats -p

@test "deploy csi-lvm-controller" {
    run kubectl apply -f /files/controller.yaml
    [ "$status" -eq 0 ]
}


@test "csi-lvm-controller running" {
    run kubectl wait -n csi-lvm --for=condition=Available deployment/csi-lvm-controller --timeout=20s
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
    run kubectl apply -f /files/block.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "pod/volume-test-block created" ]
} 

@test "linear pvc bound" {
    run kubectl wait -n default --for=condition=ready pod/volume-test --timeout=80s
    run kubectl get pvc lvm-pvc-linear -o jsonpath="{.metadata.name},{.status.phase}" 
    [ "$status" -eq 0 ]
    [ "$output" = "lvm-pvc-linear,Bound" ]
} 

@test "linear pod running" {
    run kubectl get pods volume-test -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "volume-test,Running" ]
}

@test "block pvc bound" {
    run kubectl wait -n default --for=condition=ready pod/volume-test-block --timeout=40s
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
    [ "${lines[0]}" = "pod \"volume-test\" deleted" ]
} 

@test "delete block pod" {
    run kubectl delete -f /files/block.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "pod \"volume-test-block\" deleted" ]
} 

@test "simulate reboot" {
    run kubectl apply -f /files/simulate_reboot.yaml
    [ "$status" -eq 0 ]
} 

@test "deploy linear pod again" {
    run sleep 20
    run kubectl apply -f /files/linear.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "pod/volume-test created" ]
} 

@test "deploy block pod again" {
    run kubectl apply -f /files/block.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "pod/volume-test-block created" ]
} 

@test "linear pod stays pending" {
    run kubectl wait -n default --for=condition=ready pod/volume-test --timeout=20s
    run kubectl get pods volume-test -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "volume-test,Pending" ]
}

@test "block pod stays pending" {
    run kubectl wait -n default --for=condition=ready pod/volume-test-block --timeout=10s
    run kubectl get pods volume-test-block -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "volume-test-block,Pending" ]
}
@test "start reviver" {
    run kubectl apply -f /files/reviver.yaml
    [ "$status" -eq 0 ]
} 

@test "linear pod running again" {
    run kubectl wait -n csi-lvm --for=condition=Available daemonset/csi-lvm-reviver --timeout=80s
    run kubectl wait -n default --for=condition=ready pod/volume-test --timeout=60s
    run kubectl get pods volume-test -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "volume-test,Running" ]
}

@test "block pod running again" {
    run kubectl wait -n default --for=condition=ready pod/volume-test-block --timeout=20s
    run kubectl get pods volume-test-block -o jsonpath="{.metadata.name},{.status.phase}"
    [ "$status" -eq 0 ]
    [ "$output" = "volume-test-block,Running" ]
}



@test "final delete linear pod" {
    run kubectl delete -f /files/linear.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "pod \"volume-test\" deleted" ]
}

@test "final delete block pod" {
    run kubectl delete -f /files/block.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "pod \"volume-test-block\" deleted" ]
}

@test "delete pvc" {
    run kubectl delete -f /files/pvc.yaml
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "persistentvolumeclaim \"lvm-pvc-block\" deleted" ]
    [ "${lines[1]}" = "persistentvolumeclaim \"lvm-pvc-linear\" deleted" ]
} 
