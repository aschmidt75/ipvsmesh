#!/usr/bin/env bats

load '/usr/local/lib/bats-assert/load.bash'

IPVSMESH_CONFIG="$(dirname $BATS_TEST_FILENAME)/temp/ipvsmesh.yaml"
IPVSMESH="$(dirname $BATS_TEST_FILENAME)/../release/ipvsmesh"
IPVSMESH_LOG="$(dirname $BATS_TEST_FILENAME)/temp/ipvsmesh-bats.log"
IPVSCTL_CONFIG="$(dirname $BATS_TEST_FILENAME)/temp/ipvsctl-bats.yaml"

export IPVSMESH_SVCTIMEOUT=0

setup() {
    >${IPVSMESH_CONFIG}
    >${IPVSMESH_LOG}
    [ -f ${IPVSCTL_CONFIG} ] && rm ${IPVSCTL_CONFIG}

    ${IPVSMESH} --trace daemon start --log-file ${IPVSMESH_LOG} --config fixtures/dockerfrontproxy-1.yaml 

    sleep 2
}

teardown() {
    ${IPVSMESH} daemon stop

    [ -f ${IPVSMESH_CONFIG} ] && rm ${IPVSMESH_CONFIG}
    #[ -f ${IPVSCTL_CONFIG} ] && rm ${IPVSCTL_CONFIG}

    for c in ipvsmeshbats1 ipvsmeshbats2 ipvsmeshbats3; do
        docker inspect $c >/dev/null 2>&1
        if [[ $? -eq 0 ]]; then
            docker stop $c
            docker rm $c
        fi
    done
}

@test "dockerfrontproxy: ipvsmesh config w/ labeled containers yields correct ipvsctl yaml (fixt. -1, adding/removing containers)" {
    # make sure containers are running
    run docker ps
    refute_output --partial nginx
 
    docker run --name ipvsmeshbats1 --label app=nginx -d nginx
    sleep 2

    run /bin/cat ${IPVSCTL_CONFIG}
    assert_success
 
    assert_output --partial 'address: 10.0.0.1:80'
    assert_output --partial ipvsmeshbats1
    refute_output --partial ipvsmeshbats2
    refute_output --partial ipvsmeshbats3

    docker run --name ipvsmeshbats2 --label app=nginx -d nginx
    sleep 2

    run /bin/cat ${IPVSCTL_CONFIG}
    assert_success
 
    assert_output --partial 'address: 10.0.0.1:80'
    assert_output --partial ipvsmeshbats1
    assert_output --partial ipvsmeshbats2
    refute_output --partial ipvsmeshbats3

    docker run --name ipvsmeshbats3 --label nosuchapp=someothernginx -d nginx
    sleep 2

    run /bin/cat ${IPVSCTL_CONFIG}
    assert_success
 
    assert_output --partial 'address: 10.0.0.1:80'
    assert_output --partial ipvsmeshbats1
    assert_output --partial ipvsmeshbats2
    refute_output --partial ipvsmeshbats3

    for c in ipvsmeshbats1 ipvsmeshbats2 ipvsmeshbats3; do
        docker stop $c
        docker rm $c
    done

}
