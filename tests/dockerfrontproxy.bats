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

    C1=$(docker run --name ipvsmeshbats1 --label app=nginx -d nginx)
    C2=$(docker run --name ipvsmeshbats2 --label app=nginx -d nginx)
    C3=$(docker run --name ipvsmeshbats3 --label nosuchapp=thisisnotmynginx -d nginx)
}

teardown() {
    for c in ipvsmeshbats1 ipvsmeshbats2 ipvsmeshbats3; do
        docker stop $c
        docker rm $c
    done
}

@test "dockerfrontproxy: ipvsmesh config w/ labeled containers yields correct ipvsctl yaml (fixt. -1)" {
    # make sure containers are running
    run docker ps
    assert_output --partial nginx
 
    #
    run ${IPVSMESH} --trace daemon start -f --log-file ${IPVSMESH_LOG} --config fixtures/dockerfrontproxy-1.yaml --once
    assert_success
    [ -f ${IPVSCTL_CONFIG} ]

    run /bin/cat ${IPVSCTL_CONFIG}
    assert_success
 
    assert_output --partial 'address: 10.0.0.1:80'
    assert_output --partial ipvsmeshbats1
    assert_output --partial ipvsmeshbats2
    refute_output --partial ipvsmeshbats3

}

@test "dockerfrontproxy: ipvsmesh config w/ labeled (nonexisting) containers yields correct (empty) ipvsctl yaml (fixt. -2)" {
    # make sure containers are running
    run docker ps
    assert_output --partial nginx
 
    #
    run ${IPVSMESH} --trace daemon start -f --log-file ${IPVSMESH_LOG} --config fixtures/dockerfrontproxy-2.yaml --once
    assert_success
    [ -f ${IPVSCTL_CONFIG} ]

    run /bin/cat ${IPVSCTL_CONFIG}
    assert_success
 
    assert_output --partial 'services: []'
    refute_output --partial 'address: 10.0.0.1:80'
    refute_output --partial ipvsmeshbats1
    refute_output --partial ipvsmeshbats2
    refute_output --partial ipvsmeshbats3

}
