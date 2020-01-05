# ipvsmesh

`ipvsmesh` is a daemon which operates on top of `ipvsctl` and enables ipvs-based load balancing, integrated with tools on the
application level.

## Features

* automatically balance traffic ... 
    * to local docker containers, identified by container labels
    * to local processes with listeners in specific port ranges
* balance traffic to remote services, configurable ...
    * from local configuration files
* configure from yaml file, with automatic reconfiguration

## License

(C) 2019,2020 @aschmidt, Apache 2 License.