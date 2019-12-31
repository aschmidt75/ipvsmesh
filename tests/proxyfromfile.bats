#!/usr/bin/env bats

IPVSMESH="$(dirname $BATS_TEST_FILENAME)/../release/ipvsmesh"
IPVSMESH_LOG="$(dirname $BATS_TEST_FILENAME)/temp/ipvsmesh-bats.log"
IPVSCTL_CONFIG="$(dirname $BATS_TEST_FILENAME)/temp/ipvsctl-bats.yaml"

export IPVSMESH_SVCTIMEOUT=0

@test "proxyfromfile: ipvsmesh config w/ data file yields correct ipvsctl yaml (fixt. -1, text data)" {
    >${IPVSCTL_CONFIG}
    >${IPVSMESH_LOG}

    run ${IPVSMESH} --trace daemon start -f --log-file ${IPVSMESH_LOG} --config fixtures/proxyfromfile-1.yaml --once
	[ "$status" -eq 0 ]

    [ -f ${IPVSCTL_CONFIG} ]

    run /bin/cat ${IPVSCTL_CONFIG}

    [[ "$output" =~ address:\ 10\.0\.0\.1:80 ]] 
    [[ "$output" =~ address:\ 20\.1\.0\.1:80 ]] 
    [[ "$output" =~ address:\ 20\.1\.0\.2 ]] 

    [ -f ${IPVSCTL_CONFIG} ] && rm ${IPVSCTL_CONFIG}
}

@test "proxyfromfile: ipvsmesh config w/ data file yields correct ipvsctl yaml (fixt. -2, json data)" {
    >${IPVSCTL_CONFIG}
    >${IPVSMESH_LOG}

    run ${IPVSMESH} --trace daemon start -f --log-file ${IPVSMESH_LOG} --config fixtures/proxyfromfile-2.yaml --once
	[ "$status" -eq 0 ]

    [ -f ${IPVSCTL_CONFIG} ]

    run /bin/cat ${IPVSCTL_CONFIG}

    [[ "$output" =~ address:\ 10\.0\.0\.1:80 ]] 
    [[ ! "$output" =~ address:\ 20\.1\.0\.1:80 ]] 
    [[ ! "$output" =~ address:\ 20\.1\.0\.2 ]] 
    [[ "$output" =~ address:\ 20\.2\.0\.1:80 ]] 
    [[ "$output" =~ address:\ 20\.2\.0\.2 ]] 

    [ -f ${IPVSCTL_CONFIG} ] && rm ${IPVSCTL_CONFIG}
}
