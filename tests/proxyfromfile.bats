#!/usr/bin/env bats

IPVSMESH="$(dirname $BATS_TEST_FILENAME)/../release/ipvsmesh"
IPVSMESH_CONFIG="$(dirname $BATS_TEST_FILENAME)/temp/ipvsmesh-bats.yaml"
IPVSMESH_LOG="$(dirname $BATS_TEST_FILENAME)/temp/ipvsmesh-bats.log"
IPVSCTL_CONFIG="$(dirname $BATS_TEST_FILENAME)/temp/ipvsctl-bats.yaml"

setup() {
    >${IPVSMESH_CONFIG}
    >${IPVSMESH_LOG}
    [ -f ${IPVSCTL_CONFIG} ] && rm ${IPVSCTL_CONFIG}

    ${IPVSMESH} --trace daemon start --log-file ${IPVSMESH_LOG} --config ${IPVSMESH_CONFIG}
}

teardown() {
    ${IPVSMESH} daemon stop

#    [ -f ${IPVSMESH_CONFIG} ] && rm ${IPVSMESH_CONFIG}
#    [ -f ${IPVSCTL_CONFIG} ] && rm ${IPVSCTL_CONFIG}
}

@test "ipvsmesh config w/ data file yields correct ipvsctl yaml (-1)" {
    sleep 1
    cp fixtures/proxyfromfile-1.yaml ${IPVSMESH_CONFIG} && sleep 1

    [ -f ${IPVSCTL_CONFIG} ]

    run /bin/cat ${IPVSCTL_CONFIG}

	[ "$status" -eq 0 ]
    [[ "$output" =~ address:\ 10\.0\.0\.1:80 ]] 
    [[ "$output" =~ address:\ 20\.1\.0\.1:80 ]] 
    [[ "$output" =~ address:\ 20\.1\.0\.2 ]] 
}
