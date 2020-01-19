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

    nc -l 127.0.0.1 9008 &
    C1=$!

    nc -l 127.0.0.1 9009 &
    C2=$!

    nc -l 127.0.0.1 9109 &
    C3=$!


}

teardown() {
    kill $C1
    kill $C2
    kill $C3
}

@test "socketfrontproxy: ipvsmesh config w/ listening processes yields correct ipvsctl yaml (fixt. -1) LINUX ONLY" {
    # make sure listeners are running
    run ps -p $C1
    assert_success 
    run ps -p $C2
    assert_success 
    run ps -p $C3
    assert_success 
 
    #
    run ${IPVSMESH} --trace daemon start -f --log-file ${IPVSMESH_LOG} --config fixtures/socketfrontproxy-1.yaml --once
    assert_success
    [ -f ${IPVSCTL_CONFIG} ]

    run /bin/cat ${IPVSCTL_CONFIG}
    assert_success
 
    assert_output --partial 'address: 10.0.0.1:80'
    assert_output --partial 9008
    assert_output --partial 9009
    refute_output --partial 9109

}
