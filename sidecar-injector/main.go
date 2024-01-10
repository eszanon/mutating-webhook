package main

import (
	"flag"
	"os"

	hook "github.com/eszanon/mutating-webhook/webhook"
	"gopkg.in/yaml.v2"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var log = ctrl.Log.WithName("sidecar-injector")

type HookParamters struct {
	certDir       string
	sidecarConfig string
	port          int
}

func loadConfig(configFile string) (*hook.Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var cfg hook.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func main() {
	var params HookParamters

	flag.IntVar(&params.port, "port", 8443, "Wehbook port")
	flag.StringVar(&params.certDir, "certDir", "/certs/", "Wehbook certificate folder")
	flag.StringVar(&params.sidecarConfig, "sidecarConfig", "/etc/webhook/config/sidecarconfig.yaml", "Wehbook sidecar config")
	flag.Parse()

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	entryLog := log.WithName("entrypoint")

	// Setup a Manager
	entryLog.Info("setting up manager")
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		entryLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	config, err := loadConfig(params.sidecarConfig)
	if err != nil {
		entryLog.Error(err, "unable to load sidecar config")
		os.Exit(1)
	}

	// Setup webhooks
	entryLog.Info("setting up webhook server")

	sidecarInjector := hook.NewSidecarInjector(
		"Logger",
		mgr.GetClient(),
		mgr.GetScheme(),
		config,
	)
	// mgr.GetWebhookServer().Register("/mutate", &webhook.Admission{Handler: sidecarInjector})

	// Create a webhook server.
	hookServer := webhook.NewServer(webhook.Options{
		Port:    params.port,
		CertDir: params.certDir,
	})

	if err := mgr.Add(hookServer); err != nil {
		entryLog.Error(err, "unable to register webhook server")
		os.Exit(1)
	}

	entryLog.Info("registering webhooks to the webhook server")
	// hookServer.Register("/mutate", &webhook.Admission{Handler: &hook.SidecarInjector{Name: "Logger", Client: mgr.GetClient(), SidecarConfig: config}})
	hookServer.Register("/mutate", &webhook.Admission{Handler: &webhook.Admission{Handler: sidecarInjector}})

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		entryLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		entryLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	entryLog.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}
}
