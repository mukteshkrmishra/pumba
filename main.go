package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gaia-adm/pumba/action"
	"github.com/gaia-adm/pumba/container"

	"github.com/urfave/cli"

	log "github.com/Sirupsen/logrus"
	"github.com/johntdyer/slackrus"
)

var (
	gWG       sync.WaitGroup
	client    container.Client
	chaos     action.Chaos
	gInterval time.Duration
	gTestRun  bool
)

// LinuxSignals valid Linux signal table
// http://www.comptechdoc.org/os/linux/programming/linux_pgsignals.html
var LinuxSignals = map[string]int{
	"SIGHUP":    1,
	"SIGINT":    2,
	"SIGQUIT":   3,
	"SIGILL":    4,
	"SIGTRAP":   5,
	"SIGIOT":    6,
	"SIGBUS":    7,
	"SIGFPE":    8,
	"SIGKILL":   9,
	"SIGUSR1":   10,
	"SIGSEGV":   11,
	"SIGUSR2":   12,
	"SIGPIPE":   13,
	"SIGALRM":   14,
	"SIGTERM":   15,
	"SIGSTKFLT": 16,
	"SIGCHLD":   17,
	"SIGCONT":   18,
	"SIGSTOP":   19,
	"SIGTSTP":   20,
	"SIGTTIN":   21,
	"SIGTTOU":   22,
	"SIGURG":    23,
	"SIGXCPU":   24,
	"SIGXFSZ":   25,
	"SIGVTALRM": 26,
	"SIGPROF":   27,
	"SIGWINCH":  28,
	"SIGIO":     29,
	"SIGPWR":    30,
}

const (
	// Release version
	Release = "v0.2.0"
	// DefaultSignal default kill signal
	DefaultSignal = "SIGKILL"
	// Re2Prefix re2 regexp string prefix
	Re2Prefix = "re2:"
)

func init() {
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.TextFormatter{})
	// set chaos to Pumba{}
	chaos = action.Pumba{}
}

func main() {
	rootCertPath := "/etc/ssl/docker"

	if os.Getenv("DOCKER_CERT_PATH") != "" {
		rootCertPath = os.Getenv("DOCKER_CERT_PATH")
	}

	app := cli.NewApp()
	app.Name = "Pumba"
	app.Version = Release
	app.Usage = "Pumba is a resilience testing tool, that helps applications tolerate random Docker container failures: process, network and performance."
	app.ArgsUsage = "containers (name, list of names, RE2 regex)"
	app.Before = before
	app.Commands = []cli.Command{
		{
			Name: "kill",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "signal, s",
					Usage: "termination signal, that will be sent by Pumba to the main process inside target container(s)",
					Value: DefaultSignal,
				},
			},
			Usage:       "kill specified containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "send termination signal to the main process inside target container(s)",
			Action:      kill,
			Before:      beforeCommand,
		},
		{
			Name: "netem",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "duration, d",
					Usage: "network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				},
				cli.StringFlag{
					Name:  "interface, i",
					Usage: "network interface to apply delay on",
					Value: "eth0",
				},
				cli.StringFlag{
					Name:  "target, t",
					Usage: "target IP filter; netem will impact only on traffic to target IP",
				},
			},
			Usage:       "emulate the properties of wide area networks",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "delay, loss, duplicate and re-order (run 'netem') packets, to emulate different network problems",
			Subcommands: []cli.Command{
				{
					Name: "delay",
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "amount, a",
							Usage: "delay amount; in milliseconds",
							Value: 100,
						},
						cli.IntFlag{
							Name:  "variation, v",
							Usage: "random delay variation; in milliseconds; example: 100ms ± 10ms",
							Value: 10,
						},
						cli.IntFlag{
							Name:  "correlation, c",
							Usage: "delay correlation; in percents",
							Value: 20,
						},
					},
					Usage:       "dealy egress traffic",
					ArgsUsage:   "containers (name, list of names, RE2 regex)",
					Description: "dealy egress traffic for specified containers; networks show variability so it is possible to add random variation; delay variation isn't purely random, so to emulate that there is a correlation",
					Action:      netemDelay,
					Before:      beforeCommand,
				},
				{
					Name: "loss",
				},
				{
					Name: "duplicate",
				},
				{
					Name: "corrupt",
				},
			},
		},
		{
			Name: "pause",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "duration, d",
					Usage: "pause duration: should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				},
			},
			Usage:       "pause all processes",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "pause all running processes within target containers",
			Action:      pause,
			Before:      beforeCommand,
		},
		{
			Name: "stop",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "time, t",
					Usage: "seconds to wait for stop before killing container (default 10)",
					Value: 10,
				},
			},
			Usage:       "stop containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "stop the main process inside target containers, sending  SIGTERM, and then SIGKILL after a grace period",
			Action:      stop,
			Before:      beforeCommand,
		},
		{
			Name: "rm",
			Flags: []cli.Flag{
				cli.BoolTFlag{
					Name:  "force, f",
					Usage: "force the removal of a running container (with SIGKILL)",
				},
				cli.BoolTFlag{
					Name:  "links, l",
					Usage: "remove container links",
				},
				cli.BoolTFlag{
					Name:  "volumes, v",
					Usage: "remove volumes associated with the container",
				},
			},
			Usage:       "remove containers",
			ArgsUsage:   "containers (name, list of names, RE2 regex)",
			Description: "remove target containers, with links and voluems",
			Action:      remove,
			Before:      beforeCommand,
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Usage:  "daemon socket to connect to",
			Value:  "unix:///var/run/docker.sock",
			EnvVar: "DOCKER_HOST",
		},
		cli.BoolFlag{
			Name:  "tls",
			Usage: "use TLS; implied by --tlsverify",
		},
		cli.BoolFlag{
			Name:   "tlsverify",
			Usage:  "use TLS and verify the remote",
			EnvVar: "DOCKER_TLS_VERIFY",
		},
		cli.StringFlag{
			Name:  "tlscacert",
			Usage: "trust certs signed only by this CA",
			Value: fmt.Sprintf("%s/ca.pem", rootCertPath),
		},
		cli.StringFlag{
			Name:  "tlscert",
			Usage: "client certificate for TLS authentication",
			Value: fmt.Sprintf("%s/cert.pem", rootCertPath),
		},
		cli.StringFlag{
			Name:  "tlskey",
			Usage: "client key for TLS authentication",
			Value: fmt.Sprintf("%s/key.pem", rootCertPath),
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug mode with verbose logging",
		},
		cli.BoolFlag{
			Name:  "json",
			Usage: "produce log in JSON format: Logstash and Splunk friendly"},
		cli.StringFlag{
			Name:  "slackhook",
			Usage: "web hook url; send Pumba log events to Slack",
		},
		cli.StringFlag{
			Name:  "slackchannel",
			Usage: "Slack channel (default #pumba)",
			Value: "#pumba",
		},
		cli.StringFlag{
			Name:  "interval, i",
			Usage: "recurrent interval for chaos command; use with optional unit suffix: 'ms/s/m/h'",
		},
		cli.BoolFlag{
			Name:        "random, r",
			Usage:       "randomly select single matching container from list of target containers",
			Destination: &action.RandomMode,
		},
		cli.BoolFlag{
			Name:        "dry",
			Usage:       "dry runl does not create chaos, only logs planned chaos commands",
			Destination: &action.DryMode,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func before(c *cli.Context) error {
	// set debug log level
	if c.GlobalBool("debug") {
		log.SetLevel(log.DebugLevel)
	}
	// set log formatter to JSON
	if c.GlobalBool("json") {
		log.SetFormatter(&log.JSONFormatter{})
	}
	// set Slack log channel
	if c.GlobalString("slackhook") != "" {
		log.AddHook(&slackrus.SlackrusHook{
			HookURL:        c.GlobalString("slackhook"),
			AcceptedLevels: slackrus.LevelThreshold(log.GetLevel()),
			Channel:        c.GlobalString("slackchannel"),
			IconEmoji:      ":boar:",
			Username:       "pumba_bot",
		})
	}
	// Set-up container client
	tls, err := tlsConfig(c)
	if err != nil {
		return err
	}
	// create new Docker client
	client = container.NewClient(c.GlobalString("host"), tls)
	// habdle termination signal
	handleSignals()
	return nil
}

// beforeCommand run before each chaos command
func beforeCommand(c *cli.Context) error {
	// get recurrent time interval
	if intervalString := c.GlobalString("interval"); intervalString == "" {
		return errors.New("Undefined interval value.")
	} else if interval, err := time.ParseDuration(intervalString); err != nil {
		return err
	} else {
		gInterval = interval
	}
	return nil
}

func getNamesOrPattern(c *cli.Context) ([]string, string) {
	names := []string{}
	pattern := ""
	// get container names or pattern: no Args means ALL containers
	if c.Args().Present() {
		// more than one argument, assume that this a list of names
		if len(c.Args()) > 1 {
			names = c.Args()
			log.Debugf("Names: '%s'", names)
		} else {
			first := c.Args().First()
			if strings.HasPrefix(first, Re2Prefix) {
				pattern = strings.Trim(first, Re2Prefix)
				log.Debugf("Pattern: '%s'", pattern)
			}
		}
	}
	return names, pattern
}

func runChaosCommand(cmd interface{}, names []string, pattern string, chaosFn func(container.Client, []string, string, interface{}) error) {
	// channel for 'chaos' command
	dc := make(chan interface{})
	// create Time channel for specified intterval: for TestRun use Timer (one time call)
	var cmdTimeChan <-chan time.Time
	if gTestRun {
		cmdTimeChan = time.NewTimer(gInterval).C
	} else {
		cmdTimeChan = time.NewTicker(gInterval).C
	}
	// handle interval timer event
	go func(cmd interface{}) {
		for range cmdTimeChan {
			dc <- cmd
			if gTestRun {
				close(dc)
			}
		}
	}(cmd)
	// handle 'chaos' command
	for cmd := range dc {
		gWG.Add(1)
		go func(cmd interface{}) {
			defer gWG.Done()
			if err := chaosFn(client, names, pattern, cmd); err != nil {
				log.Error(err)
			}
		}(cmd)
	}
}

// KILL Command
func kill(c *cli.Context) error {
	// get names or pattern
	names, pattern := getNamesOrPattern(c)
	// get signal
	signal := c.String("signal")
	if _, ok := LinuxSignals[signal]; !ok {
		err := errors.New("Unexpected signal: " + signal)
		log.Error(err)
		return err
	}
	runChaosCommand(action.CommandKill{Signal: signal}, names, pattern, chaos.KillContainers)
	return nil
}

// NETEM DELAY command
func netemDelay(c *cli.Context) error {
	// get names or pattern
	names, pattern := getNamesOrPattern(c)
	// get duration
	var durationString string
	if c.Parent() != nil {
		durationString = c.Parent().String("duration")
	}
	if durationString == "" {
		err := errors.New("Undefined duration interval")
		log.Error(err)
		return err
	}
	duration, err := time.ParseDuration(durationString)
	if err != nil {
		log.Error(err)
		return err
	}
	// get network interface and target ip
	netInterface := "eth0"
	var ip net.IP
	if c.Parent() != nil {
		netInterface = c.Parent().String("interface")
		// protect from Command Injection, using Regexp
		reInterface := regexp.MustCompile("[a-zA-Z]+[0-9]{0,2}")
		validInterface := reInterface.FindString(netInterface)
		if netInterface != validInterface {
			err := fmt.Errorf("Bad network interface name. Must match '%s'", reInterface.String())
			log.Error(err)
			return err
		}
		// get target IP Filter
		ip = net.ParseIP(c.Parent().String("target"))
	}
	// get delay amount
	amount := c.Int("amount")
	if amount <= 0 {
		err = errors.New("Invalid delay amount")
		log.Error(err)
		return err
	}
	// get delay variation
	variation := c.Int("variation")
	if variation < 0 || variation > amount {
		err = errors.New("Invalid delay variation")
		log.Error(err)
		return err
	}
	// get delay variation
	correlation := c.Int("correlation")
	if correlation < 0 || correlation > 100 {
		err = errors.New("Invalid delay correlation: must be between 0 and 100")
		log.Error(err)
		return err
	}
	// pepare netem delay command
	delayCmd := action.CommandNetemDelay{
		NetInterface: netInterface,
		IP:           ip,
		Duration:     duration,
		Amount:       amount,
		Variation:    variation,
		Correlation:  correlation,
	}
	runChaosCommand(delayCmd, names, pattern, chaos.NetemDelayContainers)
	return nil
}

// PAUSE command
func pause(c *cli.Context) error {
	// get names or pattern
	names, pattern := getNamesOrPattern(c)
	// get duration
	durationString := c.String("duration")
	if durationString == "" {
		err := errors.New("Undefined duration interval")
		log.Error(err)
		return err
	}
	duration, err := time.ParseDuration(durationString)
	if err != nil {
		log.Error(err)
		return err
	}
	cmd := action.CommandPause{Duration: duration}
	runChaosCommand(cmd, names, pattern, chaos.PauseContainers)
	return nil
}

// REMOVE Command
func remove(c *cli.Context) error {
	// get names or pattern
	names, pattern := getNamesOrPattern(c)
	// get force flag
	force := c.BoolT("force")
	// get link flag
	links := c.BoolT("links")
	// get link flag
	volumes := c.BoolT("volumes")
	// run chaos command
	cmd := action.CommandRemove{Force: force, Links: links, Volumes: volumes}
	runChaosCommand(cmd, names, pattern, chaos.RemoveContainers)
	return nil
}

// STOP Command
func stop(c *cli.Context) error {
	// get names or pattern
	names, pattern := getNamesOrPattern(c)
	// run chaos command
	cmd := action.CommandStop{WaitTime: c.Int("time")}
	runChaosCommand(cmd, names, pattern, chaos.StopContainers)
	return nil
}

func handleSignals() {
	// Graceful shut-down on SIGINT/SIGTERM
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	go func() {
		<-c
		gWG.Wait()
		os.Exit(1)
	}()
}

// tlsConfig translates the command-line options into a tls.Config struct
func tlsConfig(c *cli.Context) (*tls.Config, error) {
	var tlsConfig *tls.Config
	var err error
	caCertFlag := c.GlobalString("tlscacert")
	certFlag := c.GlobalString("tlscert")
	keyFlag := c.GlobalString("tlskey")

	if c.GlobalBool("tls") || c.GlobalBool("tlsverify") {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: !c.GlobalBool("tlsverify"),
		}

		// Load CA cert
		if caCertFlag != "" {
			var caCert []byte
			if strings.HasPrefix(caCertFlag, "/") {
				caCert, err = ioutil.ReadFile(caCertFlag)
				if err != nil {
					return nil, err
				}
			} else {
				caCert = []byte(caCertFlag)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caCertPool
		}

		// Load client certificate
		if certFlag != "" && keyFlag != "" {
			var cert tls.Certificate
			if strings.HasPrefix(certFlag, "/") && strings.HasPrefix(keyFlag, "/") {
				cert, err = tls.LoadX509KeyPair(certFlag, keyFlag)
				if err != nil {
					return nil, err
				}
			} else {
				cert, err = tls.X509KeyPair([]byte(certFlag), []byte(keyFlag))
				if err != nil {
					return nil, err
				}
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}
	return tlsConfig, nil
}
