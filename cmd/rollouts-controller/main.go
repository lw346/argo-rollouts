package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	"github.com/golang/glog"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/argoproj/argo-rollouts/controller"
	clientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	informers "github.com/argoproj/argo-rollouts/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-rollouts/pkg/signals"
)

const (
	// CLIName is the name of the CLI
	cliName = "argo-rollouts"
)

func newCommand() *cobra.Command {
	var (
		clientConfig        clientcmd.ClientConfig
		rolloutResyncPeriod int64
		logLevel            string
		glogLevel           int
		metricsPort         int
		rolloutThreads      int
		experimentThreads   int
		serviceThreads      int
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "argo-rollouts is a controller to operate on rollout CRD",
		RunE: func(c *cobra.Command, args []string) error {
			setLogLevel(logLevel)
			formatter := &log.TextFormatter{
				FullTimestamp: true,
			}
			log.SetFormatter(formatter)
			setGLogLevel(glogLevel)

			// set up signals so we handle the first shutdown signal gracefully
			stopCh := signals.SetupSignalHandler()

			// cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
			config, err := clientConfig.ClientConfig()
			checkError(err)
			namespace := metav1.NamespaceAll
			configNS, modified, err := clientConfig.Namespace()
			checkError(err)
			if modified {
				namespace = configNS
				log.Infof("Using namespace %s", namespace)
			}

			kubeClient, err := kubernetes.NewForConfig(config)
			checkError(err)
			rolloutClient, err := clientset.NewForConfig(config)
			checkError(err)
			resyncDuration := time.Duration(rolloutResyncPeriod) * time.Second
			kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
				kubeClient,
				resyncDuration,
				kubeinformers.WithNamespace(namespace))
			argoRolloutsInformerFactory := informers.NewSharedInformerFactoryWithOptions(
				rolloutClient,
				resyncDuration,
				informers.WithNamespace(namespace))
			cm := controller.NewManager(kubeClient, rolloutClient,
				kubeInformerFactory.Apps().V1().ReplicaSets(),
				kubeInformerFactory.Core().V1().Services(),
				argoRolloutsInformerFactory.Argoproj().V1alpha1().Rollouts(),
				argoRolloutsInformerFactory.Argoproj().V1alpha1().Experiments(),
				resyncDuration,
				metricsPort)

			// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
			// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
			kubeInformerFactory.Start(stopCh)
			argoRolloutsInformerFactory.Start(stopCh)

			if err = cm.Run(rolloutThreads, serviceThreads, experimentThreads, stopCh); err != nil {
				glog.Fatalf("Error running controller: %s", err.Error())
			}
			return nil
		},
	}
	clientConfig = addKubectlFlagsToCmd(&command)
	command.Flags().Int64Var(&rolloutResyncPeriod, "rollout-resync", controller.DefaultRolloutResyncPeriod, "Time period in seconds for rollouts resync.")
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	command.Flags().IntVar(&metricsPort, "metricsport", controller.DefaultMetricsPort, "Set the port the metrics endpoint should be exposed over")
	command.Flags().IntVar(&rolloutThreads, "rollout-threads", controller.DefaultRolloutThreads, "Set the number of worker threads for the Rollout controller")
	command.Flags().IntVar(&experimentThreads, "experiment-threads", controller.DefaultExperimentThreads, "Set the number of worker threads for the Experiment controller")
	command.Flags().IntVar(&serviceThreads, "service-threads", controller.DefaultServiceThreads, "Set the number of worker threads for the Service controller")
	return &command
}

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func addKubectlFlagsToCmd(cmd *cobra.Command) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	cmd.PersistentFlags().StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	clientcmd.BindOverrideFlags(&overrides, cmd.PersistentFlags(), kflags)
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
}

// setLogLevel parses and sets a logrus log level
func setLogLevel(logLevel string) {
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(level)
}

// setGLogLevel set the glog level for the k8s go-client
func setGLogLevel(glogLevel int) {
	_ = flag.CommandLine.Parse([]string{})
	_ = flag.Lookup("logtostderr").Value.Set("true")
	_ = flag.Lookup("v").Value.Set(strconv.Itoa(glogLevel))
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
