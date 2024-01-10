package hook

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=magalu.cloud.io

var log = logf.Log.WithName("sidecar-injector")

// SidecarInjector annotates Pods
type sidecarInjector struct {
	Name          string
	Client        client.Client
	decoder       *admission.Decoder
	SidecarConfig *Config
}

type Config struct {
	Containers []corev1.Container `yaml:"containers"`
}

type ExtendedPod struct {
	corev1.Pod
}

// NewServiceAnnotator function creates a new instance of a service annotator
func NewSidecarInjector(name string, client client.Client, scheme *runtime.Scheme, config *Config) admission.Handler {

	return &sidecarInjector{
		Name:    name,
		Client:  client,
		decoder: admission.NewDecoder(scheme),

		SidecarConfig: config,
	}
}

func shoudInject(pod *corev1.Pod) bool {
	shouldInjectSidecar, err := strconv.ParseBool(pod.Annotations["inject-logging-sidecar"])

	if err != nil {
		shouldInjectSidecar = false
	}

	if shouldInjectSidecar {
		alreadyUpdated, err := strconv.ParseBool(pod.Annotations["logging-sidecar-added"])

		if err == nil && alreadyUpdated {
			shouldInjectSidecar = false
		}
	}

	// log.Info("Should Inject: ", shouldInjectSidecar)

	return shouldInjectSidecar
}

// SidecarInjector adds an annotation to every incoming pods.
func (si *sidecarInjector) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := si.decoder.Decode(req, pod)
	if err != nil {
		log.Error(err, "Sidecar injector: cannot decode")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	shoudInjectSidecar := shoudInject(pod)

	if shoudInjectSidecar {
		log.Info("Injecting sidecar...")

		// pod.Spec.Containers = append(pod.Spec.Containers, si.SidecarConfig.Containers...)

		pod.Annotations["logging-sidecar-added"] = "true"

		log.Info("Sidecar ", si.Name, " injected.")
	} else {
		log.Info("Inject not needed.")
	}

	marshaledPod, err := json.Marshal(pod)

	if err != nil {
		log.Error(err, "Sidecar-Injector: cannot marshal")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}
