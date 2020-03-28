#!/usr/bin/env bats -p

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

@test "clean up" {
    run sleep 90
    run kubectl delete -f /files/reviver.yaml
    run kubectl delete -f /files/controller.yaml
    run kubectl delete -f /files/namespace.yaml
    run sleep 90
    [ "$status" -eq 0 ]
}
