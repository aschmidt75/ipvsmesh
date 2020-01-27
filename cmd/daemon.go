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

// Daemon controls the daemon command
func Daemon(cmd *cli.Cmd) {
	cmd.Command("start", "starts the daemon", DaemonStart)
	cmd.Command("stop", "stops the daemon", DaemonStop)
}

// DaemonStart starts the daemon either on foreground or background mode
func DaemonStart(cmd *cli.Cmd) {
	cmd.Spec = "[-f|--foreground] [--log-file=<logfile>] [--sudo] [--gid=<groupid>] [--config=<configfile>] [--once]"
	var (
		foreground = cmd.BoolOpt("f foreground", false, "Run in foreground, do not daemonize")
		once       = cmd.BoolOpt("o once", false, "Run loop only once, exit after first cycle")
		logfile    = cmd.StringOpt("log-file", "", "optional log file destination. Default destination is syslog")
		sudo       = cmd.BoolOpt("sudo", false, "use sudo when daemonizing")
		groupID    = cmd.IntOpt("gid", -1, "optional group ID for socket and log file creation")
		configfile = cmd.StringOpt("config", config.Config().DefaultConfigFile, "optional filename of config file.")
	)

	cmd.Action = func() {
		_, err := os.Stat(config.Config().DaemonSocketPath)
		if err == nil {
			log.WithField("socket", config.Config().DaemonSocketPath).Fatal("Socket file already exists. Maybe another instance is already running? Remove socket file otherwise.")
		}

		if config.Config().DaemonizeFlag {
			*foreground = true

			hook, err := lSyslog.NewSyslogHook("", "", syslog.LOG_INFO|syslog.LOG_NOTICE|syslog.LOG_DEBUG, "ipvsmesh")
			if err != nil {
				log.WithField("err", err).Fatal("unable to set up syslog")
			} else {
				log.AddHook(hook)
				log.WithField("hook", hook).Debug("syslog hook")
			}
		}

		if logfile != nil && *logfile != "" {
			f, err := os.OpenFile(*logfile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
			if err != nil {
				log.WithField("err", err).Fatal("unable to set up file logging")
			}
			if err := os.Chown(*logfile, -1, *groupID); err != nil {
				log.WithField("err", err).Warn("unable to chgrp for log file.")
			}
			log.WithField("logfile", *logfile).Trace("writing log to file")
			log.SetOutput(f)
		}

		if !*foreground {
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

			configUpdateCh := make(daemon.ConfigUpdateChanType)
			ipvsUpdateCh := make(daemon.IPVSApplierChanType)
			publisherUpdateCh := make(daemon.PublisherUpdateChanType)
			publisherConfigUpdateCh := make(daemon.PublisherConfigUpdateChanType)
			onceCh := make(chan struct{})

			// Create an applier which reads from the channel and applies updates. controls service workers
			configApplier := daemon.NewConfigApplierWorker(configUpdateCh, ipvsUpdateCh, publisherConfigUpdateCh)
			ds.Register(&configApplier.StoppableByChan)
			log.WithField("s", configApplier).Trace("registered")
			go configApplier.Worker()

			// create a watcher on the config file. It will send updates to configUpdateCh
			configWatcher := daemon.NewConfigWatcherWorker(*configfile, configUpdateCh, *once)
			ds.Register(&configWatcher.StoppableByChan)
			log.WithField("s", configWatcher).Trace("registered")
			go configWatcher.Worker()

			// service workers will be created by configApplier dynamically

			// publisher worker
			publisherWorker := daemon.NewPublisherWorker(publisherUpdateCh, publisherConfigUpdateCh, *once, onceCh)
			ds.Register(&publisherWorker.StoppableByChan)
			log.WithField("s", publisherWorker).Trace("registered")
			go publisherWorker.Worker()

			// create an IPVSApplier (holding and applying the central ipvs model)
			ipvsApplier := daemon.NewIPVSApplierWorker(ipvsUpdateCh, publisherUpdateCh)
			ds.Register(&ipvsApplier.StoppableByChan)
			log.WithField("s", ipvsApplier).Trace("registered")
			go ipvsApplier.Worker()

			go func() {
				for {
					select {
					case <-onceCh:
						log.Trace("daemon: going down via internal stop")
						ds.Stop(context.Background(), &localinterface.Empty{})
					}
				}
			}()

			// main loop, wait for commands from cmdline client as per grpc calls
			ds.Start(context.Background())

		}
	}
}

func connect() *grpc.ClientConn {
	_, err := os.Stat(config.Config().DaemonSocketPath)
	if err != nil {
		log.WithFields(log.Fields{"socket": config.Config().DaemonSocketPath, "err": err}).Fatal("Socket file not present. Maybe daemon is not running?")
	}

	var conn *grpc.ClientConn

	//	dialOptions := grpc.WithContextDialer(func(ctx context.Context, addr string, timeout time.Duration) (net.Conn, error) {
	dialOptions := grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout("unix", addr, timeout)
	})
	if config.Config().TLS {
		creds, err := credentials.NewClientTLSFromFile(config.Config().TLSCertFile, "ipvsmesh-grpc-tls-comm")
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

// DaemonStop stops the daemon by sending the stop command to background process
func DaemonStop(cmd *cli.Cmd) {
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
