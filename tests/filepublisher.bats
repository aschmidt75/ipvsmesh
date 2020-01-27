#!/usr/bin/env bats

load '/usr/local/lib/bats-assert/load.bash'

IPVSMESH_CONFIG="$(dirname $BATS_TEST_FILENAME)/temp/ipvsmesh.yaml"
IPVSMESH="$(dirname $BATS_TEST_FILENAME)/../release/ipvsmesh"
IPVSMESH_LOG="$(dirname $BATS_TEST_FILENAME)/temp/ipvsmesh-bats.log"
IPVSMESH_OUT1="$(dirname $BATS_TEST_FILENAME)/temp/ipvmesh-publisher-sample-1.yaml"
IPVSMESH_OUT2="$(dirname $BATS_TEST_FILENAME)/temp/ipvmesh-publisher-sample-1.yaml"

export IPVSMESH_SVCTIMEOUT=0

setup() {
    >${IPVSMESH_CONFIG}
    >${IPVSMESH_LOG}
    [ -f ${IPVSMESH_OUT1} ] && rm ${IPVSMESH_OUT1}
    [ -f ${IPVSMESH_OUT2} ] && rm ${IPVSMESH_OUT2}

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

@test "filepublisher: ipvsmesh config w/ labeled containers and a publisher yields correct publisher yaml output (fixt. -1)" {
    # make sure containers are running
    run docker ps
    assert_output --partial nginx
 
    #
    run ${IPVSMESH} --trace daemon start -f --log-file ${IPVSMESH_LOG} --config fixtures/filepublisher-1.yaml --once
    assert_success
    [ -f ${IPVSMESH_OUT1} ]

    run /bin/cat ${IPVSMESH_OUT1}
    assert_success
 
    assert_output --partial 'address: 10.1.2.3:80'
    assert_output --partial 'fromService=SampleService1'
    refute_output --partial '172\.' # no internal docker addresses

    refute_output --partial 'fromService=SampleService2'
}
