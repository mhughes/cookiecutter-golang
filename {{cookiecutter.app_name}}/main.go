package main

import (
	"flag"
	"fmt"
)

func main() {

	versionFlag := flag.Bool("version", false, "Version")
	flag.Parse()

	if *versionFlag {
		fmt.Println("Git Commit:", GitCommit)
		fmt.Println("Version:", Version)
		if VersionPrerelease != "" {
			fmt.Println("Version PreRelease:", VersionPrerelease)
		}
		return
	}

	fmt.Println("Hello.")
}


package main

import (

	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	conf "integrations-services-go/components/config"
	"integrations-services-go/components/queue"
	"integrations-services-go/components/logger"
)

const description = "{{cookiecutter.app_name}} Service"

var (
	log	logger.Logger
	pid        = os.Getpid()
	host       = "localhost"
	configFile = flag.String("c", "./config/api-config.yml", "YAML Config file")
)



func buildConsumer(cf *conf.Config, l logger.Logger) *queue.Consumer {
	var binding string

	if len(cf.Amqp.BindingKeys) == 0 {
		binding = ""
	} else {
		binding = cf.Amqp.BindingKeys[0]
	}

	return queue.NewConsumer(
		cf.Amqp.Url,
		cf.Amqp.Exchange,
		cf.Amqp.ExchangeType,
		cf.Amqp.Queue,
		binding,
		fmt.Sprintf("timeline-%s.%d", host, pid),
		cf.Amqp.QoS,
		cf.Concurrency,
		l)
}

func main() {
	var err error

	logger.Set(logger.CreateDefault())
	log = logger.Get()

	config, err := conf.LoadConfig(configFile)
	failOnError(err, "Could not parse config file")
	errorChan := make(chan error)

	// Start Http Server (Expvar)
	go func() {
		addr := fmt.Sprintf(":%d", config.Http.Port)
		log.LogInfo("transport", "HTTP", "addr", addr, "msg", "listening")
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			errorChan <- err
		}
	}()

	// Start Consumer
	go func() {
		err := consumer.Start(handler)
		if err != nil {
			errorChan <- err
		}
	}()

	// Listen for SIGHUP, SIGUSR1 signals
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP, syscall.SIGUSR1)
	go func() {
		for {
			s := <-hup
			log.LogInfo("signal", s, "msg", "reloading")
			conf.ReloadConfig(configFile)
		}
	}()

	// Listen for common kill types
	kill := make(chan os.Signal, 1)
	signal.Notify(kill, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	select {
	case err := <-errorChan:
		failOnError(err, "Could not run consumer")
	case s := <-kill:
		log.Log("signal", s, "msg", "stopping")
		err := consumer.Stop()
		if err != nil {
			failOnError(err, "Could not stop consumer")
		} else {
			log.LogInfo("msg", "stopped")
		}
	}
}

func failOnError(err error, msg string) {
	if err != nil {
		log.LogFatal("%s: %s", msg, err)
	}
}
