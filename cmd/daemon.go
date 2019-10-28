package cmd

import (
	"context"
	"fmt"
	"log/syslog"
	"net"
	"os"
	"time"

	"github.com/aschmidt75/ipvsmesh/config"
	"github.com/aschmidt75/ipvsmesh/daemon"
	"github.com/aschmidt75/ipvsmesh/localinterface"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	cli "github.com/jawher/mow.cli"
	log "github.com/sirupsen/logrus"
	lSyslog "github.com/sirupsen/logrus/hooks/syslog"
)

// Usages:
// daemon
func CmdDaemon(cmd *cli.Cmd) {
	cmd.Spec = "[-f|--foreground] [--log-file=<logfile>] [--sudo] [--gid=<groupid>]"
	var (
		foreground = cmd.BoolOpt("f foreground", false, "Run in foreground, do not daemonize")
		logfile    = cmd.StringOpt("log-file", "", "optional log file destination")
		sudo       = cmd.BoolOpt("sudo", false, "sudo to root when daemonizing")
		groupID    = cmd.IntOpt("gid", -1, "optional group ID for socket and log file creation")
	)

	cmd.Action = func() {
		_, err := os.Stat(config.Config().DaemonSocketPath)
		if err == nil {
			log.WithField("socket", config.Config().DaemonSocketPath).Fatal("Socket file already exists. Maybe another instance is already running? Remove socket file otherwise.")
		}

		if config.Config().DaemonizeFlag {
			*foreground = true

			hook, err := lSyslog.NewSyslogHook("", "", syslog.LOG_INFO|syslog.LOG_NOTICE|syslog.LOG_DEBUG, "fast-mesh")
			if err != nil {
				log.WithField("err", err).Fatal("unable to set up syslog")
			} else {
				log.AddHook(hook)
				log.WithField("hook", hook).Debug("syslog hook")
			}

			if logfile != nil && *logfile != "" {
				f, err := os.OpenFile(*logfile, os.O_WRONLY|os.O_APPEND, 0660)
				if err != nil {
					log.WithField("err", err).Fatal("unable to set up file logging")
				}
				if err := os.Chown(*logfile, -1, *groupID); err != nil {
					log.WithField("err", err).Warn("unable to chgrp for log file.")
				}
				log.WithField("logfile", *logfile).Trace("writing log to file")
				log.SetOutput(f)
			}
		}

		if *foreground == false {
			env := os.Environ()
			env = append(env, "IPVSMESH_DAEMONIZE=1")

			devnull, _ := os.Open(os.DevNull)

			attr := &os.ProcAttr{
				Files: []*os.File{devnull, devnull, devnull},
				Dir:   ".",
				Env:   env,
			}
			path, err := os.Executable()
			if err != nil {
				log.WithField("err", err).Fatal("Unable to determine my executable")
				os.Exit(1)
			}

			var args []string
			if *sudo {
				args = make([]string, len(os.Args)+2)
				for i, a := range os.Args {
					args[i+2] = a
				}
				args[1] = "--preserve-env"
				path = "/usr/bin/sudo"
				args[0] = path
			} else {
				args = make([]string, len(os.Args))
				copy(args, os.Args)
			}
			log.WithFields(log.Fields{"path": path, "args": args}).Trace("Respawning...")

			process, err := os.StartProcess(
				path,
				args,
				attr,
			)
			if err != nil {
				log.WithField("err", err).Fatal("Unable to respawn myself")
				os.Exit(2)
			}

			fmt.Printf("%d\n", process.Pid)

			process.Release()
			os.Exit(0)
		} else {
			log.Debug("Running in foreground")

			ds := daemon.NewService(*groupID)

			// kick off some sample workers, register as service
			demoService := daemon.NewDemoWorker("bla")
			ds.Register(&demoService.StoppableByChan)
			log.WithField("ds", ds).Trace("registered")
			go demoService.DemoWorker()

			demoService = daemon.NewDemoWorker("bli")
			ds.Register(&demoService.StoppableByChan)
			log.WithField("ds", ds).Trace("registered")
			go demoService.DemoWorker()

			demoService = daemon.NewDemoWorker("blub")
			ds.Register(&demoService.StoppableByChan)
			log.WithField("ds", ds).Trace("registered")
			go demoService.DemoWorker()

			// main loop, wait for commands from cmdline client as per grpc calls
			ds.Start(context.Background())

		}
	}
}

func connect() *grpc.ClientConn {
	_, err := os.Stat(config.Config().DaemonSocketPath)
	if err != nil {
		log.WithField("socket", config.Config().DaemonSocketPath).Fatal("Socket file not present. Maybe daemon is not running?")
	}

	var conn *grpc.ClientConn

	dialOptions := grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout("unix", addr, timeout)
	})
	if config.Config().TLS {
		creds, err := credentials.NewClientTLSFromFile(config.Config().TLSCertFile, "fm-grpc-tls-comm")
		if err != nil {
			log.Fatal("unable to load TLS key or certificate from file. Check --tls* parameters.")
		}

		conn, err = grpc.Dial(
			config.Config().DaemonSocketPath, grpc.WithTransportCredentials(creds), dialOptions)
		if err != nil {
			log.WithField("err", err).Fatal("Unable to create TLS dialing socket.")
		}

	} else {
		conn, err = grpc.Dial(
			config.Config().DaemonSocketPath, grpc.WithInsecure(), dialOptions)
		if err != nil {
			log.WithField("err", err).Fatal("Unable to create dialing socket.")
		}
	}

	return conn
}

func CmdDaemonStop(cmd *cli.Cmd) {
	cmd.Action = func() {
		// connect to backend
		conn := connect()
		defer conn.Close()

		client := localinterface.NewDaemonServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Config().DaemonConnTimeoutSecs)*time.Second)
		defer cancel()

		_, err := client.Stop(ctx, &localinterface.Empty{})
		if err != nil {
			log.WithField("err", err).Error("error stopping daemon.")
			return
		}
	}
}
